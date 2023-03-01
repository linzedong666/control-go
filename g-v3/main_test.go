package main

import (
	"context"
	"fmt"
	"go.uber.org/goleak"
	"testing"
	"time"
)

func TestMain(m *testing.M) {

	m.Run()
}

func TestGo(k *testing.T) {
	defer goleak.VerifyNone(k)
	// 同时运行2个，遇到3个错误的时候就退出，设置0表示等所有运行完毕
	t := NewTask(2, 1)
	t.AddJob(func(ctx context.Context) {
		fmt.Printf("%s: A job\n", time.Now())
	})
	t.AddJob(func(ctx context.Context) {
		time.Sleep(2 * time.Second)

		panic("B job panic")
	})
	t.AddJob(func(ctx context.Context) {
		time.Sleep(10 * time.Second)
		fmt.Printf("%s: C job\n", time.Now())
	})
	t.AddJob(func(ctx context.Context) {
		time.Sleep(50 * time.Second)
		fmt.Printf("%s: D job\n", time.Now())
	})

	t.Run()
	t.Wait()
}
