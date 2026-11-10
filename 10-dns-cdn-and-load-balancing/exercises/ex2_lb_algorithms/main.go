// ex2: ロードバランシングのアルゴリズムを、シミュレーションで比べる
//
//	A. リクエストの振り分け: ラウンドロビン / ランダム / 最小接続 / 2 つ選んで空いている方 (P2C)
//	   10 台のサーバー、ポアソン到着。サービス時間は、指数分布と、裾の重い分布 (対数正規) の 2 通り。
//	   稼働率 70% と 90% で、応答時間の平均と p99 を比べる。
//	B. 一貫性ハッシュ: サーバーを 1 台足す / 減らしたとき、キーの何割が移動するか。負荷の偏りは。
//	   (剰余によるハッシュ / 一貫性ハッシュ (仮想ノード 1〜1000) / ランデブーハッシュ)
//
// これは、モデルの結果である。実際のロードバランサの測定ではない (演習3が、実際のプロキシ)。
// 実行: go run ./ex2_lb_algorithms
package main

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"sort"
)

// ---------------- A ----------------
type srv struct {
	fin []float64 // 処理中・待ちのリクエストの、完了時刻 (先頭が古い)
	end float64   // 最後のリクエストの完了時刻
}

func (s *srv) inflight(t float64) int {
	for len(s.fin) > 0 && s.fin[0] <= t {
		s.fin = s.fin[1:]
	}
	return len(s.fin)
}

type policy func(servers []*srv, t float64, i int, rng *rand.Rand) int

var policies = []struct {
	name string
	f    policy
}{
	{"ラウンドロビン", func(s []*srv, t float64, i int, rng *rand.Rand) int { return i % len(s) }},
	{"ランダム", func(s []*srv, t float64, i int, rng *rand.Rand) int { return rng.Intn(len(s)) }},
	{"最小接続 (全部を見る)", func(s []*srv, t float64, i int, rng *rand.Rand) int {
		best, bi := 1<<30, 0
		off := rng.Intn(len(s))
		for k := range s {
			j := (k + off) % len(s)
			if n := s[j].inflight(t); n < best {
				best, bi = n, j
			}
		}
		return bi
	}},
	{"P2C (2 つ選んで空いている方)", func(s []*srv, t float64, i int, rng *rand.Rand) int {
		a, b := rng.Intn(len(s)), rng.Intn(len(s))
		for b == a {
			b = rng.Intn(len(s))
		}
		if s[a].inflight(t) <= s[b].inflight(t) {
			return a
		}
		return b
	}},
}

func serviceTime(kind string, rng *rand.Rand) float64 {
	if kind == "exp" {
		return rng.ExpFloat64()
	}
	sigma := 1.5 // 対数正規。平均が 1 になるように mu を決める。裾が重い
	return math.Exp(-sigma*sigma/2 + sigma*rng.NormFloat64())
}

func simulate(p policy, kind string, rho float64, n int, seed int64) (mean, p99 float64) {
	const N = 10
	rng := rand.New(rand.NewSource(seed))
	servers := make([]*srv, N)
	for i := range servers {
		servers[i] = &srv{}
	}
	lambda := rho * N
	t := 0.0
	var resp []float64
	for i := 0; i < n; i++ {
		t += rng.ExpFloat64() / lambda
		si := p(servers, t, i, rng)
		s := servers[si]
		start := math.Max(t, s.end)
		s.end = start + serviceTime(kind, rng)
		s.fin = append(s.fin, s.end)
		if i >= n/10 { // 最初の 1 割は、助走として除く
			resp = append(resp, s.end-t)
		}
	}
	sort.Float64s(resp)
	var sum float64
	for _, r := range resp {
		sum += r
	}
	return sum / float64(len(resp)), resp[len(resp)*99/100]
}

func partA() {
	fmt.Println("A. リクエストの振り分け (10 台、サービス時間の平均 = 1。応答時間は、待ち + サービス。単位はサービス時間の平均)")
	for _, kind := range []struct{ k, name string }{{"exp", "サービス時間: 指数分布 (ばらつき小)"}, {"heavy", "サービス時間: 裾の重い分布 (対数正規 σ=1.5。ばらつき大)"}} {
		for _, rho := range []float64{0.7, 0.9} {
			fmt.Printf("\n  %s、稼働率 %.0f%%\n", kind.name, rho*100)
			fmt.Printf("    %-30s %10s %10s\n", "方式", "平均", "p99")
			for _, p := range policies {
				m, p99 := simulate(p.f, kind.k, rho, 400000, 1)
				fmt.Printf("    %-30s %10.2f %10.2f\n", p.name, m, p99)
			}
		}
	}
}

// ---------------- B ----------------
func h64(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	x := h.Sum64()
	x ^= x >> 30 // 混ぜ直し (FNV の、下位ビットの偏りを避ける)
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}

type ring struct {
	points []uint64
	owner  []int
}

func newRing(nodes []int, vnodes int) *ring {
	type pt struct {
		h uint64
		n int
	}
	var ps []pt
	for _, n := range nodes {
		for v := 0; v < vnodes; v++ {
			ps = append(ps, pt{h64(fmt.Sprintf("node-%d#%d", n, v)), n})
		}
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].h < ps[j].h })
	r := &ring{}
	for _, p := range ps {
		r.points = append(r.points, p.h)
		r.owner = append(r.owner, p.n)
	}
	return r
}

func (r *ring) get(key string) int {
	h := h64(key)
	i := sort.Search(len(r.points), func(i int) bool { return r.points[i] >= h })
	if i == len(r.points) {
		i = 0
	}
	return r.owner[i]
}

func rendezvous(nodes []int, key string) int {
	best, bn := uint64(0), nodes[0]
	for _, n := range nodes {
		if w := h64(fmt.Sprintf("%s|%d", key, n)); w >= best {
			best, bn = w, n
		}
	}
	return bn
}

func nodesRange(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func imbalance(assign []int, n int) float64 {
	cnt := make([]int, n+1)
	for _, a := range assign {
		cnt[a]++
	}
	max, tot := 0, 0
	for i := 0; i < n; i++ {
		tot += cnt[i]
		if cnt[i] > max {
			max = cnt[i]
		}
	}
	return float64(max) / (float64(tot) / float64(n))
}

func partB() {
	const keys = 100000
	ks := make([]string, keys)
	for i := range ks {
		ks[i] = fmt.Sprintf("user:%d", i)
	}
	fmt.Println("\nB. サーバーを 10 台 → 11 台に増やしたとき / 10 台 → 9 台に減らしたとき、キー (10 万個) の何割が、別のサーバーに移るか")
	fmt.Printf("  %-34s %14s %14s %16s\n", "方式", "増やす (10→11)", "減らす (10→9)", "負荷の偏り (最大/平均, 10 台)")
	row := func(name string, f func(nodes []int, key string) int) {
		n10, n11, n9 := nodesRange(10), nodesRange(11), nodesRange(9)
		a10 := make([]int, keys)
		moved11, moved9 := 0, 0
		for i, k := range ks {
			a10[i] = f(n10, k)
		}
		for i, k := range ks {
			if f(n11, k) != a10[i] {
				moved11++
			}
			if f(n9, k) != a10[i] {
				moved9++
			}
		}
		fmt.Printf("  %-34s %13.1f%% %13.1f%% %16.2f\n", name, float64(moved11)/keys*100, float64(moved9)/keys*100, imbalance(a10, 10))
	}
	row("剰余 (ハッシュ % サーバー数)", func(nodes []int, key string) int { return int(h64(key) % uint64(len(nodes))) })
	for _, v := range []int{1, 10, 100, 1000} {
		cache := map[int]*ring{}
		row(fmt.Sprintf("一貫性ハッシュ (仮想ノード %d)", v), func(nodes []int, key string) int {
			r, ok := cache[len(nodes)]
			if !ok {
				r = newRing(nodes, v)
				cache[len(nodes)] = r
			}
			return r.get(key)
		})
	}
	row("ランデブーハッシュ", rendezvous)
	fmt.Println("  理想: 増やす = 1/11 = 9.1%、減らす = 1/10 = 10.0%。剰余は、ほぼ全部が移る (10/11 = 90.9%、9/10 = 90.0% に近い)。")
}

func main() {
	partA()
	partB()
}
