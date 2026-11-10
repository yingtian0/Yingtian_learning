// ex2: 待ち行列 — 使用率と遅延の関係 (離散イベント・シミュレーション)
//
// 1 台のサーバー、先着順 (FIFO)、ポアソン到着 (到着率 λ)。処理時間の分布を変える (平均は、同じ 1 に正規化):
//
//	指数分布 (M/M/1)、一定 (M/D/1)、裾の重い対数正規 (σ = 1.5)
//
// 使用率 ρ = λ × 平均の処理時間。
//
//	A. 使用率と、遅延 (平均、p99)。M/M/1 の理論値 (平均の滞在時間 = 1 / (1 − ρ)) との比較
//	B. リトルの法則 (L = λ W) の確認: ランダムな時刻の、システム内の個数を数える
//
// 実行: go run ./ex2_queueing
package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

func sim(rho float64, dist string, n int, seed int64) (lat []float64, arr, dep []float64) {
	rng := rand.New(rand.NewSource(seed))
	service := func() float64 {
		switch dist {
		case "exp":
			return rng.ExpFloat64()
		case "det":
			return 1
		default: // 対数正規、平均 1: exp(μ + σ²/2) = 1 → μ = −σ²/2
			s := 1.5
			return math.Exp(rng.NormFloat64()*s - s*s/2)
		}
	}
	lambda := rho
	t, free := 0.0, 0.0
	for i := 0; i < n; i++ {
		t += rng.ExpFloat64() / lambda
		s := service()
		start := math.Max(t, free)
		free = start + s
		lat = append(lat, free-t)
		arr = append(arr, t)
		dep = append(dep, free)
	}
	return
}

func main() {
	const n = 2000000
	fmt.Println("A. 使用率と遅延 (平均の処理時間 = 1。滞在時間 = 待ち + 処理。200 万リクエスト)")
	fmt.Printf("  %-6s | %-24s | %-24s | %-24s | %s\n", "ρ", "指数 (M/M/1): 平均 / p99", "一定 (M/D/1): 平均 / p99", "裾の重い (σ=1.5): 平均 / p99", "M/M/1 の理論値 (平均)")
	for _, rho := range []float64{0.3, 0.5, 0.7, 0.8, 0.9, 0.95, 0.99} {
		row := fmt.Sprintf("  %-6.2f |", rho)
		for _, d := range []string{"exp", "det", "logn"} {
			lat, _, _ := sim(rho, d, n, 1)
			sum := 0.0
			for _, x := range lat {
				sum += x
			}
			sort.Float64s(lat)
			row += fmt.Sprintf(" %8.1f / %-13.1f |", sum/float64(len(lat)), lat[len(lat)*99/100])
		}
		fmt.Printf("%s %.1f\n", row, 1/(1-rho))
	}
	fmt.Println("  → 使用率が上がると、遅延は、直線ではなく、急激に増える (ρ = 0.9 で平均 10 倍、ρ = 0.99 で 100 倍)。処理時間のばらつきが大きいと、さらに悪い。")

	fmt.Println("\nB. リトルの法則: 平均の個数 L = λ × 平均の滞在時間 W (ρ = 0.8、指数)")
	lat, arr, dep := sim(0.8, "exp", n, 2)
	sum := 0.0
	for _, x := range lat {
		sum += x
	}
	W := sum / float64(len(lat))
	rng := rand.New(rand.NewSource(3))
	horizon := arr[len(arr)-1]
	samples, tot := 100000, 0.0
	for i := 0; i < samples; i++ {
		t := horizon*0.1 + rng.Float64()*horizon*0.8 // 立ち上がりと終わりを除く
		a := sort.SearchFloat64s(arr, t+1e-12)       // t 以前に到着した数
		d := sort.SearchFloat64s(dep, t+1e-12)       // t 以前に出発した数 (FIFO なので dep は昇順)
		tot += float64(a - d)
	}
	L := tot / float64(samples)
	fmt.Printf("  サンプルした時刻での、システム内の個数の平均 L = %.2f、λ × W = %.2f × %.2f = %.2f\n", L, 0.8, W, 0.8*W)
	fmt.Println("  → 2 つは、ほぼ一致する (独立な測り方で)。リトルの法則は、分布によらず成り立つ。同時に処理している数 = 到着率 × 滞在時間 (スレッド数、接続数の見積もりに使う)。")
}
