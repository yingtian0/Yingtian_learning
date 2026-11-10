# 第3回 演習

ex1〜ex3、ex5 は C(POSIX)。ex4 と ex6 は Go。macOS(Apple Silicon)で動作を確認した。Linux でも動くように書いてあるが、**Linux での実行は未確認**。

```bash
cd 03-processes-and-scheduling/exercises
```

## 進め方

第1・2回と同じ。実行前に予想を書き、実行して比べ、外れたら理由を調べる。各計測は3回以上行う。

注意: **ex5 は、全 CPU を数秒間使い切る**。他の作業を止めてから実行する。

---

## 演習1: ミニシェル

[ex1_mini_shell.c](ex1_mini_shell.c)

```bash
cc -O1 -Wall -Wextra -o ex1 ex1_mini_shell.c
./ex1                                                   # 対話
printf 'seq 1 5 | sort -r | head -2\nexit\n' | ./ex1    # 非対話
```

**課題**

1. 次を実行して、結果を確認する: `echo hello | tr a-z A-Z`、`ls /nonexistent`(終了コードの表示)、`seq 1 5 | sort -r | head -2 > /tmp/x.txt`。
2. `run_pipeline` の、親が**パイプの両端を閉じる**ループを、一時的に消して実行する。`seq 1 5 | sort` が、どうなるか。なぜか。
3. `run_child` の `signal(SIGINT, SIG_DFL)` を消す。対話モードで `sleep 100` を実行し、Ctrl-C を押す。何が起きるか。なぜか。
4. **機能追加**: バックグラウンド実行(`&`)を実装する。`wait` を呼ばずに次のプロンプトに戻ると、終了した子がゾンビになる。SIGCHLD ハンドラで回収する(ex3 を参照)。
5. **機能追加**: `cd -` や、環境変数の展開(`$HOME`)を実装する。
6. **機能追加**: 標準エラーのリダイレクト(`2>`)を実装する。fd の付け替えの理解が深まる。
7. 発展: ジョブ制御(Ctrl-Z、`fg`、`bg`)を実装する。プロセスグループと端末の制御(`setpgid`、`tcsetpgrp`)が必要になる。

## 演習2: システムコールとコンテキストスイッチのコスト

[ex2_context_switch.c](ex2_context_switch.c)

```bash
cc -O2 -pthread -o ex2 ex2_context_switch.c && ./ex2
```

**課題**

1. 3つの値(システムコール、スレッド間、プロセス間)を、3回以上計測して、ばらつきを記録する。
2. Linux で実行できるなら、`taskset -c 0 ./ex2` で1コアに固定して比べる。固定すると、値はどう変わるか。固定しない場合との差は、何によるか。
3. **改造**: 2つの実行主体の間で、送るデータを 1 バイトから 4KB、64KB に増やす。どこから、切り替えのコストよりデータの転送が支配的になるか。
4. **改造**: `getppid` を、ほかのシステムコール(`getpid`、`read(/dev/zero)`、`write(/dev/null)`)に変えて、コストの違いを比べる。
5. **改造**: ピンポンの途中で、各側に、メモリを触る処理(例えば 2MB の配列の走査)を加える。コンテキストスイッチの**間接コスト**(キャッシュが冷える影響)が見えるか。
6. 発展: Linux で `perf stat -e context-switches,cpu-migrations ./ex2` を使い、コアの移動の回数を見る。

## 演習3: ゾンビ、孤児、シグナル

[ex3_zombie_orphan_signal.c](ex3_zombie_orphan_signal.c)

```bash
cc -O1 -Wall -Wextra -o ex3 ex3_zombie_orphan_signal.c
./ex3 zombie
./ex3 orphan
./ex3 signals
```

**課題**

1. `zombie` モードで、STAT が Z になることを確認する。`sleep(1)` の間に、別の端末から `ps -o pid,ppid,stat,comm -p <子のPID>` を見る。
2. `orphan` モードで、孫の `ppid` が、実際に 1 になることを確認する。Linux のコンテナの中で実行すると、ppid は何になるか(PID 1 が誰か)。
3. `signals` モードの (2) で、`sigprocmask` によるブロックを外すと、ハンドラは何回呼ばれるか。通常のシグナルとリアルタイムシグナル(`SIGRTMIN` 以降)の違いを調べる。
4. SIGCHLD ハンドラの `while (waitpid(...) > 0)` を、`waitpid` 1回だけに変えて、同時に多数の子が終了するケース(例: 100 個の子を同時に終了させる)を試す。ゾンビが残るか。
5. **改造**: ハンドラの中で `printf` を呼ぶ。通常は動くが、なぜ危険か。`write` を使う安全な書き方に直す。
6. **改造**: SIGTERM を受け取ったら、フラグを立てて、メインのループで後片付けしてから終了する、グレースフルシャットダウンを実装する。
7. 発展: Go で同じものを書く。`signal.Notify` を使い、SIGTERM でサーバーを止める。`docker run` の PID 1 で、シグナルが届くことを確認する。

## 演習4: スケジューラのシミュレータ(Go)

[ex4_sched_sim/](ex4_sched_sim/)

```bash
cd ex4_sched_sim && go run .
```

**課題**

1. 出力を読み、護送船団効果(ワークロード1)と、RR のトレードオフ(ワークロード2)を、自分の言葉で説明する。
2. ワークロード1で、**SJF が FCFS と同じ結果**になることを、時間を追って確認する。短いジョブが、L より先に到着するように変えると、結果はどう変わるか。
3. **改造**: 長さがばらばらのジョブ(例えば 1〜50 の乱数)を 100 個生成し、各方式の平均ターンアラウンドと平均応答を比べる。何度か乱数を変えて試す。
4. **改造**: **MLFQ** を実装する(優先度3段、クォンタムはそれぞれ 2、4、8、一定時間ごとに全員を戻す)。ワークロード1で、SRTF と比べる。MLFQ は、ジョブの長さを知らずに、SRTF に近づけるか。
5. **改造**: 実行中に I/O で待つジョブ(CPU 5 単位 → 待ち 10 単位 → CPU 5 単位)を扱えるようにする。待ちの間、CPU は他のジョブに使われる。
6. **改造**: 簡易な**スロットリング**を実装する。「100 単位ごとに、合計 50 単位まで」という CPU クォータを課し、使い切ったら、周期の終わりまで全ジョブを止める。応答時間の分布は、どう変わるか。

## 演習5: nice 値の効き方

[ex5_nice_hogs.c](ex5_nice_hogs.c)

```bash
cc -O1 -o ex5 ex5_nice_hogs.c && ./ex5 4
```

**課題**

1. nice 0 と nice 10 のグループの取り分を記録する。**Linux の CFS の重みから予想される 90.3% 対 9.7% と比べる。**
2. Linux で実行できるなら、同じ結果になるか。違えば、その理由を調べる。
3. 論理 CPU の数より少ないプロセス数(例: 各グループ 2 個)で実行する。CPU が余っていれば、nice 値は取り分に影響するか。
4. **改造**: nice 値を、0 対 5、0 対 19 など変えて、取り分の関係をグラフにする。
5. **改造**: macOS で、プロセスの QoS クラス(`pthread_set_qos_class_self_np`)を変えて、取り分がどう変わるか。性能コアと効率コアの使い分けとの関係は。
6. 発展: Linux の cgroup v2 で、2つの cgroup に CPU の重み(`cpu.weight`)を設定して、取り分を測る。

## 演習6: Go のスケジューラ

[ex6_go_scheduler/](ex6_go_scheduler/)

```bash
cd ex6_go_scheduler
go run . pingpong
go run . preempt
# 非同期プリエンプションを切ると、止まったままになる (外から止める)
go build -o /tmp/ex6 . && (GODEBUG=asyncpreemptoff=1 /tmp/ex6 preempt & pid=$!; sleep 4; kill $pid)
```

**課題**

1. goroutine の切り替え(約 100 ns)と、OS スレッド(演習2)の切り替えを比べる。
2. `GOMAXPROCS` を 1、2、4、既定と変えて、ピンポンの結果を比べる。なぜ、変わる(または変わらない)か。
3. `GODEBUG=schedtrace=1000` を付けて実行し、スケジューラの状態(各 P のキューの長さなど)を読む。
4. **改造**: 多数(10 万個)の goroutine を作り、それぞれがチャネルで待つ。メモリ使用量(`runtime.MemStats` の `StackInuse`)と、作成時間を測る。同じことを、OS スレッドでやるのが、なぜ難しいか。
5. **改造**: ブロッキングなシステムコール(例えば `syscall.Read` で、パイプの読み込み待ち)に入る goroutine を、`GOMAXPROCS(1)` の状態で多数作る。他の goroutine は動き続けるか。ランタイムは、どう対処しているか(`runtime.NumGoroutine`、スレッド数 `runtime/pprof` の `threadcreate`)。
6. **改造**: CPU を占有する関数(関数呼び出しのあるループと、ないループ)を、`GODEBUG=asyncpreemptoff=1` の有無で比べる。関数呼び出しがあれば、止められるか。
7. 発展: `go tool trace` を使って、goroutine のスケジューリングを可視化する。

---

## 参考結果(Apple M5、macOS、論理 CPU 10、Go 1.26.5)

| 演習 | 結果 |
|---|---|
| 1 | パイプ、リダイレクト、`cd` が動く。存在しないコマンドは 127、`ls` の失敗は `[exit 1]` |
| 2 | システムコール約 100 ns。スレッド間 約 2.8〜3.3 µs/往復、プロセス間 約 4.0〜4.4 µs/往復(プロセス ÷ スレッド 1.3〜1.5 倍) |
| 3 | ゾンビは STAT=Z。孤児の ppid は 1。SIGKILL は設定できず(EINVAL)。SIGUSR1 を 5 回送ってハンドラは 1 回。SIGCHLD で 3 つとも回収 |
| 4 | 護送船団: FCFS/SJF 108.0、SRTF 32.8、RR(q=10) 40.0。RR の q=1 は、切り替えコスト 1 で 195.0 |
| 5 | nice 0 が 68.3%、nice 10 が 31.7%(Linux の重みの予想は 90/10) |
| 6 | goroutine の切り替え約 100 ns。固定した OS スレッドは約 8 µs。asyncpreemptoff で止まる |

数値は環境で変わる。本文の §14 に、測定でつまずいた点を記した。
