// ex3: 再試行の嵐と、準安定な障害 (metastable failure) — 時間刻みのシミュレーション
//
// サーバー: 容量 1,000 リクエスト/秒、先着順の待ち行列。クライアント: 新規のリクエストが、平均 800/秒 (使用率 80%) で到着。
// 各試行は、1 秒でタイムアウト。タイムアウトしたら、方針に従って、再試行 (最大 3 回)。
// きっかけ (トリガー): 10 秒〜15 秒の間、サーバーの容量が、半分 (500/秒) になる (例: 障害、GC、スロットリング)。15 秒に、容量は元に戻る。
//
// 方針:
//  1. 再試行なし
//  2. 即時に再試行 (最大 3 回)
//  3. 指数バックオフ + ジッター
//  4. 指数バックオフ + ジッター + 再試行の予算 (新規の 10% まで)
//  5. 即時に再試行 + サーバーが、期限切れのリクエストを捨てる (クライアントが、すでに諦めたものは、処理しない)
//
// 見るもの: きっかけが消えた (15 秒) あとも、成功したリクエスト (goodput) が、戻らない状態 = 準安定な障害。
// モデルである。実際のシステムの定数ではない。
// 実行: go run ./ex3_retry_storm
package main

import (
	"fmt"
	"math"
	"math/rand"
)

type attempt struct {
	tries    int
	deadline float64
	done     bool
	first    float64
}

type policy struct {
	name      string
	retries   int
	backoff   bool
	budget    bool
	dropStale bool
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

func run(pol policy, total float64) (goodput, offered, qlen []float64) {
	rng := rand.New(rand.NewSource(1))
	const dt = 0.01
	const timeout = 1.0
	var queue []*attempt
	type timer struct {
		at float64
		a  *attempt
	}
	var timers []timer
	type retryEv struct {
		at    float64
		tries int
		first float64
	}
	var retries []retryEv
	tokens := 0.0
	steps := int(total / dt)
	goodput = make([]float64, int(total))
	offered = make([]float64, int(total))
	qlen = make([]float64, int(total))
	carry := 0.0
	for s := 0; s < steps; s++ {
		now := float64(s) * dt
		sec := int(now)
		// 新規のリクエスト
		var newAtt []*attempt
		for i := poisson(rng, 800*dt); i > 0; i-- {
			newAtt = append(newAtt, &attempt{deadline: now + timeout, first: now})
			tokens = math.Min(tokens+0.1, 100)
		}
		// 再試行のうち、時刻が来たもの
		var rest []retryEv
		for _, r := range retries {
			if r.at <= now {
				newAtt = append(newAtt, &attempt{tries: r.tries, deadline: now + timeout, first: r.first})
			} else {
				rest = append(rest, r)
			}
		}
		retries = rest
		for _, a := range newAtt {
			queue = append(queue, a)
			timers = append(timers, timer{a.deadline, a})
			offered[sec]++
		}
		// サーバー: 容量
		capacity := 1000.0
		if now >= 10 && now < 15 {
			capacity = 500
		}
		carry += capacity * dt
		n := int(carry)
		carry -= float64(n)
		for n > 0 && len(queue) > 0 {
			a := queue[0]
			queue = queue[1:]
			if a.deadline < now { // クライアントは、すでに諦めた
				if pol.dropStale {
					continue // 期限切れは、捨てる (容量を使わない)
				}
				n-- // 無駄な処理に、容量を使う
				continue
			}
			n--
			a.done = true
			goodput[sec]++
		}
		// タイムアウト
		var tr []timer
		for _, t := range timers {
			if t.a.done {
				continue
			}
			if t.at <= now {
				a := t.a
				if a.tries < pol.retries {
					if pol.budget {
						if tokens < 1 {
							continue
						}
						tokens--
					}
					delay := 0.0
					if pol.backoff {
						delay = rng.Float64() * 0.1 * math.Pow(2, float64(a.tries+1)) // 全ジッター: 0〜(100 ms × 2^k)
					}
					retries = append(retries, retryEv{now + delay, a.tries + 1, a.first})
				}
			} else {
				tr = append(tr, t)
			}
		}
		timers = tr
		if s%100 == 0 {
			qlen[sec] = float64(len(queue))
		}
	}
	return
}

func main() {
	pols := []policy{
		{"1. 再試行なし", 0, false, false, false},
		{"2. 即時に再試行 (最大 3 回)", 3, false, false, false},
		{"3. 指数バックオフ + ジッター", 3, true, false, false},
		{"4. バックオフ + ジッター + 再試行の予算 (10%)", 3, true, true, false},
		{"5. 即時に再試行 + サーバーが期限切れを捨てる", 3, false, false, true},
	}
	fmt.Println("容量 1,000/秒、新規 800/秒。10〜15 秒の間だけ、容量が 500/秒。各秒の、成功したリクエスト数 (goodput、/秒)")
	times := []int{5, 9, 11, 14, 16, 18, 20, 25, 30, 45, 60, 90}
	fmt.Printf("  %-46s", "方針 \\ 経過 (秒)")
	for _, t := range times {
		fmt.Printf(" %5d", t)
	}
	fmt.Println("  | 15 秒以降で goodput が、700 に戻った時刻")
	var qrows []string
	for _, p := range pols {
		g, off, q := run(p, 120)
		fmt.Printf("  %-46s", p.name)
		for _, t := range times {
			fmt.Printf(" %5.0f", g[t])
		}
		rec := "戻らない (120 秒まで)"
		for t := 16; t < 118; t++ {
			if g[t] >= 700 && g[t+1] >= 700 && g[t+2] >= 700 {
				rec = fmt.Sprintf("%d 秒", t)
				break
			}
		}
		fmt.Printf("  | %s\n", rec)
		row := fmt.Sprintf("  %-46s", p.name)
		for _, t := range times {
			row += fmt.Sprintf(" %5.0f", off[t])
		}
		qrow := fmt.Sprintf("  %-46s", p.name)
		for _, t := range times {
			qrow += fmt.Sprintf(" %5.0f", q[t])
		}
		qrows = append(qrows, row+"\n"+qrow)
	}
	fmt.Println("\n各秒に、サーバーに届いた試行の数 (上の行) と、その秒の、待ち行列の長さ (下の行)")
	fmt.Printf("  %-46s", "方針 \\ 経過 (秒)")
	for _, t := range times {
		fmt.Printf(" %5d", t)
	}
	fmt.Println()
	for _, r := range qrows {
		fmt.Println(r)
	}
	fmt.Println("\n  → 即時の再試行は、きっかけが消えても、goodput が戻らない (準安定)。待ち行列が、期限切れのリクエストで埋まり、処理しても、クライアントはもう諦めている (無駄な仕事)。")
	fmt.Println("    再試行の予算と、バックオフ、期限切れを捨てること (デッドラインの伝播) が、回復を可能にする。")
}
