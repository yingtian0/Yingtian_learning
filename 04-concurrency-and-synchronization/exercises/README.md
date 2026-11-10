# 第4回 演習

ex1、ex2 は C(pthread)。ex3〜ex6 は Go。macOS(Apple Silicon)で動作を確認した。Linux でも動くように書いてあるが、**Linux での実行は未確認**。

```bash
cd 04-concurrency-and-synchronization/exercises
```

## 進め方

第1〜3回と同じ。実行前に予想を書き、実行して比べ、外れたら理由を調べる。各計測は3回以上行う。

注意: ex2、ex5 は、CPU を使い切る。他の作業を止めてから実行する。ex3 の `hidden` モードは、止まったままになるので、外から止める(下に手順)。

---

## 演習1: データ競合と更新の喪失

[ex1_data_race.c](ex1_data_race.c)

```bash
cc -O2 -pthread -o ex1 ex1_data_race.c && ./ex1 && ./ex1
```

**課題**

1. 保護なしの版の結果は、実行のたびに変わるか。失われる割合は、おおよそどれくらいか。
2. スレッド数 `NT` を 1、2、4、8、16 と変える。保護なしの版で、1 スレッドのときは失われないことを確認する。2 スレッドだと、どうか。
3. **改造**: 保護なしの版の `volatile` を外して、`-O2` でビルドする。結果と時間は、どうなるか。コンパイラが何をしたか、逆アセンブルで確認する(`otool -tV` または `objdump -d`)。
4. **改造**: アトミック加算の `memory_order_relaxed` を `memory_order_seq_cst` に変える。時間は変わるか(ARM と x86 で、違うかもしれない)。
5. **改造**: `counter++` の中に、`sched_yield()` や `usleep(0)` を入れて、競合の起きやすさを変える。
6. **改造**: スレッドごとのカウンタの `pad` を外す(false sharing、第1回)。時間は、どう変わるか。
7. 発展: `-fsanitize=thread` を付けてビルドし、ThreadSanitizer が、保護なしの版のデータ競合を報告することを確認する。

## 演習2: ロックの実装を比べる

[ex2_locks.c](ex2_locks.c)

```bash
cc -O2 -pthread -o ex2 ex2_locks.c && ./ex2
```

**課題**

1. 表(スレッド数 × ロックの種類)を作り、第4回の本文 §4.7 の結果と比べる。
2. **チケットロックが、論理 CPU より多いスレッドで崩壊する**ことを確認する。崩壊しているときの公平さは、どうなっているか。
3. 保持時間の影響: ロックの中に、`for` ループで、少し重い計算(100 回、1000 回の足し算)を入れる。スピンロックと mutex の優劣は、どう変わるか。
4. **改造**: MCS ロックを実装して、比べる。各スレッドが、自分のノードのフラグだけを見て待つようにする。
5. **改造**: スピンロックに、**指数バックオフ**(失敗するたびに、待つ時間を倍にする)を入れる。TAS の競合は、どう変わるか。
6. **改造**: スピンの中で `sched_yield()` を呼ぶ。論理 CPU より多いスレッドでの崩壊は、どう改善するか。
7. 発展: Linux で、`taskset -c 0` で 1 コアに固定して実行する。スピンロックが、1 コアでは、なぜ極端に遅くなるか。
8. 発展: macOS の `os_unfair_lock`、Linux の `pthread_spinlock_t` も加えて比べる。

## 演習3: デッドロック

[ex3_deadlock/](ex3_deadlock/)

```bash
cd ex3_deadlock
go run . abba      # ランタイムが検出して終了する
go run . ordered   # ロックの順序をそろえて直す
go run . trylock   # TryLock + バックオフ
# hidden は止まったままになるので、外から止める
go build -o /tmp/ex3 . && (/tmp/ex3 hidden & pid=$!; sleep 3; kill $pid)
```

**課題**

1. `abba` と `hidden` の出力を比べる。同じデッドロックなのに、なぜ違うのか。
2. `hidden` で止まっている最中に、`kill -QUIT <pid>` を送り、全 goroutine のスタックダンプを見る。どの goroutine が、どのロックを待っているか読み取れるか。
3. `abba` の `time.Sleep(50ms)` を消す。デッドロックは起きるか。起きるなら、毎回か。**まれにしか起きないバグ**の性質を考える。
4. **改造**: 2つの口座の間の振替(`transfer(from, to, amount)`)を実装する。ロックの順序を、口座 ID で決める。**`transfer(a, b)` と `transfer(b, a)` を同時に大量に呼んで、デッドロックしないことを確認する**。
5. **改造**: `trylock` の待ち時間のランダムなばらつきを外して(固定の待ち時間にして)実行する。ライブロック的な振る舞いになるか。
6. **改造**: 哲学者の食事問題を実装し、(a)全員が左を先に取る版、(b)ロックの順序づけ版、(c)同時に座れるのを4人までにする版、を比べる。
7. 発展: `sync.Mutex` の代わりに、タイムアウトつきのロック(チャネルで実装)を作り、デッドロックを検出して報告する。

## 演習4: ロックフリーのスタック

[ex4_lockfree_stack/](ex4_lockfree_stack/)

```bash
cd ex4_lockfree_stack && go run .
```

**課題**

1. 正しさの確認(個数と合計)が通ることを確認する。
2. goroutine 数ごとの、ロックフリーと mutex の処理量を、表にする。**どの goroutine 数で逆転するか**。3回測って、ばらつきを見る。
3. `GOMAXPROCS` を 1、2、4 に制限して(`GOMAXPROCS=2 go run .`)、結果を比べる。コアが少ないと、ロックフリーの優位は、どう変わるか。
4. **改造**: ノードを、`sync.Pool` で再利用する。**ABA 問題**が起きるか。起きるなら、`(ポインタ, バージョン番号)` の組を CAS して直す(`atomic.Pointer` のかわりに、構造体のポインタを使い、バージョンを持たせる)。
5. **改造**: CAS が失敗した回数を数える(`atomic.Uint64`)。goroutine が増えると、失敗率が上がることを確認する。
6. **改造**: CAS の失敗後に、`runtime.Gosched()` や、短い待ちを入れる(バックオフ)。高い競合での処理量は、どう変わるか。
7. 発展: Michael-Scott キューを実装する。ロックを使うキューと比べる。

## 演習5: スケーラビリティ

[ex5_scalability/](ex5_scalability/)

```bash
cd ex5_scalability && go run .
```

**課題**

1. mutex、atomic、sharded の処理量を、goroutine 数に対してグラフにする。**mutex が、増やすと落ちる**ことを確認する。
2. USL の式 `C(N) = N / (1 + σ(N−1) + κN(N−1))` に、mutex の測定値を当てはめて、σ と κ を推定する(表計算の最小二乗法でよい)。κ が大きいと、何が言えるか。
3. sharded の結果は、ばらつきが大きい。3〜5回測って、中央値を取る。1 goroutine での値が不安定な理由を考える(コアの種類、周波数、スケジューリング)。
4. **改造**: sharded の `padded` 構造体のパディング(`_ [120]byte`)を外す。false sharing で、どれくらい遅くなるか(第1回)。
5. **改造**: mutex の代わりに、`sync.RWMutex` を、書き込みで使い、比べる。
6. **改造**: ロックを N 個に分割し(ロックストライピング)、goroutine ごとに、別のロックを使うように近づける。sharded との間は、どう埋まるか。
7. 発展: 各 goroutine が、ロックの外で、少し計算(例えば 100 ns)をしてからロックを取る設定にする。競合が減るので、スケーラビリティは改善するか。

## 演習6: race detector と競合状態

[ex6_race_detector/](ex6_race_detector/)

```bash
cd ex6_race_detector
go run . counter
go run -race . counter
go run . mapwrite
go run . bank
go run -race . bank
go run . bank-fixed
```

**課題**

1. `counter` を、通常と `-race` で実行する。`-race` の出力から、**どの行の、どの goroutine が競合したか**を読み取る。
2. `bank` を、通常と `-race` で、それぞれ複数回実行する。マイナスの口座の数は、どう違うか。**`-race` が何も報告しない**ことを確認する。
3. `bank-fixed` で、マイナスが 0 になることを確認する。修正版の `TryWithdraw` が、何を1つの操作にまとめたか、説明する。
4. **改造**: `counter` を、(a)`sync.Mutex`、(b)`sync/atomic`、(c)チャネルでの集約、で直して、`-race` で警告が消えることを確認する。
5. **改造**: `mapwrite` を、(a)`sync.Mutex` で守る、(b)`sync.Map` を使う、(c)キーごとに分割した複数の map(シャーディング)にする、で直して、比べる。
6. **改造**: **データ競合はないのに、不変条件が壊れる**、別のコードを書く(例: 2つのカウンタの合計が常に 100 であるべきなのに、片方を減らしてから、もう片方を増やす間に、他の goroutine が読む)。`-race` は報告するか。
7. 発展: `go test -race -count=100` で、まれにしか起きない競合を、繰り返して見つける方法を試す。

---

## 参考結果(Apple M5、macOS、論理 CPU 10、Go 1.26.5)

| 演習 | 結果 |
|---|---|
| 1 | 保護なしで 75〜81% が失われた。mutex 約 0.20〜0.22 秒、アトミック 約 0.34〜0.41 秒、スレッドごと 約 0.003 秒 |
| 2 | 20 スレッドで、チケットロック 0.11 Mops/s(公平さ 0.001)、pthread mutex 87 Mops/s |
| 3 | `abba` は検出、`hidden` は 3 秒待っても終了せず。`ordered`、`trylock` は完了 |
| 4 | 4 goroutine でロックフリー 22.4 対 mutex 10.6、8 goroutine で 6.0 対 12.1 (Mops/s) |
| 5 | mutex は 1 goroutine の約 280 から、4 以上で約 14 Mops/s に落ちた。sharded は数千 Mops/s |
| 6 | `counter` は 8M のうち約 2.0M。`bank` は通常 0〜4、`-race` で 179〜613。race detector は `bank` を報告しない |

数値は環境で変わる。本文の §11 に、測定でつまずいた点を記した。
