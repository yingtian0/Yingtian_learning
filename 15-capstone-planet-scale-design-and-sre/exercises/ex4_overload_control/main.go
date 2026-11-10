// ex4: 過負荷の制御 — 到着率が、容量を超えたとき、何を、どう捨てるか (時間刻みのシミュレーション)
//
// サーバー: 容量 1,000/秒。クライアントは、1 秒で諦める (それを過ぎた応答は、無駄)。リクエストの 20% は、重要 (critical)、80% は、そうでない。
// 到着率を、500〜3,000/秒 に変え、30 秒間、続ける (一時的ではなく、持続的な過負荷)。
//
//	A. 制御なし (先着順、待ち行列は、無制限。期限切れも、処理する)
//	B. 待ち行列の上限 (100 件)。超えたら、新しいものを、すぐ拒否する
//	C. 待ち時間の上限 (100 ms)。それを超えて、待ったものは、処理せずに捨てる
//	D. 適応的な後入れ先出し (LIFO): 待ち行列が 20 件を超えたら、新しいものから処理し、古いものは、捨てる
//	E. 優先度: 重要なものを、先に処理し、重要でないものは、待ち行列が 50 件を超えたら拒否する
//
// 見るもの: 成功したリクエスト (1 秒以内に応答できたもの) の数 (goodput)、その p99 遅延、重要なリクエストの成功率。
// モデルである。実際のシステムの定数ではない。
// 実行: go run ./ex4_overload_control
package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

type req struct {
	t    float64
	crit bool
}

func poisson(rng *rand.Rand, l float64) int {
	L, k, p := math.Exp(-l), 0, 1.0
	for {
		p *= rng.Float64()
		if p <= L {
			return k
		}
		k++
	}
}

type result struct {
	goodput, p99, critOK float64
}

func run(policy string, lambda float64) result {
	rng := rand.New(rand.NewSource(1))
	const dt = 0.01
	const total = 30.0
	var q []req
	var lat []float64
	okCrit, totCrit := 0, 0
	good := 0
	carry := 0.0
	for s := 0; s < int(total/dt); s++ {
		now := float64(s) * dt
		for i := poisson(rng, lambda*dt); i > 0; i-- {
			r := req{now + rng.Float64()*dt, rng.Float64() < 0.2}
			if r.crit {
				totCrit++
			}
			switch policy {
			case "B":
				if len(q) >= 100 {
					continue // 拒否
				}
			case "E":
				if !r.crit && len(q) >= 50 {
					continue
				}
			}
			if policy == "E" && r.crit {
				// 重要なものは、先頭側 (重要でないものより、前) に挿入
				idx := 0
				for idx < len(q) && q[idx].crit {
					idx++
				}
				q = append(q, req{})
				copy(q[idx+1:], q[idx:])
				q[idx] = r
				continue
			}
			q = append(q, r)
		}
		carry += 1000 * dt
		n := int(carry)
		carry -= float64(n)
		for n > 0 && len(q) > 0 {
			var r req
			switch {
			case policy == "D" && len(q) > 20:
				r = q[len(q)-1] // 新しいものから
				q = q[:len(q)-1]
			default:
				r = q[0]
				q = q[1:]
			}
			wait := math.Max(0, now-r.t) // 時間刻み (10 ms) の精度。同じ刻みの中で、処理されたものは、待ち 0 とする
			switch policy {
			case "C":
				if wait > 0.1 {
					continue // 捨てる (容量を使わない)
				}
			case "D", "B", "E":
				if wait > 1.0 {
					continue // 期限切れは、捨てる (D のみ、古いものを、実質的に捨てる)
				}
			}
			n--
			if wait <= 1.0 {
				good++
				lat = append(lat, wait*1000)
				if r.crit {
					okCrit++
				}
			}
		}
		if policy == "D" { // 古すぎるものを、先頭から捨てる
			for len(q) > 0 && now-q[0].t > 1.0 {
				q = q[1:]
			}
		}
	}
	sort.Float64s(lat)
	p := 0.0
	if len(lat) > 0 {
		p = lat[len(lat)*99/100]
	}
	return result{float64(good) / total, p, float64(okCrit) / float64(totCrit) * 100}
}

func main() {
	pols := []struct{ id, name string }{
		{"A", "A. 制御なし"},
		{"B", "B. 待ち行列の上限 100 (すぐ拒否)"},
		{"C", "C. 待ち時間 100 ms 超は捨てる"},
		{"D", "D. 適応的な LIFO"},
		{"E", "E. 優先度 + 重要でないものを拒否"},
	}
	loads := []float64{500, 800, 1000, 1200, 1500, 2000, 3000}
	fmt.Println("容量 1,000/秒。1 秒以内に応答できたリクエストの数 (goodput、/秒)。到着率 (/秒) ごと")
	fmt.Printf("  %-40s", "方針 \\ 到着率")
	for _, l := range loads {
		fmt.Printf(" %6.0f", l)
	}
	fmt.Println()
	res := map[string][]result{}
	for _, p := range pols {
		fmt.Printf("  %-40s", p.name)
		for _, l := range loads {
			r := run(p.id, l)
			res[p.id] = append(res[p.id], r)
			fmt.Printf(" %6.0f", r.goodput)
		}
		fmt.Println()
	}
	fmt.Println("\n成功したリクエストの p99 遅延 (ms)")
	fmt.Printf("  %-40s", "方針 \\ 到着率")
	for _, l := range loads {
		fmt.Printf(" %6.0f", l)
	}
	fmt.Println()
	for _, p := range pols {
		fmt.Printf("  %-40s", p.name)
		for i := range loads {
			fmt.Printf(" %6.0f", res[p.id][i].p99)
		}
		fmt.Println()
	}
	fmt.Println("\n重要なリクエストの成功率 (%)")
	fmt.Printf("  %-40s", "方針 \\ 到着率")
	for _, l := range loads {
		fmt.Printf(" %6.0f", l)
	}
	fmt.Println()
	for _, p := range pols {
		fmt.Printf("  %-40s", p.name)
		for i := range loads {
			fmt.Printf(" %6.1f", res[p.id][i].critOK)
		}
		fmt.Println()
	}
	fmt.Println("\n  → 制御なしは、容量を超えると、goodput が崩れる (処理しても、期限切れ)。上限、期限切れの破棄、LIFO は、goodput を、容量の近くに保つ。")
	fmt.Println("    優先度は、全体の goodput を保つだけでなく、重要なリクエストを、守る。何を捨てるか、を、設計で、決める。")
}
