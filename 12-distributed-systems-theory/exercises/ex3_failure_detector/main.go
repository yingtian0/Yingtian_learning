// ex3: 故障検出器 (ハートビートと、タイムアウト) のトレードオフ (シミュレーション)
//
// 送信側は、100 ms ごとに、ハートビートを送る。届く時刻は、遅延を伴う。遅延: 基本 5 ms + 対数正規分布のゆらぎ。
// 加えて、ときどき (確率 0.2%)、長い停止 (200〜2000 ms。GC の停止、パケットの喪失と再送、CPU のスロットリングなどを想定: 第8、11回) が入る。
// 故障していない (落ちていない) のに、「落ちた」と疑う = 誤検出。落ちたとき、気づくまでの時間 = 検出時間。
//
//	固定のタイムアウト T: 最後のハートビートから T ms 届かなければ、疑う
//	φ (ファイ) 累積検出器: 過去の到着間隔の、平均と標準偏差から、「この遅れが、普通でない度合い」を、φ で表し、閾値で疑う
//
// これは、モデルである。実際のネットワークの遅延の分布ではない。
// 実行: go run ./ex3_failure_detector
package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

const interval = 100.0

func arrivals(n int, seed int64) []float64 {
	rng := rand.New(rand.NewSource(seed))
	a := make([]float64, n)
	prev := 0.0
	for i := range a {
		d := 5 + math.Exp(rng.NormFloat64()*0.5+0.5)
		if rng.Float64() < 0.002 {
			d += 200 + rng.Float64()*1800
		}
		t := float64(i)*interval + d
		if t < prev { // 順序は保つ (TCP のように、追い越さない)
			t = prev
		}
		a[i], prev = t, t
	}
	return a
}

// P(X > x) の、正規分布の上側確率
func upper(x, mean, sd float64) float64 { return 0.5 * math.Erfc((x-mean)/(sd*math.Sqrt2)) }

// φ = −log10 P(間隔 > t) となる t (閾値の時間)
func phiThreshold(mean, sd, phi float64) float64 {
	lo, hi := mean, mean+100*sd+1
	for i := 0; i < 80; i++ {
		mid := (lo + hi) / 2
		if -math.Log10(upper(mid, mean, sd)+1e-300) < phi {
			lo = mid
		} else {
			hi = mid
		}
	}
	return hi
}

func main() {
	const n = 360000 // 10 時間分
	a := arrivals(n, 1)
	gaps := make([]float64, n-1)
	for i := 1; i < n; i++ {
		gaps[i-1] = a[i] - a[i-1]
	}
	sorted := append([]float64(nil), gaps...)
	sort.Float64s(sorted)
	fmt.Printf("到着間隔 (10 時間分、%d 個): p50 = %.0f ms、p99 = %.0f ms、p99.9 = %.0f ms、最大 = %.0f ms\n\n", n, sorted[n/2], sorted[n*99/100], sorted[n*999/1000], sorted[n-2])

	fmt.Println("固定のタイムアウト T")
	fmt.Printf("  %-10s %-26s %-22s\n", "T (ms)", "誤検出 (10 時間の、回数)", "検出時間 (平均、概算)")
	for _, T := range []float64{150, 200, 300, 500, 1000, 2000, 3000} {
		fp := 0
		for _, g := range gaps {
			if g > T {
				fp++
			}
		}
		// 落ちたとき: 最後のハートビート (間隔の中の、ランダムな時点) から T 後に気づく。平均 ≈ T − interval/2 + 平均の遅延
		fmt.Printf("  %-10.0f %-26d %-22.0f\n", T, fp, T-interval/2)
	}

	fmt.Println("\nφ 累積検出器 (直近 1000 個の間隔の、平均と標準偏差。正規分布を仮定)")
	fmt.Printf("  %-10s %-26s %-22s\n", "φ の閾値", "誤検出 (10 時間の、回数)", "検出時間 (平均、概算)")
	const W = 1000
	for _, phi := range []float64{1, 2, 4, 8, 16} {
		fp := 0
		var sumThr float64
		cnt := 0
		var sum, sum2 float64
		for i, g := range gaps {
			if i >= W { // 直近 W 個の窓 (今の間隔 g は、まだ入れない)
				mean := sum / W
				sd := math.Sqrt(math.Max(sum2/W-mean*mean, 1))
				thr := phiThreshold(mean, sd, phi)
				sumThr += thr
				cnt++
				if g > thr {
					fp++
				}
			}
			sum += g
			sum2 += g * g
			if i >= W {
				sum -= gaps[i-W]
				sum2 -= gaps[i-W] * gaps[i-W]
			}
		}
		fmt.Printf("  %-10.0f %-26d %-22.0f\n", phi, fp, sumThr/float64(cnt)-interval/2)
	}
	fmt.Println("\n  → 誤検出を減らそうとして、タイムアウトを長くすると、検出が遅くなる。逆も同じ。")
	fmt.Println("    『遅い』と『死んでいる』は、外から見分けられない (FLP の議論、本文 §6)。検出器は、『疑う』だけで、『死んだ』と断定は、できない。")
}
