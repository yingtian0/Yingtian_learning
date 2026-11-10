// ex3: デッドロックを作る、検出される/されない、直す
//
//	go run . abba      2つのロックを逆の順で取る → ランタイムが検出して終了する
//	go run . hidden    同じデッドロックだが、別の goroutine が動いているので検出されない (止まったまま)
//	go run . ordered   ロックを取る順序をそろえて直す
//	go run . trylock   TryLock と待ち時間のばらつき (バックオフ) で回避する
//
// ランタイムの検出は「すべての goroutine が眠っている」ときだけ働く。
// 一部だけが止まっている (部分的なデッドロック) ものは、見つけてくれない。
package main

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"
)

var a, b sync.Mutex

func abba() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		a.Lock()
		time.Sleep(50 * time.Millisecond) // 相手が b を取る時間を与える
		b.Lock()
		b.Unlock()
		a.Unlock()
	}()
	go func() {
		defer wg.Done()
		b.Lock()
		time.Sleep(50 * time.Millisecond)
		a.Lock()
		a.Unlock()
		b.Unlock()
	}()
	wg.Wait()
	fmt.Println("完了")
}

func ordered() {
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			a.Lock() // 両方とも「a → b」の順で取る
			time.Sleep(50 * time.Millisecond)
			b.Lock()
			b.Unlock()
			a.Unlock()
		}()
	}
	wg.Wait()
	fmt.Println("完了 (ロックの順序をそろえたので、デッドロックしない)")
}

func trylock() {
	var wg sync.WaitGroup
	var retries [2]int
	wg.Add(2)
	worker := func(id int, first, second *sync.Mutex) {
		defer wg.Done()
		for {
			first.Lock()
			time.Sleep(50 * time.Millisecond) // 相手が、自分の first を取る時間を与える (ここで両者が衝突する)
			if second.TryLock() {             // 取れなければ、持っているロックを手放してやり直す
				second.Unlock()
				first.Unlock()
				return
			}
			first.Unlock()
			retries[id]++
			time.Sleep(time.Duration(rand.Intn(5000)) * time.Microsecond) // ばらつきを入れて、同時のやり直しを避ける
		}
	}
	go worker(0, &a, &b)
	go worker(1, &b, &a)
	wg.Wait()
	fmt.Printf("完了 (やり直し回数: %d, %d)\n", retries[0], retries[1])
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("使い方: go run . abba | hidden | ordered | trylock")
		return
	}
	switch os.Args[1] {
	case "abba":
		abba()
	case "hidden":
		go func() { // 別の goroutine が、周期的に起きているだけで、ランタイムは「全員眠っている」と判断しない
			for {
				time.Sleep(100 * time.Millisecond)
			}
		}()
		abba()
	case "ordered":
		ordered()
	case "trylock":
		trylock()
	}
}
