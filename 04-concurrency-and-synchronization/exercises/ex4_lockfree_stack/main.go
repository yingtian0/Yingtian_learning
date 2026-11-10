// ex4: ロックフリーのスタック (Treiber stack) と、mutex のスタックの比較
//
// ロックフリー: head ポインタを CAS (compare-and-swap) で書き換える。
//
//	Push: 新しいノードの next を今の head にして、head が変わっていなければ head を新ノードにする。
//	      変わっていたら (他のスレッドが先に更新した) やり直す。
//
// どちらの実装も、ノードを1つずつ確保する (公平な比較のため)。
//
// 正しさの確認: 各 goroutine が一意な値を push し、最後に全部 pop して、個数と合計が合うか調べる。
package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type node struct {
	val  int
	next *node
}

// ---- ロックフリー ----
type LFStack struct{ head atomic.Pointer[node] }

func (s *LFStack) Push(v int) {
	n := &node{val: v}
	for {
		old := s.head.Load()
		n.next = old
		if s.head.CompareAndSwap(old, n) {
			return
		}
	}
}

func (s *LFStack) Pop() (int, bool) {
	for {
		old := s.head.Load()
		if old == nil {
			return 0, false
		}
		if s.head.CompareAndSwap(old, old.next) {
			return old.val, true
		}
	}
}

// ---- mutex ----
type MuStack struct {
	mu   sync.Mutex
	head *node
}

func (s *MuStack) Push(v int) {
	n := &node{val: v}
	s.mu.Lock()
	n.next = s.head
	s.head = n
	s.mu.Unlock()
}

func (s *MuStack) Pop() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.head == nil {
		return 0, false
	}
	n := s.head
	s.head = n.next
	return n.val, true
}

type Stack interface {
	Push(int)
	Pop() (int, bool)
}

// 正しさ: 全部 push → 全部 pop して、個数と合計が合うか
func check(s Stack, g, per int) bool {
	var wg sync.WaitGroup
	for id := 0; id < g; id++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < per; i++ {
				s.Push(id*per + i)
			}
		}(id)
	}
	wg.Wait()
	var sum, cnt int64
	var mu sync.Mutex
	for id := 0; id < g; id++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var ls, lc int64
			for {
				v, ok := s.Pop()
				if !ok {
					break
				}
				ls += int64(v)
				lc++
			}
			mu.Lock()
			sum += ls
			cnt += lc
			mu.Unlock()
		}()
	}
	wg.Wait()
	total := int64(g * per)
	want := total * (total - 1) / 2
	return cnt == total && sum == want
}

// 速度: 各 goroutine が push と pop を交互に行う。全体の操作回数は一定。
func bench(s Stack, g int, totalOps int) float64 {
	per := totalOps / g
	var wg sync.WaitGroup
	start := time.Now()
	for id := 0; id < g; id++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < per; i++ {
				s.Push(i)
				s.Pop()
			}
		}()
	}
	wg.Wait()
	return float64(per*g*2) / time.Since(start).Seconds() / 1e6
}

func main() {
	fmt.Printf("論理CPU = %d、GOMAXPROCS = %d\n\n", runtime.NumCPU(), runtime.GOMAXPROCS(0))
	for _, c := range []struct {
		name string
		mk   func() Stack
	}{
		{"ロックフリー (CAS)", func() Stack { return &LFStack{} }},
		{"mutex", func() Stack { return &MuStack{} }},
	} {
		fmt.Printf("%-20s 正しさ: %v\n", c.name, check(c.mk(), 8, 100000))
	}
	fmt.Printf("\n%-10s %18s %18s   (Mops/s: push+pop を 1 回ずつで 2 操作と数える)\n", "goroutine", "ロックフリー", "mutex")
	const totalOps = 2000000
	for _, g := range []int{1, 2, 4, 8, 16} {
		lf := bench(&LFStack{}, g, totalOps)
		mu := bench(&MuStack{}, g, totalOps)
		fmt.Printf("%-10d %18.2f %18.2f\n", g, lf, mu)
	}
}
