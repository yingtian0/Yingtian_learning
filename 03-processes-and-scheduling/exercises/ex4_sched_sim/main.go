// ex4: スケジューリングのシミュレータ
//
// 時間を 1 単位ずつ進め、各方式が「どのジョブを次に走らせるか」を決める。
// 方式: FCFS / SJF(非プリエンプティブ) / SRTF(プリエンプティブ) / RR(クォンタム q)
// 指標:
//
//	ターンアラウンド時間 = 完了時刻 − 到着時刻
//	応答時間             = 初めて CPU を得た時刻 − 到着時刻
//	切り替え回数         = 走るジョブが変わった回数。1回ごとに switchCost だけ時間が過ぎる。
//
// 最後に、CFS の考え方 (重みつきの仮想実行時間) を、nice 値の違いで再現する。
//
// 実行: go run .
package main

import "fmt"

type Job struct {
	Name           string
	Arrival, Burst int
}

type Result struct {
	AvgTurnaround, AvgResponse float64
	Switches                   int
	Finish                     int
}

// decide は、時刻 t に走らせるジョブの添字を返す (なければ -1)。
// ready: 到着済みで未完了のジョブ。cur: 直前まで走っていたジョブ (終了済みなら -1)。qUsed: cur が連続で走った時間。
type decide func(t int, ready []int, cur int, qUsed int, jobs []Job, rem []int) int

func simulate(jobs []Job, pick decide, switchCost int) Result {
	n := len(jobs)
	rem := make([]int, n)
	first := make([]int, n)
	done := make([]int, n)
	for i := range jobs {
		rem[i] = jobs[i].Burst
		first[i] = -1
	}
	t, finished, last, cur, qUsed, switches := 0, 0, -1, -1, 0, 0
	for finished < n {
		var ready []int
		for i := range jobs {
			if jobs[i].Arrival <= t && rem[i] > 0 {
				ready = append(ready, i)
			}
		}
		next := pick(t, ready, cur, qUsed, jobs, rem)
		if next < 0 {
			t++
			cur = -1
			continue
		}
		if last >= 0 && next != last {
			switches++
			t += switchCost // 切り替えの間は、誰も進まない
		}
		if next != cur {
			qUsed = 0
		}
		if first[next] < 0 {
			first[next] = t
		}
		t++
		rem[next]--
		qUsed++
		last, cur = next, next
		if rem[next] == 0 {
			done[next] = t
			finished++
			cur = -1
		}
	}
	var ta, rs float64
	for i := range jobs {
		ta += float64(done[i] - jobs[i].Arrival)
		rs += float64(first[i] - jobs[i].Arrival)
	}
	return Result{ta / float64(n), rs / float64(n), switches, t}
}

func fcfs(t int, ready []int, cur, qUsed int, jobs []Job, rem []int) int {
	if cur >= 0 {
		return cur
	}
	best := -1
	for _, i := range ready {
		if best < 0 || jobs[i].Arrival < jobs[best].Arrival {
			best = i
		}
	}
	return best
}

func sjf(t int, ready []int, cur, qUsed int, jobs []Job, rem []int) int {
	if cur >= 0 {
		return cur // 走り始めたら最後まで
	}
	best := -1
	for _, i := range ready {
		if best < 0 || jobs[i].Burst < jobs[best].Burst {
			best = i
		}
	}
	return best
}

func srtf(t int, ready []int, cur, qUsed int, jobs []Job, rem []int) int {
	best := cur
	for _, i := range ready {
		if best < 0 || rem[i] < rem[best] {
			best = i
		}
	}
	return best
}

func roundRobin(q int) decide {
	var queue []int
	enq := map[int]bool{}
	return func(t int, ready []int, cur, qUsed int, jobs []Job, rem []int) int {
		for _, i := range ready { // 新しく到着したジョブを待ち行列の後ろへ
			if !enq[i] {
				enq[i] = true
				queue = append(queue, i)
			}
		}
		if cur >= 0 && qUsed < q {
			return cur
		}
		if cur >= 0 { // クォンタムを使い切った → 後ろへ回す
			queue = append(queue, cur)
		}
		if len(queue) == 0 {
			return -1
		}
		next := queue[0]
		queue = queue[1:]
		return next
	}
}

func printHeader(title string) {
	fmt.Printf("\n%s\n%-14s %14s %12s %10s %8s\n", title, "方式", "平均ターンアラウンド", "平均応答", "切り替え", "終了時刻")
}

func printRow(name string, r Result) {
	fmt.Printf("%-14s %14.1f %12.1f %10d %8d\n", name, r.AvgTurnaround, r.AvgResponse, r.Switches, r.Finish)
}

// CFS の考え方: 仮想実行時間 (vruntime) が最も小さいジョブを走らせ、走った分だけ 1024/重み ずつ進める。
func cfsShares() {
	weights := map[int]float64{0: 1024, 5: 335, 10: 110, -5: 3121}
	for _, nices := range [][2]int{{0, 5}, {0, 10}, {-5, 0}} {
		w := [2]float64{weights[nices[0]], weights[nices[1]]}
		vr := [2]float64{}
		ran := [2]int{}
		const ticks = 100000
		for t := 0; t < ticks; t++ {
			i := 0
			if vr[1] < vr[0] {
				i = 1
			}
			ran[i]++
			vr[i] += 1024 / w[i]
		}
		fmt.Printf("nice %3d と nice %3d: 実測 %5.1f%% 対 %5.1f%%   重みから計算 %5.1f%% 対 %5.1f%%\n",
			nices[0], nices[1],
			float64(ran[0])/ticks*100, float64(ran[1])/ticks*100,
			w[0]/(w[0]+w[1])*100, w[1]/(w[0]+w[1])*100)
	}
}

func main() {
	convoy := []Job{{"L", 0, 100}, {"S1", 1, 5}, {"S2", 2, 5}, {"S3", 3, 5}, {"S4", 4, 5}}
	printHeader("【ワークロード1】長いジョブ L (100) が先に来て、短いジョブ (5) が4つ続く / 切り替えコスト 0")
	printRow("FCFS", simulate(convoy, fcfs, 0))
	printRow("SJF", simulate(convoy, sjf, 0))
	printRow("SRTF", simulate(convoy, srtf, 0))
	printRow("RR q=10", simulate(convoy, roundRobin(10), 0))

	equal := []Job{{"A", 0, 20}, {"B", 0, 20}, {"C", 0, 20}, {"D", 0, 20}, {"E", 0, 20}}
	printHeader("【ワークロード2】同じ長さ (20) のジョブが5つ同時に到着 / 切り替えコスト 0")
	printRow("FCFS", simulate(equal, fcfs, 0))
	printRow("RR q=1", simulate(equal, roundRobin(1), 0))
	printRow("RR q=5", simulate(equal, roundRobin(5), 0))
	printRow("RR q=20", simulate(equal, roundRobin(20), 0))

	fmt.Println("\n【ワークロード2】クォンタムを変えたときの変化 / 切り替えコスト 1")
	fmt.Printf("%-8s %14s %12s %10s %8s\n", "q", "平均ターンアラウンド", "平均応答", "切り替え", "終了時刻")
	for _, q := range []int{1, 2, 5, 10, 20} {
		r := simulate(equal, roundRobin(q), 1)
		fmt.Printf("%-8d %14.1f %12.1f %10d %8d\n", q, r.AvgTurnaround, r.AvgResponse, r.Switches, r.Finish)
	}

	fmt.Println("\n【CFS の考え方】CPU を奪い合う2つのジョブの取り分 (nice 値による重みで決まる)")
	cfsShares()
}
