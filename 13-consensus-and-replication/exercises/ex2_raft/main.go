// ex2: Raft (リーダー選出 + ログの複製) の、離散イベント・シミュレーション
//
// 5 ノード。ネットワークは、遅延 (2〜8 ms)、喪失 (5%)、重複 (2%)、順序の入れ替わり、分断を起こす。ノードは、クラッシュと再起動をする
// (永続状態 = term、votedFor、ログ は残り、それ以外は消える)。
// 実装: Raft 論文の Figure 2 の、主要な規則 (RequestVote、AppendEntries、現在の term のエントリだけを、多数派の複製で commit)。
//
//	クラスタのメンバーの変更、スナップショット、ログの圧縮は、実装していない。
//
// 検査する安全性の性質 (実行中、常時):
//  1. 選出の安全性: 1 つの term に、リーダーは、高々 1 人
//  2. 状態機械の安全性: 同じ index に commit された (適用された) エントリは、全ノードで同じ
//  3. リーダーの完全性: リーダーになったノードの、ログに、commit 済みの全エントリが含まれる
//  4. ログの一致 (終了時): 2 つのログで、同じ index・同じ term のエントリがあれば、それ以前の全てが同じ
//
// 実験:
//
//	A. リーダーが落ちてから、新しいリーダーが決まるまでの時間 (選挙タイムアウトの範囲を変えて)
//	B. 乱数で、障害 (クラッシュ、分断、喪失) を注入した、多数の実行で、安全性の違反が 0 であることの確認
//	C. わざと壊した版 (規則を 1 つ外す) で、違反が、検出できるか (検査器が、機能しているかの確認)
//
// 実行: go run ./ex2_raft
package main

import (
	"container/heap"
	"fmt"
	"math/rand"
	"sort"
)

const (
	follower = iota
	candidate
	leader
)

type entry struct{ term, cmd int }

type msg struct {
	kind              int // 0 RequestVote、1 VoteReply、2 AppendEntries、3 AEReply
	from, to, term    int
	lastIdx, lastTerm int  // RequestVote
	granted           bool // VoteReply
	prevIdx, prevTerm int  // AppendEntries
	entries           []entry
	leaderCommit      int
	success           bool // AEReply
	matchIdx          int
}

type event struct {
	t       float64
	seq     int
	kind    int // 0 メッセージ配送、1 選挙タイマー、2 ハートビート、3 クライアント、4 障害
	node    int
	gen     int
	m       msg
	special func()
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

type node struct {
	id       int
	alive    bool
	term     int
	votedFor int
	log      []entry
	state    int
	commit   int
	applied  int
	votes    int
	voted    map[int]bool
	nextIdx  []int
	matchIdx []int
	timerGen int
	hbGen    int
}

type cfg struct {
	n                  int
	electMin, electMax float64
	loss, dup          float64
	unsafeCommit       bool // 過去の term のエントリも、複製の数だけで commit (Raft の Figure 8 の誤り)
	unsafeVote         bool // 投票のとき、ログの新しさを確認しない
}

type sim struct {
	c          cfg
	rng        *rand.Rand
	now        float64
	q          pq
	seq        int
	nodes      []*node
	group      []int
	leaders    map[int]int
	committed  map[int]entry
	violations map[string]int
	cmdCounter int
	elections  int
	becameLdr  int
	firstLdr   float64
	ldrTimes   []float64 // リーダーが決まった時刻
}

func newSim(c cfg, seed int64) *sim {
	s := &sim{c: c, rng: rand.New(rand.NewSource(seed)), leaders: map[int]int{}, committed: map[int]entry{}, violations: map[string]int{}, firstLdr: -1}
	s.group = make([]int, c.n)
	for i := 0; i < c.n; i++ {
		s.nodes = append(s.nodes, &node{id: i, alive: true, votedFor: -1, log: []entry{{0, 0}}})
	}
	for _, n := range s.nodes {
		s.resetTimer(n)
	}
	return s
}

func (s *sim) schedule(e *event) { s.seq++; e.seq = s.seq; heap.Push(&s.q, e) }

func (s *sim) resetTimer(n *node) {
	n.timerGen++
	s.schedule(&event{t: s.now + s.c.electMin + s.rng.Float64()*(s.c.electMax-s.c.electMin), kind: 1, node: n.id, gen: n.timerGen})
}

func (s *sim) send(m msg) {
	if s.group[m.from] != s.group[m.to] || s.rng.Float64() < s.c.loss {
		return
	}
	copies := 1
	if s.rng.Float64() < s.c.dup {
		copies = 2
	}
	for i := 0; i < copies; i++ {
		s.schedule(&event{t: s.now + 2 + s.rng.Float64()*6, kind: 0, node: m.to, m: m})
	}
}

func (n *node) lastIdx() int { return len(n.log) - 1 }

func (s *sim) majority() int { return s.c.n/2 + 1 }

func (s *sim) stepDown(n *node, term int) {
	n.term, n.votedFor, n.state = term, -1, follower
}

func (s *sim) startElection(n *node) {
	n.term++
	n.state, n.votedFor, n.votes = candidate, n.id, 1
	n.voted = map[int]bool{n.id: true}
	s.elections++
	s.resetTimer(n)
	for p := 0; p < s.c.n; p++ {
		if p != n.id {
			s.send(msg{kind: 0, from: n.id, to: p, term: n.term, lastIdx: n.lastIdx(), lastTerm: n.log[n.lastIdx()].term})
		}
	}
}

func (s *sim) becomeLeader(n *node) {
	n.state = leader
	n.nextIdx = make([]int, s.c.n)
	n.matchIdx = make([]int, s.c.n)
	for i := range n.nextIdx {
		n.nextIdx[i] = len(n.log)
	}
	s.becameLdr++
	s.ldrTimes = append(s.ldrTimes, s.now)
	if s.firstLdr < 0 {
		s.firstLdr = s.now
	}
	if o, ok := s.leaders[n.term]; ok && o != n.id {
		s.violations["選出の安全性 (同じ term に 2 人のリーダー)"]++
	}
	s.leaders[n.term] = n.id
	for idx, ce := range s.committed { // リーダーの完全性
		if idx >= len(n.log) || n.log[idx] != ce {
			s.violations["リーダーの完全性 (commit 済みのエントリが、新リーダーのログにない)"]++
			break
		}
	}
	n.log = append(n.log, entry{n.term, -1}) // no-op: 現在の term のエントリを作り、過去のエントリを、間接的に commit できるようにする
	n.matchIdx[n.id] = n.lastIdx()
	s.broadcastAE(n)
	n.hbGen++
	s.schedule(&event{t: s.now + 50, kind: 2, node: n.id, gen: n.hbGen})
}

func (s *sim) sendAE(n *node, p int) {
	ni := n.nextIdx[p]
	if ni < 1 {
		ni = 1
	}
	var es []entry
	end := len(n.log)
	if end-ni > 20 {
		end = ni + 20
	}
	if ni < len(n.log) {
		es = append(es, n.log[ni:end]...)
	}
	s.send(msg{kind: 2, from: n.id, to: p, term: n.term, prevIdx: ni - 1, prevTerm: n.log[ni-1].term, entries: es, leaderCommit: n.commit})
}

func (s *sim) broadcastAE(n *node) {
	for p := 0; p < s.c.n; p++ {
		if p != n.id {
			s.sendAE(n, p)
		}
	}
}

func (s *sim) advanceCommit(n *node) {
	for N := n.lastIdx(); N > n.commit; N-- {
		if !s.c.unsafeCommit && n.log[N].term != n.term {
			break
		}
		cnt := 0
		for p := 0; p < s.c.n; p++ {
			if n.matchIdx[p] >= N {
				cnt++
			}
		}
		if cnt >= s.majority() {
			n.commit = N
			break
		}
	}
	s.apply(n)
}

func (s *sim) apply(n *node) {
	for n.applied < n.commit && n.applied+1 < len(n.log) {
		n.applied++
		e := n.log[n.applied]
		if ce, ok := s.committed[n.applied]; ok {
			if ce != e {
				s.violations["状態機械の安全性 (同じ index に、異なるエントリが commit)"]++
			}
		} else {
			s.committed[n.applied] = e
		}
	}
}

func (s *sim) handle(n *node, m msg) {
	if m.term > n.term {
		s.stepDown(n, m.term)
	}
	switch m.kind {
	case 0: // RequestVote
		upToDate := m.lastTerm > n.log[n.lastIdx()].term || (m.lastTerm == n.log[n.lastIdx()].term && m.lastIdx >= n.lastIdx())
		if s.c.unsafeVote {
			upToDate = true
		}
		grant := m.term == n.term && (n.votedFor == -1 || n.votedFor == m.from) && upToDate
		if grant {
			n.votedFor = m.from
			s.resetTimer(n)
		}
		s.send(msg{kind: 1, from: n.id, to: m.from, term: n.term, granted: grant})
	case 1: // VoteReply
		if n.state == candidate && m.term == n.term && m.granted && !n.voted[m.from] { // 重複したメッセージを、二重に数えない
			n.voted[m.from] = true
			n.votes++
			if n.votes >= s.majority() {
				s.becomeLeader(n)
			}
		}
	case 2: // AppendEntries
		if m.term < n.term {
			s.send(msg{kind: 3, from: n.id, to: m.from, term: n.term})
			return
		}
		n.state = follower
		s.resetTimer(n)
		if m.prevIdx > n.lastIdx() || n.log[m.prevIdx].term != m.prevTerm {
			s.send(msg{kind: 3, from: n.id, to: m.from, term: n.term, success: false})
			return
		}
		for i, e := range m.entries {
			idx := m.prevIdx + 1 + i
			if idx < len(n.log) && n.log[idx].term != e.term {
				n.log = n.log[:idx]
			}
			if idx >= len(n.log) {
				n.log = append(n.log, e)
			}
		}
		last := m.prevIdx + len(m.entries)
		if m.leaderCommit > n.commit {
			c := m.leaderCommit
			if last < c {
				c = last
			}
			if c > n.commit {
				n.commit = c
			}
		}
		s.apply(n)
		s.send(msg{kind: 3, from: n.id, to: m.from, term: n.term, success: true, matchIdx: last})
	case 3: // AEReply
		if n.state != leader || m.term != n.term {
			return
		}
		if m.success {
			if m.matchIdx > n.matchIdx[m.from] {
				n.matchIdx[m.from] = m.matchIdx
			}
			n.nextIdx[m.from] = n.matchIdx[m.from] + 1
			s.advanceCommit(n)
		} else {
			if n.nextIdx[m.from] > 1 {
				n.nextIdx[m.from]--
			}
			s.sendAE(n, m.from)
		}
	}
}

func (s *sim) crash(n *node) { n.alive = false; n.state = follower }

func (s *sim) restart(n *node) {
	n.alive, n.state, n.commit, n.applied = true, follower, 0, 0
	s.resetTimer(n)
}

func (s *sim) run(until float64) {
	for len(s.q) > 0 && s.q[0].t <= until {
		e := heap.Pop(&s.q).(*event)
		s.now = e.t
		switch e.kind {
		case 0:
			if n := s.nodes[e.node]; n.alive {
				s.handle(n, e.m)
			}
		case 1:
			n := s.nodes[e.node]
			if n.alive && e.gen == n.timerGen && n.state != leader {
				s.startElection(n)
			}
		case 2:
			n := s.nodes[e.node]
			if n.alive && n.state == leader && e.gen == n.hbGen {
				s.broadcastAE(n)
				s.schedule(&event{t: s.now + 50, kind: 2, node: n.id, gen: n.hbGen})
			}
		case 3, 4:
			e.special()
		}
	}
	s.now = until
}

func (s *sim) logMatching() {
	for a := 0; a < s.c.n; a++ {
		for b := a + 1; b < s.c.n; b++ {
			la, lb := s.nodes[a].log, s.nodes[b].log
			for i := len(la) - 1; i >= 1; i-- {
				if i < len(lb) && la[i] == lb[i] {
					for j := 1; j <= i; j++ {
						if la[j] != lb[j] {
							s.violations["ログの一致 (同じ index・term なのに、それ以前が違う)"]++
							break
						}
					}
					break
				}
			}
		}
	}
}

// 障害と、クライアントの要求を、注入する
func (s *sim) injectChaos(horizon float64) {
	// クライアント: 20 ms ごとに、リーダーを自称するノードに、コマンドを送る
	for t := 100.0; t < horizon; t += 20 {
		s.schedule(&event{t: t, kind: 3, special: func() {
			for _, n := range s.nodes {
				if n.alive && n.state == leader {
					s.cmdCounter++
					n.log = append(n.log, entry{n.term, s.cmdCounter})
					n.matchIdx[n.id] = n.lastIdx()
					s.broadcastAE(n)
					return
				}
			}
		}})
	}
	// 障害: 150〜600 ms ごとに、クラッシュ (100〜800 ms 後に再起動)、または分断 (200〜1000 ms 後に回復)
	for t := 300.0; t < horizon; t += 150 + s.rng.Float64()*450 {
		if s.rng.Float64() < 0.5 {
			v := s.rng.Intn(s.c.n)
			dur := 100 + s.rng.Float64()*700
			s.schedule(&event{t: t, kind: 4, special: func() {
				if s.nodes[v].alive {
					s.crash(s.nodes[v])
					s.schedule(&event{t: s.now + dur, kind: 4, special: func() { s.restart(s.nodes[v]) }})
				}
			}})
		} else {
			dur := 200 + s.rng.Float64()*800
			k := 1 + s.rng.Intn(2)
			perm := s.rng.Perm(s.c.n)
			s.schedule(&event{t: t, kind: 4, special: func() {
				for i := range s.group {
					s.group[i] = 0
				}
				for _, v := range perm[:k] {
					s.group[v] = 1
				}
				s.schedule(&event{t: s.now + dur, kind: 4, special: func() {
					for i := range s.group {
						s.group[i] = 0
					}
				}})
			}})
		}
	}
}

func partA() {
	fmt.Println("A. リーダーが落ちてから、新しいリーダーが決まるまでの時間 (5 ノード、障害はリーダーのクラッシュ 1 回だけ、各 400 回の実行)")
	fmt.Printf("  %-26s %9s %9s %9s %9s %s\n", "選挙タイムアウトの範囲", "p50(ms)", "p99(ms)", "最大(ms)", "平均の選挙の回数", "(分裂した投票: 選挙の回数が 1 より多いほど多い)")
	for _, r := range [][2]float64{{150, 300}, {150, 160}, {150, 151}, {1500, 3000}} {
		var ts []float64
		var elections []float64
		for seed := int64(0); seed < 400; seed++ {
			s := newSim(cfg{n: 5, electMin: r[0], electMax: r[1], loss: 0.02, dup: 0.01}, seed)
			s.run(r[1]*3 + 500) // 最初のリーダーが決まるまで、十分待つ
			var l *node
			for _, n := range s.nodes {
				if n.alive && n.state == leader {
					l = n
				}
			}
			if l == nil {
				continue
			}
			s.crash(l)
			t0 := s.now
			e0 := s.elections
			cnt := s.becameLdr
			for s.becameLdr == cnt && s.now < t0+60000 {
				s.run(s.now + 5)
			}
			if s.becameLdr > cnt {
				ts = append(ts, s.now-t0)
				elections = append(elections, float64(s.elections-e0))
			}
		}
		if len(ts) == 0 {
			fmt.Printf("  [%5.0f, %5.0f) ms          60 秒以内に、リーダーが決まった実行: 0 / 400 (選挙が、ほぼ同時に始まり続け、票が割れ続ける)\n", r[0], r[1])
			continue
		}
		sort.Float64s(ts)
		sum := 0.0
		for _, e := range elections {
			sum += e
		}
		fmt.Printf("  [%5.0f, %5.0f) ms          %9.0f %9.0f %9.0f %9.2f   (決まった実行 %d / 400)\n", r[0], r[1], ts[len(ts)/2], ts[len(ts)*99/100], ts[len(ts)-1], sum/float64(len(elections)), len(ts))
	}
}

func chaos(c cfg, runs int) (map[string]int, float64, float64, int) {
	total := map[string]int{}
	var committedSum, termMax float64
	broken := 0
	for seed := int64(0); seed < int64(runs); seed++ {
		s := newSim(c, seed)
		s.injectChaos(20000)
		s.run(20000)
		s.logMatching()
		for k, v := range s.violations {
			total[k] += v
		}
		if len(s.violations) > 0 {
			broken++
		}
		committedSum += float64(len(s.committed))
		mt := 0
		for t := range s.leaders {
			if t > mt {
				mt = t
			}
		}
		termMax += float64(mt)
	}
	return total, committedSum / float64(runs), termMax / float64(runs), broken
}

func main() {
	partA()
	const runs = 300
	fmt.Printf("\nB・C. 障害を注入した実行 (各 20 秒の模擬時間 × %d 回。クラッシュ、分断、メッセージの喪失 5%%、重複 2%%)\n", runs)
	base := cfg{n: 5, electMin: 150, electMax: 300, loss: 0.05, dup: 0.02}
	for _, v := range []struct {
		name string
		c    cfg
	}{
		{"B. 正しい Raft", base},
		{"C1. 壊した版: 過去の term のエントリも、複製の数だけで commit", func() cfg { c := base; c.unsafeCommit = true; return c }()},
		{"C2. 壊した版: 投票のとき、ログの新しさを確認しない", func() cfg { c := base; c.unsafeVote = true; return c }()},
	} {
		viol, avgCommitted, avgTerm, broken := chaos(v.c, runs)
		fmt.Printf("\n  %s\n", v.name)
		fmt.Printf("    commit されたエントリ (平均、1 実行あたり) = %.0f、到達した term (平均) = %.1f\n", avgCommitted, avgTerm)
		fmt.Printf("    違反が検出された実行: %d / %d\n", broken, runs)
		keys := make([]string, 0)
		for k := range viol {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			fmt.Println("    違反: なし")
		}
		for _, k := range keys {
			fmt.Printf("    違反: %s: %d 件\n", k, viol[k])
		}
	}
}
