// ex4: 複製の方式と、書き込みの遅延 (モンテカルロ・シミュレーション)
//
//	チェーン複製 (chain replication): クライアント → head → … → tail → クライアント。書き込みは、鎖に沿って、直列に伝わる
//	多数派複製 (Raft / Paxos 型): クライアント → リーダー → (全フォロワーへ並列) → 過半数の ack → クライアント
//	全員の ack を待つ複製: リーダーが、全フォロワーの ack を待つ
//
// 1 ホップの遅延 = 0.5 ms + 指数分布 (平均 0.5 ms)。さらに、各ノードは、確率 q で、50 ms の停止 (GC、スロットリングなどを想定。第11回) を、そのホップに足す。
// これは、モデルである。実際のネットワークとノードの特性ではない。
// 実行: go run ./ex4_replication_latency
package main

import (
	"fmt"
	"math/rand"
	"sort"
)

var rng = rand.New(rand.NewSource(1))

var q = 0.0

func hop() float64 {
	d := 0.5 + rng.ExpFloat64()*0.5
	if rng.Float64() < q {
		d += 50
	}
	return d
}

func chain(n int) float64 { // クライアント → head、n-1 回の転送、tail → クライアント
	t := 0.0
	for i := 0; i < n+1; i++ {
		t += hop()
	}
	return t
}

func majority(n int) float64 {
	rtts := make([]float64, 0, n-1)
	for i := 0; i < n-1; i++ {
		rtts = append(rtts, hop()+hop())
	}
	sort.Float64s(rtts)
	need := n/2 + 1 - 1 // リーダー自身が 1 票
	return hop() + rtts[need-1] + hop()
}

func allAck(n int) float64 {
	m := 0.0
	for i := 0; i < n-1; i++ {
		if r := hop() + hop(); r > m {
			m = r
		}
	}
	return hop() + m + hop()
}

func pct(a []float64, p float64) float64 { return a[int(p*float64(len(a)-1))] }

func main() {
	const N = 400000
	for _, qq := range []float64{0, 0.01, 0.05} {
		q = qq
		fmt.Printf("各ホップで、%.0f%% の確率で、50 ms の停止が入る場合 (書き込みの遅延、ms)\n", qq*100)
		fmt.Printf("  %-34s %8s %8s %8s %9s\n", "方式", "p50", "p99", "p99.9", "平均")
		for _, n := range []int{3, 5} {
			for _, m := range []struct {
				name string
				f    func(int) float64
			}{{"チェーン複製", chain}, {"多数派複製", majority}, {"全員の ack を待つ", allAck}} {
				xs := make([]float64, N)
				sum := 0.0
				for i := range xs {
					xs[i] = m.f(n)
					sum += xs[i]
				}
				sort.Float64s(xs)
				fmt.Printf("  %-26s (N=%d) %8.2f %8.2f %8.2f %9.2f\n", m.name, n, pct(xs, 0.5), pct(xs, 0.99), pct(xs, 0.999), sum/N)
			}
		}
		fmt.Println()
	}
	fmt.Println("  → 停止のない理想的な場合は、チェーンは、ホップ数に比例して遅く、多数派は、並列で速い (中央値の差)。")
	fmt.Println("    停止が入ると、チェーンは、鎖のどこかのノードが遅れれば遅れ (裾が悪化)、多数派は、遅いノードを、待たずに済む (裾に強い)。")
	fmt.Println("    ただし、チェーンは、読み (tail だけ) が、強い一貫性で、安く、帯域の使い方が、鎖に沿って均等 (本文 §6)。この実験は、書き込みの遅延だけを見ている。")
}
