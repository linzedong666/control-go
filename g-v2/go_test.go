package g_v2

import (
	_ "control-go/g"
	"errors"
	"fmt"
	"go.uber.org/goleak"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

func TestGos(t *testing.T) {
	defer goleak.VerifyNone(t)
	start := time.Now()
	for i := 0; i < 1; i++ {
		go func(i int) {
			//gos()
			fmt.Println(i, "------", gos())
		}(i)
	}
	fmt.Println(time.Now().Sub(start).Seconds())
	fmt.Println("总数", runtime.NumGoroutine())
	for runtime.NumGoroutine() > 2 {
		fmt.Println(runtime.NumGoroutine())
		time.Sleep(time.Second)
	}
	fmt.Println("结束", runtime.NumGoroutine())
}
func gos() error {
	s := New(Config{Wait: true, Limit: 3})
	s.Go(func() {
		s.SentErr(errors.New("123"))
		//panic("睡死" + strconv.FormatBool(s.isCause.Load()))
	})
	s.Go(func() {
		panic("123")
		s.SentErr(errors.New("123"))
	})
	s.Go(func() {
		panic("123")
	})
	s.Go(func() {
		panic("123")
	})
	s.Run()
	return s.Err()
}

type markBits struct {
	bytep *uint8
	mask  uint8
	index uintptr
}

//go:linkname isMarked runtime.markBits.isMarked
func isMarked(m markBits) bool

//go:linkname spanOf runtime.spanOf
func spanOf(p uintptr) unsafe.Pointer

//go:linkname objIndex runtime.(*mspan).objIndex
func objIndex(s unsafe.Pointer, p uintptr) uintptr

//go:linkname allocBitsForIndex runtime.(*mspan).allocBitsForIndex
func allocBitsForIndex(s unsafe.Pointer, allocBitIndex uintptr) markBits

func allocBitsForAddr(p uintptr) markBits {
	s := spanOf(p)
	objIndex := objIndex(s, p)
	return allocBitsForIndex(s, objIndex)
}

var ca [10]chan int
var pa [10]uintptr

func TestLinkName(t *testing.T) {

}
func TestGC(t *testing.T) {
	for i := 0; i < len(pa); i++ {
		c := make(chan int, 10)
		p := *(*uintptr)(unsafe.Pointer(&c))
		c <- 1
		c <- 2
		c <- 3
		ca[i] = c
		pa[i] = p
	}
	var c chan int
	for i := 0; i < len(pa); i++ {
		if i == 1 {
			c = ca[i]
			continue
		}
		if i%2 == 0 {
			continue
		}
		ca[i] = nil
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Println(m)
	runtime.GC()
	runtime.ReadMemStats(&m)
	fmt.Println(m)
	for i := 0; i < len(pa); i++ {
		println(isMarked(allocBitsForAddr(pa[i])))
	}
	println(c)
}
