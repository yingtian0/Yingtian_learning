// ex5: CPU クォータ (cgroup の cpu.max) による、スロットリングの、シミュレーション
//
// 仕組み (CFS bandwidth control): コンテナは、周期 P (既定 100 ms) あたり、Q ms の CPU 時間を、使える (Q = 200 なら「2 CPU」)。
// 周期の中で、全スレッドの CPU 時間の合計が Q に達すると、残りの周期は、全スレッドが止まる (スロットリング)。次の周期で、再開する。
// 「平均の CPU 使用率が、上限より低い」のに、スレッドが並列に走ると、周期の前半で、クォータを使い切って、後半が止まる。
//
// モデル: 1 ms ずつ進める。リクエストは、ポアソン到着、必要な CPU 時間は、固定 (既定 20 ms)。
// 実行中のスレッドの数は、最大 T (= Go なら GOMAXPROCS)。各 ms で、実行中のスレッドの数だけ、クォータを消費する。
// これは、挙動を理解するためのモデルで、実際のカーネルの、スケジューラの詳細 (スライス、複数のコア間での、クォータの配布) は、省いている。
//
// 実行: go run ./ex5_cfs_throttle_sim
package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

type result struct {
	p50, p99, max float64
	throttledFrac float64
	util          float64
}

func simulate(quotaMs, periodMs, threads int, utilization float64, serviceMs int, seed int64, totalMs int) result {
	rng := rand.New(rand.NewSource(seed))
	// 平均の使用率 = λ × serviceMs / quotaMs × (quotaMs/periodMs) → λ [件/ms] = utilization × (quotaMs/periodMs) / serviceMs
	lambda := utilization * float64(quotaMs) / float64(periodMs) / float64(serviceMs)
	type req struct{ arrive, remain int }
	var queue []req // 待ち (到着順)
	var running []req
	var lat []float64
	quota := quotaMs
	throttledPeriods, periods := 0, 0
	throttledThis := false
	used := 0
	for t := 0; t < totalMs; t++ {
		if t%periodMs == 0 {
			if t > 0 {
				periods++
				if throttledThis {
					throttledPeriods++
				}
			}
			quota, throttledThis = quotaMs, false
		}
		// 到着 (1 ms あたり、ポアソン)
		for k := poisson(rng, lambda); k > 0; k-- {
			queue = append(queue, req{t, serviceMs})
		}
		for len(running) < threads && len(queue) > 0 {
			running = append(running, queue[0])
			queue = queue[1:]
		}
		if len(running) > 0 {
			if quota < len(running) { // クォータが足りない: 使い切って、残りは止まる
				for i := 0; i < quota; i++ {
					running[i].remain--
				}
				used += quota
				quota = 0
				throttledThis = true
			} else {
				for i := range running {
					running[i].remain--
				}
				quota -= len(running)
				used += len(running)
			}
			if quota == 0 {
				throttledThis = true
			}
			// 終了
			var next []req
			for _, r := range running {
				if r.remain <= 0 {
					lat = append(lat, float64(t+1-r.arrive))
				} else {
					next = append(next, r)
				}
			}
			running = next
		}
	}
	sort.Float64s(lat)
	pct := func(p float64) float64 { return lat[int(p*float64(len(lat)-1))] }
	return result{pct(0.5), pct(0.99), lat[len(lat)-1], float64(throttledPeriods) / float64(periods), float64(used) / float64(totalMs) / (float64(quotaMs) / float64(periodMs))}
}

func poisson(rng *rand.Rand, lambda float64) int {
	l, k, p := math.Exp(-lambda), 0, 1.0
	for {
		p *= rng.Float64()
		if p <= l {
			return k
		}
		k++
	}
}

func main() {
	const quota, period, service, total = 200, 100, 20, 600000
	fmt.Printf("CFS クォータのシミュレーション (上限 = %d ms / %d ms = 2 CPU、1 件の処理 = CPU %d ms、%d 秒分)\n", quota, period, service, total/1000)
	fmt.Println("  平均の使用率 = 上限に対する割合。 スレッド数 T = 同時に走れる数 (Go の GOMAXPROCS に相当)")
	fmt.Printf("  %-8s %-6s | %8s %8s %8s | %s\n", "使用率", "T", "p50(ms)", "p99(ms)", "最大(ms)", "スロットリングされた周期")
	for _, u := range []float64{0.3, 0.5, 0.7, 0.9} {
		for _, T := range []int{2, 4, 8, 16} {
			r := simulate(quota, period, T, u, service, 1, total)
			fmt.Printf("  %-8.0f%% %-6d | %8.0f %8.0f %8.0f | %5.1f%%\n", u*100, T, r.p50, r.p99, r.max, r.throttledFrac*100)
		}
		fmt.Println()
	}
	fmt.Println("  処理時間の下限は 20 ms (スロットリングがなければ、使用率 30% なら、p99 もこれに近い)")
}
