package gsync

import (
	_ "control-go/g"
	"errors"
	"fmt"
	"go.uber.org/goleak"
	"runtime"
	"sync"
	"sync/atomic"
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
	//fmt.Println(m)
	runtime.GC()
	runtime.ReadMemStats(&m)
	//fmt.Println(m)
	for i := 0; i < len(pa); i++ {
		println(isMarked(allocBitsForAddr(pa[i])))
	}
	println(c)
}

type goSync2 struct {
	funcs []func()

	mu    sync.Mutex //保证以下两个字段并发安全
	errs  []error
	wchan chan struct{} //在所有goroutine执行结束之前进行正常的阻塞

	gNum    atomic.Int64 //需要开启的协程数量
	gStart  atomic.Int64 //已经开启的协程数量
	gFinish atomic.Int64 //已经执行结束的协程

	limit chan struct{} //控制在同一时刻协程的启动数量

	syncErrChan chan error  //接收用户手动记录的err
	errOnce     sync.Once   //避免重复接收err
	isCause     atomic.Bool //协程组是否因err而结束

	running atomic.Bool //协程控制是否已经在运行
	wait    bool        //是否等待协程组结束后结束阻塞
}
type A struct {
	c int16
}

func TestSize(t *testing.T) {
	g := gSync{}
	c := closedchan
	var funcs []func()
	var b atomic.Bool
	var err []error
	var i atomic.Int64
	var ce chan error
	var once sync.Once
	fmt.Println("chan:", unsafe.Sizeof(c), unsafe.Alignof(c))
	fmt.Println("funcs:", unsafe.Sizeof(funcs), unsafe.Alignof(funcs))
	fmt.Println("bool:", unsafe.Sizeof(b), unsafe.Alignof(b))
	fmt.Println("err:", unsafe.Sizeof(err), unsafe.Alignof(err))
	fmt.Println("i:", unsafe.Sizeof(i), unsafe.Alignof(i))
	fmt.Println("ce:", unsafe.Sizeof(ce), unsafe.Alignof(ce))
	fmt.Println("once:", unsafe.Sizeof(once), unsafe.Alignof(once))
	fmt.Println("mutex", unsafe.Sizeof(sync.Mutex{}), unsafe.Alignof(sync.Mutex{}))
	fmt.Println("value", unsafe.Sizeof(atomic.Value{}), unsafe.Alignof(atomic.Value{}))
	fmt.Println("gSync", unsafe.Sizeof(g), unsafe.Alignof(g))
	fmt.Println("A", unsafe.Sizeof(A{}), unsafe.Alignof(A{}))
}
