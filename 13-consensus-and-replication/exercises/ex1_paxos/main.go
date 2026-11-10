// ex1: 単一値の Paxos (single-decree) の、離散イベント・シミュレーション
//
// 5 つのアクセプタ、3 つのプロポーザ (それぞれ、別の値 A、B、C を、提案する)。
// メッセージ: 遅延 (1〜10 ms)、喪失 (10%)、重複 (5%)、順序の入れ替わり。アクセプタは、ときどき、クラッシュして、再起動する。
//
// 「選ばれた (chosen)」の定義 (外から観測した、正解): ある提案番号 b で、過半数 (3 つ以上) のアクセプタが、同じ値を受理した。
// 安全性: 選ばれた値は、高々 1 つ。
//
// 比べるもの:
//  1. 正しい Paxos + ランダムなバックオフ
//  2. 正しい Paxos + バックオフなし (全プロポーザが、同じ固定のタイムアウトで再試行する)
//  3. 壊した版 (a): プロポーザが、フェーズ 1 で、既に受理された値を見ても、無視して、自分の値を提案する
//  4. 壊した版 (b): アクセプタが、再起動で、状態 (約束した番号、受理した値) を、忘れる (永続化しない)
//
// 実行: go run ./ex1_paxos
package main

import (
	"container/heap"
	"fmt"
	"math/rand"
	"sort"
)

type msg struct {
	kind     int // 0 prepare、1 promise、2 accept、3 accepted
	from, to int
	b        int // 提案番号
	accN     int // promise: その時点で、受理済みの番号
	accV     int // promise: 受理済みの値
	v        int // accept: 提案する値
}

type event struct {
	t    float64
	seq  int
	kind int // 0 配送、1 プロポーザのタイマー、2 障害
	node int
	gen  int
	m    msg
	fn   func()
}
type pq []*event

func (p pq) Len() int { return len(p) }
func (p pq) Less(i, j int) bool {
	if p[i].t != p[j].t {
		return p[i].t < p[j].t
	}
	return p[i].seq < p[j].seq
}
func (p pq) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p *pq) Push(x any)   { *p = append(*p, x.(*event)) }
func (p *pq) Pop() any     { o := *p; n := len(o); x := o[n-1]; *p = o[:n-1]; return x }

const nAcc, nProp = 5, 3

type acceptor struct {
	alive          bool
	promised, accN int
	accV           int
}

type proposer struct {
	id       int
	myVal    int
	round    int
	ballot   int
	phase    int // 1: prepare 中、2: accept 中、3: 完了
	promises map[int][2]int
	accepts  map[int]bool
	val      int
	gen      int
}

type cfgT struct {
	backoff       bool
	ignoreAccept  bool
	amnesia       bool
	baseTimeoutMs float64
}

type sim struct {
	c        cfgT
	rng      *rand.Rand
	now      float64
	q        pq
	seq      int
	acc      [nAcc]*acceptor
	prop     [nProp]*proposer
	tally    map[[2]int]map[int]bool // (ballot, 値) → 受理したアクセプタ
	chosen   map[int]int             // 選ばれた (ballot → 値)
	firstAt  float64
	messages int
}

func (s *sim) schedule(e *event) { s.seq++; e.seq = s.seq; heap.Push(&s.q, e) }

func (s *sim) send(m msg) {
	s.messages++
	if s.rng.Float64() < 0.10 {
		return
	}
	copies := 1
	if s.rng.Float64() < 0.05 {
		copies = 2
	}
	for i := 0; i < copies; i++ {
		s.schedule(&event{t: s.now + 1 + s.rng.Float64()*9, kind: 0, node: m.to, m: m})
	}
}

// 提案番号: 全プロポーザで、一意 (round × 10 + id)
func (s *sim) startRound(p *proposer) {
	p.round++
	p.ballot = p.round*10 + p.id
	p.phase = 1
	p.promises = map[int][2]int{}
	p.accepts = map[int]bool{}
	for a := 0; a < nAcc; a++ {
		s.send(msg{kind: 0, from: p.id, to: a, b: p.ballot})
	}
	p.gen++
	to := s.c.baseTimeoutMs
	if s.c.backoff {
		to += s.rng.Float64() * 200
	}
	s.schedule(&event{t: s.now + to, kind: 1, node: p.id, gen: p.gen})
}

func (s *sim) handleAcceptor(a *acceptor, aid int, m msg) {
	switch m.kind {
	case 0:
		if m.b > a.promised {
			a.promised = m.b
			s.send(msg{kind: 1, from: aid, to: m.from, b: m.b, accN: a.accN, accV: a.accV})
		}
	case 2:
		if m.b >= a.promised {
			a.promised, a.accN, a.accV = m.b, m.b, m.v
			k := [2]int{m.b, m.v}
			if s.tally[k] == nil {
				s.tally[k] = map[int]bool{}
			}
			s.tally[k][aid] = true
			if len(s.tally[k]) >= nAcc/2+1 {
				if _, ok := s.chosen[m.b]; !ok {
					s.chosen[m.b] = m.v
					if len(s.chosen) == 1 {
						s.firstAt = s.now
					}
				}
			}
			s.send(msg{kind: 3, from: aid, to: m.from, b: m.b})
		}
	}
}

func (s *sim) handleProposer(p *proposer, m msg) {
	if m.b != p.ballot || p.phase == 3 {
		return
	}
	switch m.kind {
	case 1:
		if p.phase != 1 {
			return
		}
		p.promises[m.from] = [2]int{m.accN, m.accV}
		if len(p.promises) >= nAcc/2+1 {
			// 規則: 返ってきた受理済みの値のうち、最大の番号の値を、使う。なければ、自分の値
			bestN, v := 0, p.myVal
			if !s.c.ignoreAccept {
				for _, pr := range p.promises {
					if pr[0] > bestN {
						bestN, v = pr[0], pr[1]
					}
				}
			}
			p.val, p.phase = v, 2
			for a := 0; a < nAcc; a++ {
				s.send(msg{kind: 2, from: p.id, to: a, b: p.ballot, v: v})
			}
		}
	case 3:
		if p.phase != 2 {
			return
		}
		p.accepts[m.from] = true
		if len(p.accepts) >= nAcc/2+1 {
			p.phase = 3 // このプロポーザは、決定を知った
		}
	}
}

func run(c cfgT, seed int64, horizon float64) (violation bool, decided bool, at float64, msgs int) {
	s := &sim{c: c, rng: rand.New(rand.NewSource(seed)), tally: map[[2]int]map[int]bool{}, chosen: map[int]int{}}
	for i := range s.acc {
		s.acc[i] = &acceptor{alive: true}
	}
	for i := range s.prop {
		s.prop[i] = &proposer{id: i, myVal: 100 + i}
		p := s.prop[i]
		s.schedule(&event{t: float64(i) * 3, kind: 2, fn: func() { s.startRound(p) }}) // ほぼ同時に始める
	}
	// アクセプタのクラッシュ: 100〜400 ms ごとに、1 つが、50〜300 ms 止まる
	for t := 20.0; t < horizon; t += 100 + s.rng.Float64()*300 {
		v := s.rng.Intn(nAcc)
		dur := 50 + s.rng.Float64()*250
		s.schedule(&event{t: t, kind: 2, fn: func() {
			if s.acc[v].alive {
				s.acc[v].alive = false
				s.schedule(&event{t: s.now + dur, kind: 2, fn: func() {
					s.acc[v].alive = true
					if s.c.amnesia {
						*s.acc[v] = acceptor{alive: true}
					}
				}})
			}
		}})
	}
	for len(s.q) > 0 && s.q[0].t <= horizon {
		e := heap.Pop(&s.q).(*event)
		s.now = e.t
		switch e.kind {
		case 0:
			if e.m.kind == 0 || e.m.kind == 2 {
				if a := s.acc[e.node]; a.alive {
					s.handleAcceptor(a, e.node, e.m)
				}
			} else {
				s.handleProposer(s.prop[e.node], e.m)
			}
		case 1:
			p := s.prop[e.node]
			if e.gen == p.gen && p.phase != 3 {
				s.startRound(p)
			}
		case 2:
			e.fn()
		}
	}
	vals := map[int]bool{}
	for _, v := range s.chosen {
		vals[v] = true
	}
	return len(vals) > 1, len(s.chosen) > 0, s.firstAt, s.messages
}

func main() {
	const runs = 2000
	const horizon = 3000.0
	fmt.Printf("単一値の Paxos (アクセプタ %d、プロポーザ %d、喪失 10%%、重複 5%%、アクセプタのクラッシュ、各 %d 回の実行、%.0f ms まで)\n\n", nAcc, nProp, runs, horizon)
	fmt.Printf("  %-52s %-18s %-14s %-12s %s\n", "版", "安全性の違反", "決まった実行", "決まるまで p50 / p99 (ms)", "メッセージ数 (平均)")
	for _, v := range []struct {
		name string
		c    cfgT
	}{
		{"1. 正しい Paxos + ランダムなバックオフ", cfgT{backoff: true, baseTimeoutMs: 60}},
		{"2. 正しい Paxos + バックオフなし (固定タイムアウト)", cfgT{backoff: false, baseTimeoutMs: 60}},
		{"3. 壊した版 (a): 受理済みの値を無視して、自分の値を提案", cfgT{backoff: true, baseTimeoutMs: 60, ignoreAccept: true}},
		{"4. 壊した版 (b): アクセプタが、再起動で状態を忘れる", cfgT{backoff: true, baseTimeoutMs: 60, amnesia: true}},
	} {
		viol, dec := 0, 0
		var ts []float64
		msgs := 0
		for seed := int64(0); seed < runs; seed++ {
			vi, d, at, m := run(v.c, seed, horizon)
			if vi {
				viol++
			}
			if d {
				dec++
				ts = append(ts, at)
			}
			msgs += m
		}
		sort.Float64s(ts)
		p50, p99 := 0.0, 0.0
		if len(ts) > 0 {
			p50, p99 = ts[len(ts)/2], ts[len(ts)*99/100]
		}
		fmt.Printf("  %-52s %-18s %-14s %6.0f / %-8.0f %d\n", v.name, fmt.Sprintf("%d / %d", viol, runs), fmt.Sprintf("%d / %d", dec, runs), p50, p99, msgs/runs)
	}
	fmt.Println("\n  → 正しい版は、どんなタイミングでも、選ばれた値は 1 つだけ (安全性)。決まるかどうか (活性) は、タイミングと再試行の仕方に、依存する。")
	fmt.Println("    規則を 1 つ外すと (受理済みの値の引き継ぎ、状態の永続化)、安全性が破れる。")
}
