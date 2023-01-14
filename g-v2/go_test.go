package g_v2

import (
	"errors"
	"fmt"
	"go.uber.org/goleak"
	"testing"
	"time"
)

func TestGos(t *testing.T) {
	defer goleak.VerifyNone(t)
	for i := 0; i < 100; i++ {
		go func() {
			fmt.Println(i, "------", gos())
		}()
	}
	//time.Sleep(time.Second * 2)
}
func gos() error {
	s := New(Config{Limit: 1})
	s.Go(func() {
		s.SentErr(errors.New("123"))
	})

	s.Go(func() {
		time.Sleep(time.Second)
		//panic("睡死" + strconv.FormatBool(s.isFinish.Load()))
	})
	s.Run()
	return s.Err()
}
