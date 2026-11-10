// ex1: 遅延の裾 (The Tail at Scale) — ファンアウトと、ヘッジされたリクエスト (モンテカルロ)
//
// 1 台のサーバーの応答時間: 中央値 約 10 ms の対数正規 (σ = 0.3)。さらに、1% の確率で、+100 ms の停止 (GC、スロットリングなど。第11回)。
// 1 つのリクエストが、N 台に、並列に問い合わせて (ファンアウト)、全ての応答を待つなら、リクエストの遅延は、N 台の最大。
//
//	A. ファンアウト N と、遅いリクエストの割合 (理論値 1 − 0.99^N と、実測)
//	B. ヘッジされたリクエスト: 一定時間待っても、応答がなければ、別のサーバーに、同じ要求を送り、先に来た方を採る。効果と、追加の負荷
//	C. 遅さの原因が、サーバーではなく、リクエストそのもの (重いクエリ) なら、ヘッジは、効かない (第14回の straggler と同じ)
//
// モデルである。実際のサービスの遅延分布ではない。
// 実行: go run ./ex1_tail_at_scale
package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

var rng = rand.New(rand.NewSource(1))

func server() float64 {
	t := 10 * math.Exp(rng.NormFloat64()*0.3)
	if rng.Float64() < 0.01 {
		t += 100
	}
	return t
}

func pct(a []float64, p float64) float64 { return a[int(p*float64(len(a)-1))] }

func partA() {
	fmt.Println("A. ファンアウト: N 台の最大 (各 20 万リクエスト)")
	fmt.Printf("  %-8s %8s %8s %8s | %-26s %s\n", "N", "p50", "p99", "p99.9", "100 ms 超の割合 (実測)", "理論値 1 − 0.99^N (停止に当たる確率)")
	const R = 200000
	for _, n := range []int{1, 10, 100, 1000} {
		xs := make([]float64, R)
		slow := 0
		for i := range xs {
			m := 0.0
			for j := 0; j < n; j++ {
				if t := server(); t > m {
					m = t
				}
			}
			xs[i] = m
			if m > 100 {
				slow++
			}
		}
		sort.Float64s(xs)
		fmt.Printf("  %-8d %8.1f %8.1f %8.1f | %-26s %.1f%%\n", n, pct(xs, 0.5), pct(xs, 0.99), pct(xs, 0.999), fmt.Sprintf("%.1f%%", float64(slow)/R*100), (1-math.Pow(0.99, float64(n)))*100)
	}
	fmt.Println("  → 1 台の 1% の停止が、N = 100 では、リクエストの 63% を遅くする。ファンアウトが大きいほど、「まれな」遅さが、「いつも」になる。")
}

func partB() {
	fmt.Println("\nB. ヘッジされたリクエスト (N = 100 のファンアウト。各サブリクエストが、h ms 待っても返らなければ、別のサーバーに、もう 1 つ送る。各 5 万リクエスト)")
	fmt.Printf("  %-26s %8s %8s %8s | %s\n", "ヘッジの待ち時間 h", "p50", "p99", "p99.9", "追加のサブリクエスト (全体に対して)")
	const R = 50000
	single := make([]float64, 200000)
	for i := range single {
		single[i] = server()
	}
	sort.Float64s(single)
	run := func(name string, h float64, hedge bool) {
		xs := make([]float64, R)
		extra, total := 0, 0
		for i := range xs {
			m := 0.0
			for j := 0; j < 100; j++ {
				t := server()
				total++
				if hedge && t > h {
					extra++
					if t2 := h + server(); t2 < t {
						t = t2
					}
				}
				if t > m {
					m = t
				}
			}
			xs[i] = m
		}
		sort.Float64s(xs)
		fmt.Printf("  %-26s %8.1f %8.1f %8.1f | %.1f%%\n", name, pct(xs, 0.5), pct(xs, 0.99), pct(xs, 0.999), float64(extra)/float64(total)*100)
	}
	run("ヘッジなし", 0, false)
	for _, q := range []float64{0.5, 0.9, 0.95, 0.99} {
		h := pct(single, q)
		run(fmt.Sprintf("h = 1 台の p%.0f (%.1f ms)", q*100, h), h, true)
	}
	fmt.Println("  → h = 1 台の p95 で、中央値は 108 → 26 ms に縮み、追加の負荷は 約 5%。h を短くしすぎると、追加の負荷が増える (h = p50 なら 約 50%)。")
	fmt.Println("    p99 は不安定: 「両方のコピーが、停止に当たる」確率 (約 1e-4 / サブリクエスト) が、h によらず、N = 100 では、約 1% のリクエストに起きる。p99 は、この崖の縁に、ちょうど位置する (h = p90 と p95 の差は、標本の揺れ)。p99.9 は、どの h でも、約 114 ms。")
}

func partC() {
	fmt.Println("\nC. 遅さの原因: サーバーか、リクエストか (N = 100、ヘッジ h = 16 ms、各 5 万リクエスト)")
	const R = 50000
	fmt.Printf("  %-52s %8s %8s %8s\n", "モデル", "p50", "p99", "p99.9")
	for _, intrinsic := range []bool{false, true} {
		for _, hedge := range []bool{false, true} {
			xs := make([]float64, R)
			for i := range xs {
				m := 0.0
				for j := 0; j < 100; j++ {
					// intrinsic: 1% のサブリクエストは、データが重く、どのサーバーに送っても、+100 ms かかる。サーバー側の、停止は、なし
					heavy := intrinsic && rng.Float64() < 0.01
					one := func() float64 {
						if intrinsic {
							t := 10 * math.Exp(rng.NormFloat64()*0.3)
							if heavy {
								t += 100
							}
							return t
						}
						return server()
					}
					t := one()
					if hedge && t > 16 {
						if t2 := 16 + one(); t2 < t {
							t = t2
						}
					}
					if t > m {
						m = t
					}
				}
				xs[i] = m
			}
			sort.Float64s(xs)
			name := "遅さの原因 = サーバーの停止 (ランダム)"
			if intrinsic {
				name = "遅さの原因 = リクエストが重い (どのサーバーでも遅い)"
			}
			if hedge {
				name += " + ヘッジ"
			} else {
				name += "、ヘッジなし"
			}
			fmt.Printf("  %-52s %8.1f %8.1f %8.1f\n", name, pct(xs, 0.5), pct(xs, 0.99), pct(xs, 0.999))
		}
	}
	fmt.Println("  → ヘッジは、「別のサーバーなら、速い」場合にだけ効く。原因が、リクエストの重さなら、効かず、負荷だけが増える。")
}

func main() {
	partA()
	partB()
	partC()
}
