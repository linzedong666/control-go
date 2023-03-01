package gsync

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var closedchan = make(chan struct{})

func init() {
	close(closedchan)
}

type Config struct {
	Limit int //限制同时存在的协程数，0则不受限制
	//GoCount int  //要控制的协程数量
	Wait    bool //是否等待所有goroutine执行完毕再关闭,遇到错误可立即结束阻塞,默认不等
	Timeout time.Duration
}

type gSync struct {
	funcs []func()

	mu    sync.Mutex //保证以下两个字段并发安全
	errs  []error
	wchan atomic.Value //在所有goroutine执行结束之前进行正常的阻塞,chan struct {}, 懒惰地创建，由最后一个执行完毕的goroutine关闭

	gNum    atomic.Int64 //需要开启的协程数量
	gStart  atomic.Int64 //已经开启的协程数量
	gFinish atomic.Int64 //已经执行结束的协程

	limit chan struct{} //控制在同一时刻协程的启动数量

	syncErrChan chan error  //接收用户手动记录的err
	errOnce     sync.Once   //避免重复接收err
	isCause     atomic.Bool //协程组是否因err而结束

	running atomic.Bool //协程控制是否已经在运行
	wait    bool        //是否等待协程组结束后结束阻塞

	timeout <-chan struct{}
}

func New(config Config) *gSync {
	g := &gSync{}
	g.gStart.Store(0)
	g.gFinish.Store(0)
	//g.wchan = make(chan struct{}, 0)
	g.syncErrChan = make(chan error, 1)
	if config.Limit > 0 {
		g.limit = make(chan struct{}, config.Limit)
	}
	g.wait = config.Wait
	if config.Timeout > 0 {
		timeout, _ := context.WithTimeout(context.Background(), config.Timeout)
		g.timeout = timeout.Done()
	}
	return g
}

// Go 该方法不支持并发
func (g *gSync) Go(f func()) error {
	if g.running.Load() {
		return errors.New("goroutines have been activated,can't add any more")
	}
	g.funcs = append(g.funcs, f)
	return nil
}

func (g *gSync) Run() error {
	g.running.Store(true)
	g.gNum.Store(int64(len(g.funcs)))
	go func() {
		for i := range g.funcs {
			//要是某个协程因错误或者某些原因导致标识结束，则不再开启任务
			if g.isCause.Load() {
				return
			}
			if g.limit != nil {
				g.limit <- struct{}{}
			}
			go func(index int) {
				g.gStart.Add(1)
				defer func() {
					if r := recover(); r != nil {
						err := fmt.Errorf("goroutine panic:%s ,%s", identifyPanic(), fmt.Sprintf("%v", r))
						g.mu.Lock()
						g.errs = append(g.errs, err)
						g.mu.Unlock()
					}

					if g.limit != nil {
						<-g.limit
					}
					//如果当前goroutine为最后一个，结束协程组的阻塞
					if g.gFinish.Add(1) == g.gNum.Load() {
						if d := g.wchan.Load(); d != nil {
							close(d.(chan struct{}))
							return
						}
						g.mu.Lock()
						if d := g.wchan.Load(); d != nil {
							close(d.(chan struct{}))
						} else {
							g.wchan.Store(closedchan)
						}
						g.mu.Unlock()
					}
				}()
				g.funcs[index]()
			}(i)
		}
	}()
	return nil
}

func (g *gSync) SentErr(err error) {
	if err == nil {
		return
	}
	if g.wait {
		g.errOnce.Do(func() {
			g.mu.Lock()
			g.errs = append(g.errs, fmt.Errorf("goroutine err:%s", err))
			g.mu.Unlock()
		})
		return
	}

	g.errOnce.Do(func() {
		g.syncErrChan <- err
		g.isCause.Store(true)
	})
}

// Err 协程组结束之前会阻塞,重复调用可能会陷入永久阻塞
func (g *gSync) Err() error {
	//协程组开启前不可调用
	if !g.running.Load() {
		panic("")
	}
	g.mu.Lock()
	if g.wchan.Load() == nil {
		g.wchan.Store(make(chan struct{}))
	}
	g.mu.Unlock()
	select {
	//重复调用该方法可能会陷入阻塞
	case err := <-g.syncErrChan:
		return err
	case <-g.wchan.Load().(chan struct{}):
		// 二重检测是否没有错误
		if g.isCause.Load() {
			return <-g.syncErrChan
		}
	case <-g.timeout:
		return errors.New("execution timeout")
	}
	if len(g.errs) == 1 {
		return g.errs[0]
	}
	if len(g.errs) > 1 {
		msg := g.errs[0].Error()
		for i := 1; i < len(g.errs); i++ {
			msg = fmt.Sprintf("%s | %s", msg, g.errs[i])
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// identifyPanic 获取 panic 发生的地方
func identifyPanic() string {
	var name, file string
	var line int
	var pc = make([]uintptr, 16)

	_ = runtime.Callers(3, pc[:])
	frames := runtime.CallersFrames(pc)
	for frame, more := frames.Next(); more; frame, more = frames.Next() {
		file = frame.File
		line = frame.Line
		name = frame.Function
		if !strings.HasPrefix(name, "runtime.") {
			break
		}
	}
	switch {
	case name != "":
		return fmt.Sprintf("%v:%v", name, line)
	case file != "":
		return fmt.Sprintf("%v:%v", file, line)
	}

	return fmt.Sprintf("pc:%x", pc)
}
