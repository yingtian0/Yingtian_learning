// ex6: データ競合と、競合状態 (race condition)。Go の race detector で何が見つかり、何が見つからないか。
//
//	go run . counter           保護なしの counter++ (データ競合)
//	go run . mapwrite          複数の goroutine が map に書く (データ競合。ランタイムが検出して落ちることがある)
//	go run . bank              残高を調べてから引き出す。個々の操作はロックで守られているが、全体が1つの操作になっていない
//	go run . bank-fixed        確認と引き出しを1つのロックの中でやる
//
// それぞれ、`go run -race . <mode>` でも実行して、違いを見る。
package main

import (
	"fmt"
	"os"
	"sync"
)

func counter() {
	var n int
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000000; i++ {
				n++
			}
		}()
	}
	wg.Wait()
	fmt.Printf("counter: 結果 %d (正しい値 8000000)\n", n)
}

func mapwrite() {
	m := map[int]int{}
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100000; i++ {
				m[g*100000+i] = i
			}
		}(g)
	}
	wg.Wait()
	fmt.Println("mapwrite: 完了 (要素数", len(m), ")")
}

type Account struct {
	mu      sync.Mutex
	balance int
}

func (a *Account) Balance() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.balance
}
func (a *Account) Withdraw(n int) {
	a.mu.Lock()
	a.balance -= n
	a.mu.Unlock()
}

// 確認と引き出しを、1つのロックの中でやる
func (a *Account) TryWithdraw(n int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.balance < n {
		return false
	}
	a.balance -= n
	return true
}

func bank(fixed bool) {
	const accounts = 20000
	accs := make([]*Account, accounts)
	for i := range accs {
		accs[i] = &Account{balance: 100}
	}
	var wg sync.WaitGroup
	for g := 0; g < 2; g++ { // 2人が、同時に全口座から 100 を引き出そうとする
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, a := range accs {
				if fixed {
					a.TryWithdraw(100)
				} else if a.Balance() >= 100 { // 確認 (ロックして、放す)
					a.Withdraw(100) // 引き出し (もう一度ロックする) ← この隙間に、もう1人が割り込める
				}
			}
		}()
	}
	wg.Wait()
	neg := 0
	for _, a := range accs {
		if a.balance < 0 {
			neg++
		}
	}
	fmt.Printf("bank(fixed=%v): 残高がマイナスになった口座 = %d / %d\n", fixed, neg, accounts)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("使い方: go run . counter | mapwrite | bank | bank-fixed")
		return
	}
	switch os.Args[1] {
	case "counter":
		counter()
	case "mapwrite":
		mapwrite()
	case "bank":
		bank(false)
	case "bank-fixed":
		bank(true)
	}
}
