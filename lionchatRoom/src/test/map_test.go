package test

import (
	"fmt"
	"testing"

	treemap "github.com/liyue201/gostl/ds/map"
)

func TestMap(t *testing.T) {
	m := treemap.New(treemap.WithGoroutineSafe())
	m.Insert(1, "fuck")
	ans := m.Find(1).Value()
	t.Log(ans.(string))
	res := m.Find(2)
	t.Log(res.IsValid())
}

func df(data int) int {
	if data == 1 {
		defer func() {
			fmt.Println("fuck")
		}()
		if data*10 == 100 {
			return 100
		}
	}
	return -100
}

func TestDefer(t *testing.T) {
	df(2)
	df(1)
}
