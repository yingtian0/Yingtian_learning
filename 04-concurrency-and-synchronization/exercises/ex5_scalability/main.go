// ex5: 共有カウンタはスケールするか
//
// 全体で 4000 万回の +1 を、goroutine の数を変えて分担する。
//
//	mutex     : 全員が1つのカウンタを mutex で保護
//	atomic    : 全員が1つのカウンタを atomic.Add で更新
//	sharded   : goroutine ごとに別のカウンタ (キャッシュラインを分ける)。最後に合計
//
// goroutine を増やして速くなるのは、共有をなくした sharded だけ、というのが予想。
package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const totalOps = 40000000

type padded struct {
	n uint64
	_ [120]byte // 128 バイトに揃える (第1回: false sharing 対策)
}

func run(g int, work func(id, n int)) float64 {
	per := totalOps / g
	var wg sync.WaitGroup
	start := time.Now()
	for id := 0; id < g; id++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			work(id, per)
		}(id)
	}
	wg.Wait()
	return float64(per*g) / time.Since(start).Seconds() / 1e6
}

func main() {
	fmt.Printf("論理CPU = %d、全体 %d 回\n\n", runtime.NumCPU(), totalOps)
	fmt.Printf("%-10s %12s %12s %12s   (Mops/s)\n", "goroutine", "mutex", "atomic", "sharded")
	for _, g := range []int{1, 2, 4, 8, 10, 16} {
		var mu sync.Mutex
		var c1 uint64
		m := run(g, func(id, n int) {
			for i := 0; i < n; i++ {
				mu.Lock()
				c1++
				mu.Unlock()
			}
		})

		var c2 atomic.Uint64
		a := run(g, func(id, n int) {
			for i := 0; i < n; i++ {
				c2.Add(1)
			}
		})

		shards := make([]padded, g)
		s := run(g, func(id, n int) {
			for i := 0; i < n; i++ {
				shards[id].n++
			}
		})
		var sum uint64
		for i := range shards {
			sum += shards[i].n
		}
		if sum != uint64(totalOps/g*g) || c1 != uint64(totalOps/g*g) || c2.Load() != uint64(totalOps/g*g) {
			fmt.Println("合計が合わない!")
		}
		fmt.Printf("%-10d %12.1f %12.1f %12.1f\n", g, m, a, s)
	}
}
