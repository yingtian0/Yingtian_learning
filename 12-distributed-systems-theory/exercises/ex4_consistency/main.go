// ex4: 複製されたレジスタの一貫性 — 線形化可能性 (linearizability) の検査器を書いて、クォーラムを試す (シミュレーション)
//
// モデル: N = 3 個のレプリカ。クライアントは、ランダムなレプリカ (コーディネータ) に要求する。
//
//	書き込み: コーディネータが、全レプリカに送り、W 個の ack を待って、応答する。バージョンは、書き込みの開始順 (全体で一意)
//	読み出し: コーディネータが、R 個のレプリカに問い合わせ、その時点の値を集め、最大のバージョンの値を返す
//	ネットワークの遅延: 片道 0.5 + 指数分布 (平均 3 ms)
//
// 履歴 (各操作の、開始時刻、応答時刻、値) を作り、線形化可能か、Wing & Gong の方式 (深さ優先探索) で検査する。
//
//	線形化可能 = 各操作が、開始と応答の間のどこかの 1 点で、瞬時に実行されたように、全操作を一列に並べられる (読みは、直前の書き込みの値を返す)
//
// 見るもの: R + W > N (クォーラムが交わる) なら、線形化可能か? R + W <= N なら、どれくらい崩れるか?
// 実行: go run ./ex4_consistency
package main

import (
	"fmt"
	"math/rand"
	"sort"
)

type op struct {
	client     int
	write      bool
	inv, resp  float64
	val        int // 書き込み: 書く値 (= バージョン)。読み出し: 返した値
	ver        int
	coordinate int
}

const N = 3

func delay(rng *rand.Rand) float64 { return 0.5 + rng.ExpFloat64()*3 }

// 1 つの履歴を作る: clients 個のクライアントが、それぞれ opsPer 個の操作を、直列に行う
func genHistory(rng *rand.Rand, clients, opsPer, R, W int, think float64) []op {
	type sched struct {
		o        op
		toRep    [N]float64 // コーディネータ → レプリカ
		fromRep  [N]float64
		replicas []int // 読みで、問い合わせる先
	}
	var all []*sched
	for c := 0; c < clients; c++ {
		t := rng.Float64() * 5
		for k := 0; k < opsPer; k++ {
			s := &sched{}
			s.o = op{client: c, write: rng.Float64() < 0.5, inv: t}
			for r := 0; r < N; r++ {
				s.toRep[r], s.fromRep[r] = delay(rng), delay(rng)
			}
			// コーディネータは、レプリカの 1 つ。自分自身への通信は、ほぼ 0 の遅延
			co := rng.Intn(N)
			s.o.coordinate = co
			s.toRep[co], s.fromRep[co] = 0.01, 0.01
			s.replicas = rng.Perm(N)[:R]
			if !s.o.write { // 読みは、コーディネータ自身を、優先して含める
				found := false
				for _, r := range s.replicas {
					if r == co {
						found = true
					}
				}
				if !found {
					s.replicas[0] = co
				}
			}
			// 応答時刻
			var times []float64
			if s.o.write {
				for r := 0; r < N; r++ {
					times = append(times, s.toRep[r]+s.fromRep[r])
				}
				sort.Float64s(times)
				s.o.resp = s.o.inv + times[W-1]
			} else {
				m := 0.0
				for _, r := range s.replicas {
					if d := s.toRep[r] + s.fromRep[r]; d > m {
						m = d
					}
				}
				s.o.resp = s.o.inv + m
			}
			t = s.o.resp + rng.ExpFloat64()*think
			all = append(all, s)
		}
	}
	// バージョン: 書き込みの開始順
	sort.Slice(all, func(i, j int) bool { return all[i].o.inv < all[j].o.inv })
	ver := 0
	for _, s := range all {
		if s.o.write {
			ver++
			s.o.ver, s.o.val = ver, ver
		}
	}
	// レプリカごとの、書き込みの適用時刻
	type app struct {
		t   float64
		ver int
	}
	var apps [N][]app
	for _, s := range all {
		if s.o.write {
			for r := 0; r < N; r++ {
				apps[r] = append(apps[r], app{s.o.inv + s.toRep[r], s.o.ver})
			}
		}
	}
	stateAt := func(r int, t float64) int {
		v := 0
		for _, a := range apps[r] {
			if a.t <= t && a.ver > v {
				v = a.ver
			}
		}
		return v
	}
	var out []op
	for _, s := range all {
		if !s.o.write {
			best := 0
			for _, r := range s.replicas {
				if v := stateAt(r, s.o.inv+s.toRep[r]); v > best {
					best = v
				}
			}
			s.o.val, s.o.ver = best, best
		}
		out = append(out, s.o)
	}
	return out
}

// 線形化可能か (レジスタ、初期値 0)
func linearizable(h []op) bool {
	n := len(h)
	type key struct {
		mask uint32
		val  int
	}
	memo := map[key]bool{}
	var dfs func(mask uint32, val int) bool
	dfs = func(mask uint32, val int) bool {
		if mask == (1<<n)-1 {
			return true
		}
		k := key{mask, val}
		if r, ok := memo[k]; ok {
			return r
		}
		// まだ並べていない操作のうち、最も早い応答時刻
		minResp := 1e18
		for i := 0; i < n; i++ {
			if mask>>i&1 == 0 && h[i].resp < minResp {
				minResp = h[i].resp
			}
		}
		res := false
		for i := 0; i < n && !res; i++ {
			if mask>>i&1 == 1 || h[i].inv > minResp { // 他の操作が、これより前に終わっている → これは、まだ選べない
				continue
			}
			if h[i].write {
				res = dfs(mask|1<<i, h[i].val)
			} else if h[i].val == val {
				res = dfs(mask|1<<i, val)
			}
		}
		memo[k] = res
		return res
	}
	return dfs(0, 0)
}

// 古い読み: 読みの開始前に、応答済みの書き込みがあるのに、それより古いバージョンを返した
func staleReads(h []op) (stale, reads int) {
	for _, r := range h {
		if r.write {
			continue
		}
		reads++
		for _, w := range h {
			if w.write && w.resp < r.inv && w.ver > r.ver {
				stale++
				break
			}
		}
	}
	return
}

func main() {
	const histories = 4000
	fmt.Printf("レプリカ N = %d、4 クライアント × 3 操作 = 12 操作の履歴を %d 個ずつ。\n\n", N, histories)
	for _, think := range []float64{1, 20} {
		fmt.Printf("操作の間の待ち (平均) = %.0f ms (%s)\n", think, map[float64]string{1: "混んでいる: 操作が、よく重なる", 20: "空いている: 操作は、あまり重ならない"}[think])
		fmt.Printf("  %-8s %-8s %-6s | %-22s | %-22s\n", "R", "W", "R+W>N", "線形化できない履歴", "古い読み (読みの総数に対して)")
		for _, c := range []struct{ R, W int }{{1, 1}, {1, 2}, {2, 2}, {1, 3}, {3, 1}, {3, 3}} {
			rng := rand.New(rand.NewSource(7))
			bad, st, rd := 0, 0, 0
			for i := 0; i < histories; i++ {
				h := genHistory(rng, 4, 3, c.R, c.W, think)
				if !linearizable(h) {
					bad++
				}
				s, r := staleReads(h)
				st, rd = st+s, rd+r
			}
			fmt.Printf("  %-8d %-8d %-6v | %5d / %d (%5.1f%%)   | %5d / %d (%5.1f%%)\n", c.R, c.W, c.R+c.W > N, bad, histories, float64(bad)/histories*100, st, rd, float64(st)/float64(rd)*100)
		}
		fmt.Println()
	}
}
