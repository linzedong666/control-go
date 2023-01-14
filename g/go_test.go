package g

import (
	"fmt"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	//goleak.VerifyTestMain(m)
	m.Run()
}
func TestGo(t *testing.T) {
	go func() {
		var s map[string]string
		s["1"] = "2"
	}()
	time.Sleep(time.Second * 10)
}
func gos() {
	s := NewGoS(2)
	go func() {
		s.Go(func() {
			time.Sleep(time.Second * 1)
			var i []int
			fmt.Println(i[0])
		})
	}()
	time.Sleep(time.Second * 2)
	s.Go(func() {
		time.Sleep(time.Second * 1)
		panic("睡死")
	})

	err := s.Err()
	fmt.Println(err)
}
