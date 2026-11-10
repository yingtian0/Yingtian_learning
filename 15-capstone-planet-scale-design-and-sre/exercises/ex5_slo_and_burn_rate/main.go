// ex5: SLO、エラーバジェット、可用性の合成、バーンレートによる警報 (シミュレーション)
//
//	A. 可用性の合成: 直列に依存するサービスの数と、可用性。冗長化 (並列) の効果。月あたりの、許される停止時間
//	B. 30 日間 (43,200 分) の、1 分ごとのエラー数を、模擬する。SLO = 99.9% (エラーバジェット = 0.1%)。
//	   事象: 大きな障害 (10 分、エラー率 50%) 3 回、ゆっくりした劣化 (6 時間、エラー率 0.8% = バジェットを 8 倍の速さで消費) 4 回、
//	         短いスパイク (1 分、エラー率 20%) 30 回。平常時のエラー率 0.02%。トラフィック 6,000 リクエスト/分。
//	   警報の方針を比べる: 検出までの時間、見逃し、不要な呼び出し (呼び出す価値のない事象での警報)、検出までに使ったバジェット
//	C. バジェットの内訳
//
// モデルである。実際のトラフィックと、エラーの特性ではない。
// 実行: go run ./ex5_slo_and_burn_rate
package main

import (
	"fmt"
	"math"
	"math/rand"
)

func partA() {
	fmt.Println("A. 可用性の合成")
	fmt.Println("  各依存先が 99.9% のとき、リクエストが、N 個全てに、依存する (どれか 1 つが落ちたら失敗)。月 (30 日) の停止の目安:")
	fmt.Printf("  %-8s %-14s %-16s\n", "N", "可用性", "許される停止 (月)")
	for _, n := range []int{1, 3, 5, 10, 30, 100} {
		a := math.Pow(0.999, float64(n))
		fmt.Printf("  %-8d %-14.4f%% %-16s\n", n, a*100, fmtDur((1-a)*30*24*60))
	}
	fmt.Println("  → 自分の目標 (SLO) は、依存先の SLO より、高くできない (全てに、直列に依存するなら)。依存先を減らす、または、依存先が落ちても、動く設計 (縮退、キャッシュ) が要る。")
	fmt.Println("\n  冗長化 (並列): 2 つの独立なコピーの、どちらかが生きていればよい。独立でない部分 (共通の原因: 同じデプロイ、同じ設定) の割合 c を考える")
	fmt.Printf("  %-34s %-14s %-16s\n", "構成 (各コピーは 99.9%)", "可用性", "許される停止 (月)")
	for _, c := range []float64{0, 0.1, 0.5, 1.0} {
		// 各コピーの停止 (0.1%) のうち、c の割合は、共通の原因 (両方が同時に落ちる)、(1−c) は、独立
		common := 0.001 * c
		indep := 0.001 * (1 - c)
		a := (1 - common) * (1 - indep*indep)
		fmt.Printf("  %-34s %-14.5f%% %-16s\n", fmt.Sprintf("2 コピー、共通の原因 %.0f%%", c*100), a*100, fmtDur((1-a)*30*24*60))
	}
	fmt.Println("  → 共通の原因が、わずか 10% でも、可用性は、独立の場合 (99.9999%) から、99.989% へ落ちる。冗長化の効果は、障害の独立性で、決まる (第14回: 相関する故障)。")
}

func fmtDur(min float64) string {
	switch {
	case min < 1:
		return fmt.Sprintf("%.1f 秒", min*60)
	case min < 120:
		return fmt.Sprintf("%.1f 分", min)
	default:
		return fmt.Sprintf("%.1f 時間", min/60)
	}
}

type incident struct {
	start, end int
	kind       string
	rate       float64
}

const (
	minutes  = 43200
	traffic  = 6000
	slo      = 0.999
	baseRate = 0.0002
)

func poisson(rng *rand.Rand, l float64) int {
	if l > 50 {
		return int(math.Max(0, math.Round(l+rng.NormFloat64()*math.Sqrt(l))))
	}
	L, k, p := math.Exp(-l), 0, 1.0
	for {
		p *= rng.Float64()
		if p <= L {
			return k
		}
		k++
	}
}

func generate(seed int64) ([]int, []incident) {
	rng := rand.New(rand.NewSource(seed))
	var inc []incident
	place := func(n, dur int, kind string, rate float64) {
		for i := 0; i < n; i++ {
			s := rng.Intn(minutes - dur - 1)
			inc = append(inc, incident{s, s + dur, kind, rate})
		}
	}
	place(3, 10, "大きな障害", 0.5)
	place(4, 360, "ゆっくりした劣化", 0.008)
	place(30, 1, "短いスパイク", 0.2)
	rate := make([]float64, minutes)
	for i := range rate {
		rate[i] = baseRate
	}
	for _, in := range inc {
		for t := in.start; t < in.end; t++ {
			rate[t] += in.rate
		}
	}
	errs := make([]int, minutes)
	for t := range errs {
		errs[t] = poisson(rng, rate[t]*traffic)
	}
	return errs, inc
}

func main() {
	partA()
	fmt.Println("\nB. 警報の方針 (100 回の模擬 × 30 日。SLO 99.9%、バジェット = 0.1%)")
	type pol struct {
		name string
		fire func(errRatio func(w int, t int) float64, t int) bool
	}
	budget := 1 - slo
	pols := []pol{
		{"1. 5 分のエラー率 > 0.1% (SLO と同じ)", func(r func(int, int) float64, t int) bool { return r(5, t) > budget }},
		{"2. 5 分のエラー率 > 1% (バーン 10 倍)", func(r func(int, int) float64, t int) bool { return r(5, t) > 10*budget }},
		{"3. 1 時間のエラー率 > 1.44% (バーン 14.4 倍)", func(r func(int, int) float64, t int) bool { return r(60, t) > 14.4*budget }},
		{"4. 多窓・多バーンレート (1h & 5m で 14.4 倍、または 6h & 30m で 6 倍)", func(r func(int, int) float64, t int) bool {
			return (r(60, t) > 14.4*budget && r(5, t) > 14.4*budget) || (r(360, t) > 6*budget && r(30, t) > 6*budget)
		}},
	}
	type agg struct {
		detect map[string][]float64
		missed map[string]int
		total  map[string]int
		falseP float64
		pages  float64
		burned map[string][]float64
	}
	res := make([]agg, len(pols))
	for i := range res {
		res[i] = agg{map[string][]float64{}, map[string]int{}, map[string]int{}, 0, 0, map[string][]float64{}}
	}
	const runs = 100
	for seed := int64(0); seed < runs; seed++ {
		errs, inc := generate(seed)
		prefix := make([]float64, minutes+1)
		for t := 0; t < minutes; t++ {
			prefix[t+1] = prefix[t] + float64(errs[t])
		}
		errRatio := func(w, t int) float64 { // t までの w 分のエラー率
			lo := t - w + 1
			if lo < 0 {
				return 0
			}
			return (prefix[t+1] - prefix[lo]) / float64(w*traffic)
		}
		for pi, p := range pols {
			// 警報の「発火」: 条件が、偽から真になった時刻
			var fires []int
			prev := false
			for t := 0; t < minutes; t++ {
				cur := p.fire(errRatio, t)
				if cur && !prev {
					fires = append(fires, t)
				}
				prev = cur
			}
			res[pi].pages += float64(len(fires))
			for _, in := range inc {
				res[pi].total[in.kind]++
				found := -1
				for _, f := range fires {
					if f >= in.start && f <= in.end+60 {
						found = f
						break
					}
				}
				if found < 0 {
					res[pi].missed[in.kind]++
					continue
				}
				res[pi].detect[in.kind] = append(res[pi].detect[in.kind], float64(found-in.start))
				// 検出までに使ったバジェット (エラー数 ÷ 月のバジェット)
				e := 0.0
				for t := in.start; t <= found && t < minutes; t++ {
					e += float64(errs[t])
				}
				res[pi].burned[in.kind] = append(res[pi].burned[in.kind], e/(budget*float64(minutes*traffic))*100)
			}
			// 発火のうち、どの事象とも対応しないもの → 不要な呼び出し。短いスパイクにだけ対応するものも、「呼び出す価値がない」とみなす
			for _, f := range fires {
				worth := false
				for _, in := range inc {
					if in.kind != "短いスパイク" && f >= in.start && f <= in.end+60 {
						worth = true
					}
				}
				if !worth {
					res[pi].falseP++
				}
			}
		}
	}
	mean := func(a []float64) float64 {
		if len(a) == 0 {
			return math.NaN()
		}
		s := 0.0
		for _, x := range a {
			s += x
		}
		return s / float64(len(a))
	}
	fmt.Printf("  %-70s | %-26s | %-26s | %s\n", "方針", "大きな障害: 検出 (分) / 見逃し", "ゆっくりした劣化: 検出 (分) / 見逃し", "1 か月あたり: 警報 / うち不要")
	for i, p := range pols {
		r := res[i]
		fmt.Printf("  %-70s | %6.1f / %-3d (使ったバジェット %4.1f%%) | %6.1f / %-3d (%4.1f%%) | %5.1f / %5.1f\n", p.name,
			mean(r.detect["大きな障害"]), r.missed["大きな障害"], mean(r.burned["大きな障害"]),
			mean(r.detect["ゆっくりした劣化"]), r.missed["ゆっくりした劣化"], mean(r.burned["ゆっくりした劣化"]),
			r.pages/runs, r.falseP/runs)
	}
	fmt.Printf("  (見逃し = 全 %d 回の大きな障害 (100 回 × 3)、%d 回のゆっくりした劣化 (100 回 × 4) のうちの数。条件が、すでに真のところに、事象が重なり、新しい発火がなかった場合も、見逃しに数える。「不要」= 大きな障害にも、ゆっくりした劣化にも、対応しない警報)\n", 300, 400)

	fmt.Println("\nC. バジェットの内訳 (1 つの模擬の例。月のバジェット = 0.1% = 259,200 エラー)")
	errs, inc := generate(1)
	total := 0
	for _, e := range errs {
		total += e
	}
	kinds := map[string]float64{}
	for _, in := range inc {
		e := 0.0
		for t := in.start; t < in.end; t++ {
			e += in.rate * traffic
		}
		kinds[in.kind] += e
	}
	b := budget * float64(minutes*traffic)
	fmt.Printf("  事象ごとの、エラー (期待値): 大きな障害 %.0f (バジェットの %.1f%%)、ゆっくりした劣化 %.0f (%.1f%%)、短いスパイク %.0f (%.1f%%)、平常時のエラー %.0f (%.1f%%)\n",
		kinds["大きな障害"], kinds["大きな障害"]/b*100, kinds["ゆっくりした劣化"], kinds["ゆっくりした劣化"]/b*100, kinds["短いスパイク"], kinds["短いスパイク"]/b*100,
		baseRate*float64(minutes*traffic), baseRate*float64(minutes*traffic)/b*100)
	fmt.Printf("  この月の、実際の、エラーの合計: %d (バジェットの %.1f%%)。SLO を、守れたか: %v\n", total, float64(total)/b*100, float64(total) <= b)
	fmt.Println("\n  → 単純なしきい値は、不要な警報が多い (方針 1)、または、ゆっくりした劣化を見逃す (方針 2、3)。多窓・多バーンレートは、短い窓で「今も続いている」ことを確認し、長い窓で「バジェットを、有意に、使っている」ことを確認するので、不要な警報を抑えつつ、両方を検出する。")
}
