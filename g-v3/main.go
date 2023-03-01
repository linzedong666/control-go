package main

import (
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"
)

type Job func(ctx context.Context)

type Task struct {
	jobs       list.List
	errorLimit int
	errorList  []error
	mu         sync.Mutex
	queue      chan struct{}

	ctx    context.Context
	cancel context.CancelFunc
}

func NewTask(concurrentCount int, errorLimit int) *Task {
	t := &Task{
		jobs:       list.List{},
		mu:         sync.Mutex{},
		errorLimit: errorLimit,
		queue:      make(chan struct{}, concurrentCount),
	}
	return t
}

func (t *Task) AddJob(callback func(ctx context.Context)) *Task {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.jobs.PushBack(Job(callback))
	return t
}

func (t *Task) Run() {
	t.ctx, t.cancel = context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-t.ctx.Done(): //全局任务全退出
				return
			default:
			}
			t.mu.Lock()
			el := t.jobs.Front()
			t.mu.Unlock()

			if el != nil {
				job := el.Value.(Job)
				select {
				case t.queue <- struct{}{}: // 控制同时运行的数量
					go t.handle(job) // 异步运行
				case <-t.ctx.Done(): // 识别全局是否退出
					return
				}
			} else {
				return
			}

			t.mu.Lock()
			t.jobs.Remove(el)
			t.mu.Unlock()
		}
	}()

}

func (t *Task) Stop() {
	t.cancel()
}

func (t *Task) checkStopping() {
	t.mu.Lock()
	defer t.mu.Unlock()
	// 错误数量过多
	if t.errorLimit > 0 && len(t.errorList) >= t.errorLimit {
		t.cancel()
	}
	// 没队列了
	if t.jobs.Len() <= 0 {
		t.cancel()
	}
}

func (t *Task) handle(j Job) {
	defer func() {
		// 运行完毕则让出队列
		<-t.queue
		t.checkStopping()
	}()

	defer func() {
		if err1 := recover(); err1 != nil {
			t.throwError(fmt.Errorf("%v", err1), j)
			return
		}
	}()

	j(t.ctx)
}

func (t *Task) throwError(err error, j Job) {
	t.mu.Lock()
	t.errorList = append(t.errorList, err)
	t.mu.Unlock()

	t.checkStopping()
}

func (t *Task) Wait() []error {
	select {
	case <-t.ctx.Done():
	}

	return t.errorList
}

func main() {
	// 同时运行2个，遇到3个错误的时候就退出，设置0表示等所有运行完毕
	t := NewTask(2, 1)
	t.AddJob(func(ctx context.Context) {
		fmt.Printf("%s: A job\n", time.Now())
	})
	t.AddJob(func(ctx context.Context) {
		time.Sleep(200 * time.Millisecond)

		panic("B job panic")
	})
	t.AddJob(func(ctx context.Context) {
		time.Sleep(100 * time.Millisecond)
		fmt.Printf("%s: C job\n", time.Now())
	})
	t.AddJob(func(ctx context.Context) {
		time.Sleep(100 * time.Second)
		fmt.Printf("%s: D job\n", time.Now())
	})

	t.Run()
	t.Wait()
}
