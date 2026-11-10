// ex3: 2 相コミット (2PC) の、ブロッキングと、原子性 (モンテカルロ・シミュレーション)
//
// 5 つの参加者 (participant) と、1 つのコーディネータ。参加者は、準備 (prepare) に、95% の確率で「はい」と答える。
// 全員が「はい」なら、コーディネータは、コミットを決定 (ログに記録) して、全員に送る。1 人でも「いいえ」なら、中止 (abort)。
// コーディネータは、1 回のトランザクションあたり、1% の確率で、次の 4 つの時点のどれかで、クラッシュする (一様):
//
//	(1) 準備を送る前   (2) 全員の投票を集めた後、決定を記録する前   (3) 決定を記録した後、送る前   (4) 決定を一部の参加者にだけ送った後
//
// 参加者が、「はい」と答えた後 (準備済み)、決定を知るまでの状態 = 不確定 (in doubt)。ロックを持ったまま、待つ。
//
// 比べるもの:
//
//	A. 標準の 2PC (コーディネータが、1 台。クラッシュ後の復旧に、平均 10 秒)
//	B. コーディネータを、複製 (Paxos / Raft で、状態を複製) した場合 (新しいリーダーが、0.3 秒で引き継ぐ。第13回の、Spanner の構成)
//	C. 壊した版: 準備済みの参加者が、タイムアウト (1 秒) で、独断で中止する (「待ちたくない」)
//
// 時間の数値は、モデルの仮定である。実システムの測定ではない。
// 実行: go run ./ex3_two_phase_commit
package main

import (
	"fmt"
	"math/rand"
	"sort"
)

type result struct {
	txns, commit, abort, crashed int
	blockedTxns                  int
	lockTimes                    []float64 // 不確定のまま、ロックを持っていた時間 (秒) — 準備済みの参加者ごとの最大
	violations                   int
}

func simulate(variant string, txns int, seed int64) result {
	rng := rand.New(rand.NewSource(seed))
	var r result
	r.txns = txns
	for i := 0; i < txns; i++ {
		allYes := true
		for p := 0; p < 5; p++ {
			if rng.Float64() > 0.95 {
				allYes = false
			}
		}
		crash := rng.Float64() < 0.01
		point := 0
		if crash {
			point = 1 + rng.Intn(4)
			r.crashed++
		}
		// 通常: 準備 1 往復 + 決定 1 往復 = 約 10 ms の間、準備済みの参加者が、ロックを持つ
		if !crash {
			if allYes {
				r.commit++
			} else {
				r.abort++
			}
			r.lockTimes = append(r.lockTimes, 0.010)
			continue
		}
		var down float64
		if variant == "B" {
			down = 0.3
		} else {
			down = rng.ExpFloat64() * 10
		}
		decidedCommit := allYes && point >= 3 // 決定が、ログに記録された
		switch point {
		case 1: // 準備を送る前: 誰も準備していない。参加者は、タイムアウトで、中止できる
			r.abort++
			r.lockTimes = append(r.lockTimes, 0)
		case 2: // 投票を集めた後、決定を記録する前: 復旧後、コーディネータは、決定を持たない → 中止
			r.abort++
			if variant == "C" {
				r.lockTimes = append(r.lockTimes, 1.0) // 独断で、1 秒後に中止 (結果として、中止と一致するので、違反ではない)
			} else {
				r.blockedTxns++
				r.lockTimes = append(r.lockTimes, down)
			}
		case 3: // 決定を記録した後、送る前: 復旧後、ログから、コミットを再送
			if decidedCommit {
				r.commit++
			} else {
				r.abort++
			}
			if variant == "C" {
				r.lockTimes = append(r.lockTimes, 1.0)
				if decidedCommit { // 参加者は、中止したのに、コーディネータは、コミットを再送する → 原子性の違反
					r.violations++
				}
			} else {
				r.blockedTxns++
				r.lockTimes = append(r.lockTimes, down)
			}
		case 4: // 決定を、一部にだけ送った後
			if decidedCommit {
				r.commit++
			} else {
				r.abort++
			}
			delivered := 1 + rng.Intn(4) // 1〜4 人が、決定を知った
			_ = delivered
			if variant == "C" {
				r.lockTimes = append(r.lockTimes, 1.0)
				if decidedCommit { // 一部は、コミット済み、残りは、独断で中止 → 違反
					r.violations++
				}
			} else {
				// 協調的な終了 (cooperative termination): 決定を知った参加者に、他の参加者が、問い合わせる (100 ms のタイムアウト)
				r.lockTimes = append(r.lockTimes, 0.1)
			}
		}
	}
	return r
}

func pct(a []float64, p float64) float64 {
	if len(a) == 0 {
		return 0
	}
	s := append([]float64(nil), a...)
	sort.Float64s(s)
	return s[int(p*float64(len(s)-1))]
}

func main() {
	const txns = 1000000
	fmt.Printf("2PC: %d 個のトランザクション (参加者 5、コーディネータの、クラッシュ確率 1%%/トランザクション)\n\n", txns)
	fmt.Printf("  %-44s %-9s %-9s %-8s %-10s | %-26s | %s\n", "版", "コミット", "中止", "クラッシュ", "ブロック", "ロック保持 p50 / p99 / 最大 (秒)", "原子性の違反")
	for _, v := range []struct{ id, name string }{
		{"A", "A. 標準の 2PC (コーディネータ 1 台、復旧 平均 10 秒)"},
		{"B", "B. コーディネータを複製 (引き継ぎ 0.3 秒)"},
		{"C", "C. 壊した版: 準備済みの参加者が、1 秒で、独断で中止"},
	} {
		r := simulate(v.id, txns, 1)
		fmt.Printf("  %-44s %-9d %-9d %-8d %-10d | %7.3f / %6.1f / %7.1f   | %d\n", v.name, r.commit, r.abort, r.crashed, r.blockedTxns, pct(r.lockTimes, 0.5), pct(r.lockTimes, 0.99999), pct(r.lockTimes, 1), r.violations)
	}
	fmt.Println("\n  → 標準の 2PC は、原子性は守るが、コーディネータの故障で、準備済みの参加者が、ロックを持ったまま、復旧まで待つ (ブロッキング)。")
	fmt.Println("    コーディネータの状態を、複製すれば、ブロックの時間が、数十分の 1 に縮む。独断で中止すると、原子性が破れる。")
}
