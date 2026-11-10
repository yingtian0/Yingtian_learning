// ex4: 輻輳制御のシミュレータ (Reno / CUBIC / 簡易 BBR)
//
// ボトルネックのリンク (速度 R、バッファ B パケット、ドロップテール) を、1 本または複数のフローが共有する。
// 時間を 1 ミリ秒ずつ進め、パケット単位で、キュー、遅延、ACK を扱う。
// ループバックの実機では、損失も遅延もなく、輻輳制御の振る舞いが見えないため、シミュレーションで確かめる。
//
// 重要な注意: これは、輻輳制御の「考え方」を再現する、簡易モデルである。
//
//   - Reno : スロースタート、輻輳回避 (AIMD)、損失で cwnd を半分に。タイムアウトは扱わない。
//
//   - CUBIC: 3 次関数での成長、損失で 0.7 倍。RFC 8312 の、主要な部分のみ (高速収束などは省略)。
//
//   - BBR  : ボトルネックの帯域と、往復時間の最小値を推定し、その積 (BDP) に合わせて、ペーシングする。
//     開始、排出、帯域の探索の3つだけ (ProbeRTT なし)。実際の BBR の実装ではない。
//     実際の OS の TCP の測定ではなく、モデルの結果である。
//
//     A. Reno の、のこぎり波 (cwnd の時間変化)
//     B. バッファの大きさと、利用率・キューの遅延 (Reno、CUBIC、BBR)
//     C. ランダムな損失率と、スループット (Mathis の式との比較)
//     D. 複数のフロー: RTT が違う場合の、公平性。アルゴリズムが違う場合の、公平性。
//
// 実行: go run .
package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
)

const (
	pktBytes = 1500
	ringSize = 4096
	warmupMs = 10000
)

type Flow struct {
	cc          CC
	d1, d2      int // ボトルネックの後の伝搬遅延、ACK の戻りの遅延 (ミリ秒)
	inflight    int
	recoveryEnd int
	credit      float64
	ackRing     [ringSize][]int // その時刻に届く ACK (送信時刻のリスト)
	lossRing    [ringSize][]int // その時刻に分かる損失 (送信時刻のリスト)
	delivered   int64           // 計測期間内に届いたパケット
	cwndSamples []float64
	rttMinMs    int
	totalAck    int
}

type CC interface {
	Name() string
	OnAck(now, rtt int, f *Flow)
	OnLoss(now int, f *Flow)
	Cwnd() float64
	Pacing() float64 // パケット/ミリ秒。0 ならペーシングなし
}

// ---------------- Reno ----------------
type Reno struct{ cwnd, ssthresh float64 }

func NewReno() *Reno         { return &Reno{cwnd: 10, ssthresh: math.Inf(1)} }
func (r *Reno) Name() string { return "Reno" }
func (r *Reno) OnAck(now, rtt int, f *Flow) {
	if r.cwnd < r.ssthresh {
		r.cwnd++ // スロースタート: ACK 1 つにつき +1 (1 RTT で倍)
	} else {
		r.cwnd += 1 / r.cwnd // 輻輳回避: 1 RTT で +1
	}
}
func (r *Reno) OnLoss(now int, f *Flow) {
	r.ssthresh = math.Max(r.cwnd/2, 2)
	r.cwnd = r.ssthresh
}
func (r *Reno) Cwnd() float64   { return r.cwnd }
func (r *Reno) Pacing() float64 { return 0 }

// ---------------- CUBIC (簡易) ----------------
type Cubic struct {
	cwnd, ssthresh, wMax, wEst float64
	epochStart                 int
	hasEpoch                   bool
}

const (
	cubicC    = 0.4
	cubicBeta = 0.7
)

func NewCubic() *Cubic        { return &Cubic{cwnd: 10, ssthresh: math.Inf(1)} }
func (c *Cubic) Name() string { return "CUBIC" }
func (c *Cubic) OnAck(now, rtt int, f *Flow) {
	if c.cwnd < c.ssthresh {
		c.cwnd++
		return
	}
	if !c.hasEpoch {
		c.hasEpoch = true
		c.epochStart = now
		c.wEst = c.cwnd
		if c.wMax < c.cwnd {
			c.wMax = c.cwnd
		}
	}
	k := math.Cbrt(math.Max(c.wMax-c.cwnd, 0) / cubicC)
	t := float64(now-c.epochStart)/1000 + float64(f.rttMinMs)/1000
	target := cubicC*math.Pow(t-k, 3) + c.wMax
	c.wEst += 3 * (1 - cubicBeta) / (1 + cubicBeta) / c.cwnd // Reno と同じ速さで伸びる場合 (TCP フレンドリー)
	if c.wEst > target {
		target = c.wEst
	}
	if target > c.cwnd {
		c.cwnd += (target - c.cwnd) / c.cwnd
	} else {
		c.cwnd += 0.01 / c.cwnd
	}
}
func (c *Cubic) OnLoss(now int, f *Flow) {
	c.wMax = c.cwnd
	c.cwnd = math.Max(c.cwnd*cubicBeta, 2)
	c.ssthresh = c.cwnd
	c.hasEpoch = false
}
func (c *Cubic) Cwnd() float64   { return c.cwnd }
func (c *Cubic) Pacing() float64 { return 0 }

// ---------------- BBR (簡易) ----------------
type BBR struct {
	mode                    int // 0: 開始, 1: 排出, 2: 帯域の探索
	btl                     float64
	samples                 []float64
	rtprop                  float64
	rtpropStamp             int
	delivered               int
	lastSampleT, lastSample int
	fullCnt                 int
	lastRoundBw             float64
	cycleIdx, cycleStart    int
	pacingGain, cwndGain    float64
}

var bbrGains = []float64{1.25, 0.75, 1, 1, 1, 1, 1, 1}

func NewBBR() *BBR          { return &BBR{pacingGain: 2.885, cwndGain: 2.885} }
func (b *BBR) Name() string { return "BBR(簡易)" }
func (b *BBR) OnAck(now, rtt int, f *Flow) {
	b.delivered++
	if b.rtprop == 0 || float64(rtt) <= b.rtprop || now-b.rtpropStamp > 10000 {
		b.rtprop = float64(rtt)
		b.rtpropStamp = now
	}
	interval := now - b.lastSampleT
	if interval >= int(math.Max(b.rtprop, 1)) {
		rate := float64(b.delivered-b.lastSample) / float64(interval)
		b.samples = append(b.samples, rate)
		if len(b.samples) > 10 {
			b.samples = b.samples[1:]
		}
		b.btl = 0
		for _, s := range b.samples {
			if s > b.btl {
				b.btl = s
			}
		}
		b.lastSampleT, b.lastSample = now, b.delivered
		if b.mode == 0 {
			if rate >= b.lastRoundBw*1.25 {
				b.lastRoundBw = rate
				b.fullCnt = 0
			} else {
				b.fullCnt++
				if b.fullCnt >= 3 {
					b.mode = 1
					b.pacingGain = 1 / 2.885
				}
			}
		}
	}
	if b.mode == 1 && float64(f.inflight) <= b.btl*b.rtprop {
		b.mode = 2
		b.cwndGain = 2
		b.cycleIdx, b.cycleStart = 2, now
		b.pacingGain = 1
	}
	if b.mode == 2 && float64(now-b.cycleStart) >= b.rtprop {
		b.cycleIdx = (b.cycleIdx + 1) % len(bbrGains)
		b.cycleStart = now
		b.pacingGain = bbrGains[b.cycleIdx]
	}
}
func (b *BBR) OnLoss(now int, f *Flow) {} // 損失では、窓を縮めない
func (b *BBR) Cwnd() float64 {
	bdp := b.btl * b.rtprop
	if bdp == 0 {
		return 10
	}
	return math.Max(b.cwndGain*bdp, 4)
}
func (b *BBR) Pacing() float64 {
	if b.btl == 0 {
		return 1
	}
	return b.pacingGain * b.btl
}

// ---------------- シミュレータ ----------------
type FlowCfg struct {
	cc    string
	rttMs int
}

type Config struct {
	rateMbps   float64
	bufPkts    int
	flows      []FlowCfg
	durationS  int
	lossP      float64
	sampleCwnd bool
}

type Result struct {
	thrMbps  []float64
	util     float64
	qDelayMs float64
	cwnd     [][]float64
	rateMs   float64
}

func newCC(name string) CC {
	switch name {
	case "Reno":
		return NewReno()
	case "CUBIC":
		return NewCubic()
	default:
		return NewBBR()
	}
}

type Packet struct{ flow, sent int }

func simulate(cfg Config, seed int64) Result {
	rng := rand.New(rand.NewSource(seed))
	rate := cfg.rateMbps * 1e6 / 8 / pktBytes / 1000 // パケット/ミリ秒
	flows := make([]*Flow, len(cfg.flows))
	for i, fc := range cfg.flows {
		d2 := fc.rttMs / 2
		d1 := fc.rttMs - d2 - 1
		if d1 < 0 {
			d1 = 0
		}
		flows[i] = &Flow{cc: newCC(fc.cc), d1: d1, d2: d2, rttMinMs: fc.rttMs}
	}
	q := make([]Packet, cfg.bufPkts+1)
	head, qlen := 0, 0
	credit := 0.0
	total := cfg.durationS * 1000
	var qSum float64
	var qCnt int
	var deliveredAll int64

	for now := 0; now < total; now++ {
		slot := now % ringSize
		// 1. ACK の処理
		for _, f := range flows {
			for _, sent := range f.ackRing[slot] {
				f.inflight--
				f.cc.OnAck(now, now-sent, f)
			}
			f.ackRing[slot] = f.ackRing[slot][:0]
		}
		// 2. 損失の通知の処理 (同じ窓の中の複数の損失は、1 回の損失事象として扱う)
		for _, f := range flows {
			for _, sent := range f.lossRing[slot] {
				f.inflight--
				if sent > f.recoveryEnd {
					f.cc.OnLoss(now, f)
					f.recoveryEnd = now
				}
			}
			f.lossRing[slot] = f.lossRing[slot][:0]
		}
		// 3. リンクがパケットを送り出す
		credit += rate
		for credit >= 1 && qlen > 0 {
			credit--
			p := q[head]
			head = (head + 1) % len(q)
			qlen--
			f := flows[p.flow]
			if cfg.lossP > 0 && rng.Float64() < cfg.lossP {
				f.lossRing[(now+f.d1+f.d2)%ringSize] = append(f.lossRing[(now+f.d1+f.d2)%ringSize], p.sent)
			} else {
				f.ackRing[(now+f.d1+f.d2)%ringSize] = append(f.ackRing[(now+f.d1+f.d2)%ringSize], p.sent)
				if now >= warmupMs {
					f.delivered++
					deliveredAll++
				}
			}
		}
		if credit > rate+1 {
			credit = rate + 1 // 空いている間に、貯めすぎない
		}
		// 4. 送信側が送る (フローの順序を、時刻ごとにずらして、不公平を避ける)
		for k := 0; k < len(flows); k++ {
			fi := (now + k) % len(flows)
			f := flows[fi]
			room := int(f.cc.Cwnd()) - f.inflight
			if pr := f.cc.Pacing(); pr > 0 {
				f.credit += pr
				if f.credit > pr+2 {
					f.credit = pr + 2
				}
				if room > int(f.credit) {
					room = int(f.credit)
				}
			}
			for i := 0; i < room; i++ {
				f.inflight++
				if f.cc.Pacing() > 0 {
					f.credit--
				}
				if qlen >= cfg.bufPkts { // バッファが一杯: ドロップ
					delay := f.d1 + f.d2 + int(float64(qlen)/rate)
					f.lossRing[(now+delay)%ringSize] = append(f.lossRing[(now+delay)%ringSize], now)
				} else {
					q[(head+qlen)%len(q)] = Packet{fi, now}
					qlen++
				}
			}
		}
		if now >= warmupMs {
			qSum += float64(qlen)
			qCnt++
		}
		if cfg.sampleCwnd && now%500 == 0 {
			flows[0].cwndSamples = append(flows[0].cwndSamples, flows[0].cc.Cwnd())
		}
	}
	measured := float64(total-warmupMs) / 1000
	res := Result{rateMs: rate}
	for _, f := range flows {
		res.thrMbps = append(res.thrMbps, float64(f.delivered)*pktBytes*8/1e6/measured)
	}
	res.util = float64(deliveredAll) / (rate * float64(total-warmupMs))
	res.qDelayMs = qSum / float64(qCnt) / rate
	res.cwnd = [][]float64{flows[0].cwndSamples}
	return res
}

func bdpPkts(rateMbps float64, rttMs int) int {
	return int(rateMbps * 1e6 / 8 / pktBytes * float64(rttMs) / 1000)
}

func main() {
	const rate = 100.0
	const rtt = 50
	bdp := bdpPkts(rate, rtt)
	fmt.Printf("リンク %.0f Mbps、往復の伝搬遅延 %d ms、パケット %d バイト → BDP (帯域遅延積) = %d パケット (%.0f KB)\n\n", rate, rtt, pktBytes, bdp, float64(bdp*pktBytes)/1000)

	// ---- A ----
	fmt.Println("A. Reno の、のこぎり波 (バッファ = 1 × BDP)")
	r := simulate(Config{rateMbps: rate, bufPkts: bdp, flows: []FlowCfg{{"Reno", rtt}}, durationS: 60, sampleCwnd: true}, 1)
	fmt.Printf("  スループット %.1f Mbps (利用率 %.1f%%)、平均のキューの遅延 %.1f ms\n", r.thrMbps[0], r.util*100, r.qDelayMs)
	fmt.Println("  cwnd (パケット) の時間変化 (1 秒ごと、60 秒間):")
	for i := 0; i < len(r.cwnd[0]); i += 2 {
		v := r.cwnd[0][i]
		fmt.Printf("  %5.1fs %6.0f %s\n", float64(i)*0.5, v, strings.Repeat("#", int(v/10)))
	}
	minC, maxC := math.Inf(1), 0.0
	for i := 30; i < len(r.cwnd[0]); i++ { // 10 秒後以降
		minC = math.Min(minC, r.cwnd[0][i])
		maxC = math.Max(maxC, r.cwnd[0][i])
	}
	fmt.Printf("  10 秒以降の cwnd の範囲: %.0f 〜 %.0f パケット (BDP = %d、BDP + バッファ = %d)\n", minC, maxC, bdp, 2*bdp)

	// ---- B ----
	fmt.Println("\nB. バッファの大きさ (BDP に対する倍率) と、利用率 / 平均のキューの遅延 (単独のフロー)")
	fmt.Printf("  %-8s", "バッファ")
	for _, a := range []string{"Reno", "CUBIC", "BBR"} {
		fmt.Printf(" %20s", a+" 利用率/遅延")
	}
	fmt.Println()
	for _, mult := range []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 4} {
		buf := int(float64(bdp) * mult)
		fmt.Printf("  %5.2f BDP", mult)
		for _, a := range []string{"Reno", "CUBIC", "BBR"} {
			r := simulate(Config{rateMbps: rate, bufPkts: buf, flows: []FlowCfg{{a, rtt}}, durationS: 60}, 1)
			fmt.Printf("   %5.1f%% / %6.1f ms", r.util*100, r.qDelayMs)
		}
		fmt.Println()
	}

	// ---- C ----
	fmt.Println("\nC. ランダムな損失率と、スループット (単独のフロー、バッファ = 1 BDP)")
	fmt.Printf("  %-10s %12s %12s %12s %16s\n", "損失率", "Reno", "CUBIC", "BBR(簡易)", "Mathis の式")
	for _, p := range []float64{0, 0.0001, 0.001, 0.01, 0.05} {
		fmt.Printf("  %-10g", p)
		for _, a := range []string{"Reno", "CUBIC", "BBR"} {
			r := simulate(Config{rateMbps: rate, bufPkts: bdp, flows: []FlowCfg{{a, rtt}}, durationS: 60, lossP: p}, 1)
			fmt.Printf(" %9.1f Mbps", r.thrMbps[0])
		}
		if p > 0 {
			m := math.Min(1.22/(float64(rtt)/1000*math.Sqrt(p))*pktBytes*8/1e6, rate)
			fmt.Printf(" %12.1f Mbps", m)
		}
		fmt.Println()
	}
	fmt.Println("  (Mathis の式: スループット ≒ 1.22 × MSS / (RTT × √損失率)。リンク速度 100 Mbps が上限)")

	// ---- D ----
	fmt.Println("\nD. 複数のフローが、同じボトルネックを共有する (100 Mbps。バッファは、特に書かない限り 1 BDP = RTT 50 ms 換算)")
	cases := []struct {
		name  string
		flows []FlowCfg
		buf   float64
	}{
		{"Reno (RTT 20 ms) と Reno (RTT 100 ms)", []FlowCfg{{"Reno", 20}, {"Reno", 100}}, 1},
		{"Reno (RTT 50 ms) と Reno (RTT 50 ms)", []FlowCfg{{"Reno", 50}, {"Reno", 50}}, 1},
		{"Reno (RTT 50 ms) と CUBIC (RTT 50 ms)", []FlowCfg{{"Reno", 50}, {"CUBIC", 50}}, 1},
		{"Reno (RTT 50 ms) と BBR(簡易) (RTT 50 ms)", []FlowCfg{{"Reno", 50}, {"BBR", 50}}, 1},
		{"Reno (RTT 50 ms) と BBR(簡易)、バッファ 4 BDP", []FlowCfg{{"Reno", 50}, {"BBR", 50}}, 4},
		{"BBR(簡易) (RTT 50 ms) と BBR(簡易) (RTT 50 ms)", []FlowCfg{{"BBR", 50}, {"BBR", 50}}, 1},
	}
	for _, c := range cases {
		r := simulate(Config{rateMbps: rate, bufPkts: int(float64(bdp) * c.buf), flows: c.flows, durationS: 120}, 1)
		fmt.Printf("  %-48s → %5.1f Mbps と %5.1f Mbps (比 %.2f 対 1)、利用率 %.1f%%\n", c.name, r.thrMbps[0], r.thrMbps[1],
			math.Max(r.thrMbps[0], r.thrMbps[1])/math.Min(r.thrMbps[0], r.thrMbps[1]), r.util*100)
	}
}
