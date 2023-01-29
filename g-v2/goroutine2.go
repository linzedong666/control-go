package g_v2

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

var closedchan = make(chan struct{})

func init() {
	close(closedchan)
}

type Config struct {
	Limit int //限制同时存在的协程数，0则不受限制
	//GoCount int  //要控制的协程数量
	Wait bool //是否等待所有goroutine执行完毕再关闭,遇到错误可立即结束阻塞,默认不等
}

type goSync struct {
	funcs   []func()
	running atomic.Bool

	errs   []error
	eMutex sync.Mutex
	wchan  chan struct{}

	gNum    atomic.Int64
	gStart  atomic.Int64
	gFinish atomic.Int64

	limit chan struct{}
	wait  bool //是否等待协程组结束后结束阻塞

	syncErrChan chan error
	errOnce     sync.Once
	isCause     atomic.Bool //因错误导致结束

	closeOnce sync.Once
}

func New(config Config) *goSync {
	g := &goSync{}
	g.gStart.Store(0)
	g.gFinish.Store(0)
	g.wchan = make(chan struct{}, 0)
	g.syncErrChan = make(chan error, 1)
	if config.Limit > 0 {
		g.limit = make(chan struct{}, config.Limit)
	}
	g.wait = config.Wait
	return g
}

// Go 该方法不支持并发
func (g *goSync) Go(f func()) error {
	if g.running.Load() {
		return errors.New("goroutines have been activated,can't add any more")
	}
	g.funcs = append(g.funcs, f)
	return nil
}

func (g *goSync) Run() error {
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
						g.eMutex.Lock()
						g.errs = append(g.errs, err)
						g.eMutex.Unlock()
					}
					if g.gFinish.Add(1) == g.gNum.Load() {
						close(g.wchan)
					}
					if g.limit != nil {
						//即使启动前因为并发导致没有执行g.limit <- struct{}{}，但只要最终调用了close,该协程最终也能关闭
						<-g.limit
					}
				}()
				g.funcs[index]()
			}(i)
		}
	}()
	return nil
}

func (g *goSync) SentErr(err error) {
	if err == nil {
		panic("goroutines have been not activated,can't called this method")
	}
	//后续可额外处理同步等待的错误，新起数据结构接收或者输送到goSync.errs中
	if g.wait {
		return
	}

	g.errOnce.Do(func() {
		g.syncErrChan <- err
		g.isCause.Store(true)
	})
}

// Err 协程组结束之前会阻塞,不可重复调用
func (g *goSync) Err() error {
	//协程组开启前不可调用
	if !g.running.Load() {
		panic("")
	}
	select {
	case err := <-g.syncErrChan:
		return err
	case <-g.wchan:
		// 二重检测是否没有错误
		if g.isCause.Load() {
			return <-g.syncErrChan
		}
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
