// ex6: Go のスケジューラ
//
//	go run . pingpong   goroutine の切り替えと、OS スレッドの切り替えを比べる
//	go run . preempt    CPU を占有し続ける goroutine があっても、他の goroutine が動けるか
//
// preempt は、非同期プリエンプション (Go 1.14 以降) の効果を見る。
//
//	GODEBUG=asyncpreemptoff=1 go run . preempt   ← 無効にすると、止まったままになる (外から kill が必要)
package main

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// 2つの goroutine が、チャネルで交互にやりとりする。1往復あたりの時間を返す。
func pingpong(locked bool, rounds int) float64 {
	a, b := make(chan struct{}), make(chan struct{})
	done := make(chan struct{})
	go func() {
		if locked {
			runtime.LockOSThread() // この goroutine を専用の OS スレッドに固定する
		}
		for i := 0; i < rounds; i++ {
			<-a
			b <- struct{}{}
		}
		close(done)
	}()
	if locked {
		runtime.LockOSThread()
	}
	start := time.Now()
	for i := 0; i < rounds; i++ {
		a <- struct{}{}
		<-b
	}
	<-done
	return float64(time.Since(start).Nanoseconds()) / float64(rounds)
}

func runPingpong() {
	fmt.Printf("論理CPU = %d、GOMAXPROCS(既定) = %d\n\n", runtime.NumCPU(), runtime.GOMAXPROCS(0))
	const rounds = 300000
	fmt.Printf("%-44s %12s %14s\n", "設定", "1往復(ns)", "1切り替え(ns)")

	prev := runtime.GOMAXPROCS(1)
	r := pingpong(false, rounds)
	fmt.Printf("%-44s %12.0f %14.0f\n", "goroutine / GOMAXPROCS=1", r, r/2)
	runtime.GOMAXPROCS(prev)

	r = pingpong(false, rounds)
	fmt.Printf("%-44s %12.0f %14.0f\n", "goroutine / GOMAXPROCS=既定", r, r/2)

	r = pingpong(true, rounds/10)
	fmt.Printf("%-44s %12.0f %14.0f\n", "各 goroutine を専用の OS スレッドに固定", r, r/2)
}

func runPreempt() {
	runtime.GOMAXPROCS(1) // 実行できるのは1つだけ
	go func() {
		for { // 関数呼び出しのない、CPU を占有し続けるループ
		}
	}()
	start := time.Now()
	time.Sleep(100 * time.Millisecond) // この間に、占有ループが唯一の P を握る
	fmt.Printf("main が再開した。100ms の sleep に実際にかかった時間 = %v\n", time.Since(start).Round(time.Millisecond))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("使い方: go run . pingpong | preempt")
		return
	}
	switch os.Args[1] {
	case "pingpong":
		runPingpong()
	case "preempt":
		runPreempt()
	}
}
