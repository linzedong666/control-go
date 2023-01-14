package g

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
	Limit   int  //限制同时存在的协程数，0则不受限制
	GoCount int  //要控制的协程数量
	Wait    bool //是否等待所有goroutine执行完毕再关闭,遇到错误可立即结束阻塞,默认不等
}

type goSync struct {
	errs   []error
	eMutex sync.Mutex
	wchan  chan struct{}

	gNum    atomic.Int64
	gStart  atomic.Int64
	gFinish atomic.Int64

	limit chan struct{}
	wait  bool //是否等待协程组结束后结束阻塞

	errChan     atomic.Value
	errChanOnce sync.Once
	isFinish    atomic.Bool
}

func New(config Config) *goSync {
	g := NewGoS(config.GoCount)
	if config.Limit > 0 {
		g.limit = make(chan struct{}, config.Limit)
	}
	g.wait = config.Wait
	return g
}

func NewGoS(goCount int) *goSync {
	//var waitGroup sync.WaitGroup
	//waitGroup.Add(goCount)
	//g := &goSync{w: waitGroup}
	g := &goSync{}
	g.gNum.Store(int64(goCount))
	g.gStart.Store(0)
	g.gFinish.Store(0)
	g.wchan = make(chan struct{}, 0)
	g.errChan.Store(make(chan error, 1))
	return g
}

func (g *goSync) Go(f func()) error {
	if g.gStart.Add(1) > g.gNum.Load() {
		return errors.New("The number of goroutines created exceeds the limit.")
	}
	if g.isFinish.Load() {
		return errors.New("goroutine group control has ended early due to unknown error")
	}
	if g.limit != nil && !g.isFinish.Load() {
		g.limit <- struct{}{}
	}

	go func() {
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
				<-g.limit
			}
		}()
		f()
	}()
	return nil
}

// SentErr 只有当wait为false时候可以调用
func (g *goSync) SentErr(err error) {
	if g.wait {
		panic("goSync is in a synchronous wait state")
	}
	if err == nil {
		panic("error cannot be nil")
	}
	g.errChanOnce.Do(func() {
		g.isFinish.Store(true)
		g.errChan.Load().(chan error) <- err
	})
}

// Err 协程组结束之前会阻塞,不可重复调用
func (g *goSync) Err() error {
	select {
	case <-g.wchan:
		g.close()
	case err := <-g.errChan.Load().(chan error):
		g.close()
		return err
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
func (g *goSync) close() {
	g.isFinish.Store(true)
	close(g.errChan.Load().(chan error))
	if g.limit != nil {
		close(g.limit)
	}

}
