# 第3回 参考文献

## 教科書(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Arpaci-Dusseau, *OSTEP* | 4〜10章(プロセス、プロセス API、制限つき直接実行、スケジューリング入門、MLFQ、比例配分、マルチ CPU スケジューリング) | 本回の中心。各章末の「ホームワーク」(スケジューラのシミュレータ)は、演習4の発展になる |
| Bryant & O'Hallaron, *CS:APP* | 8章(例外的な制御フロー: プロセス、シグナル) | シグナルの扱いの落とし穴(ハンドラの安全な書き方)が詳しい。シェルを書く演習もある |
| Stevens & Rago, *Advanced Programming in the UNIX Environment* (APUE) | プロセス、シグナル、プロセス間通信の章 | `fork`/`exec`/`wait`/シグナルの正確な仕様と、移植性の注意 |
| Love, *Linux Kernel Development* | プロセス管理、スケジューラの章 | `task_struct`、スケジューラの実装(CFS 時代の記述) |

## 必読論文

1. **Corbató, Merwin-Daggett, Daley, "An Experimental Time-Sharing System" (1962)**
   - CTSS。時分割と、のちの MLFQ の原型。
2. **Lozi, Lepers, Funston, Gaud, Quéma, Fedorova, "The Linux Scheduler: a Decade of Wasted Cores" (EuroSys, 2016)**
   - Linux の負荷分散に、一部のコアが空いているのに、待っているスレッドがいる状態が続くバグがあることを、実測と可視化ツールで示した。スケジューラの正しさを検証する難しさを学べる。
3. **Sha, Rajkumar, Lehoczky, "Priority Inheritance Protocols: An Approach to Real-Time Synchronization" (IEEE Trans. Computers, 1990)**
   - 優先度逆転の解決策の原典。
4. **Dean & Barroso, "The Tail at Scale" (CACM, 2013)**
   - テイルレイテンシが、大規模なシステムで支配的になる理由と、対策(ヘッジ要求など)。スケジューリングは、テイルの原因の1つとして出てくる。
5. **Humphries et al., "ghOSt: Fast & Flexible User-Space Delegation of Linux Scheduling" (SOSP, 2021)**
   - Google の、スケジューリングの判断をユーザー空間に委ねる仕組み。OS のスケジューラの限界と、アプリごとの最適化を考える。

## 発展

### スケジューリングの理論
- **Liu & Layland, "Scheduling Algorithms for Multiprogramming in a Hard-Real-Time Environment" (JACM, 1973)**: Rate-Monotonic と EDF。周期タスクの使用率の上限(約 69%)。
- **Waldspurger & Weihl, "Lottery Scheduling: Flexible Proportional-Share Resource Management" (OSDI, 1994)**: くじ引きによる比例配分。
- **Stoica et al., "A Proportional Share Resource Allocation Algorithm for Real-Time, Time-Shared Systems" (RTSS, 1996)**: EEVDF の原典。Linux 6.6 のスケジューラの元になった考え方。

### スレッドとユーザー空間のスケジューリング
- **Anderson, Bershad, Lazowska, Levy, "Scheduler Activations: Effective Kernel Support for the User-Level Management of Parallelism" (SOSP, 1991)**: ユーザー空間のスレッドとカーネルの協調。Go の設計とも通じる。
- **Ousterhout et al., "Shenango: Achieving High CPU Efficiency for Latency-sensitive Datacenter Workloads" (NSDI, 2019)** と **Fried et al., "Caladan: Mitigating Interference at Microsecond Timescales" (OSDI, 2020)**: 数 µs の単位で、コアをアプリ間で再配分する。
- **Go のスケジューラの設計文書**: Dmitry Vyukov, "Scalable Go Scheduler Design Doc" (2012)。G-M-P モデルと work stealing の設計の出発点。

### 割り込みとスケジューリングの接点
- **Mogul & Ramakrishnan, "Eliminating Receive Livelock in an Interrupt-driven Kernel" (USENIX ATC, 1996)**: 割り込みが多すぎて、プロセスが全く走れなくなる状態。第6回の準備になる。

### Linux の資料
- LWN.net の EEVDF に関する記事(2023年)。
- kernel.org の `Documentation/scheduler/`。
- `man 7 sched`、`man 7 signal`、`man 2 clone`。

## 読み方のコツ

1. スケジューリングの論文では、**評価に使った負荷の性質**(CPU を使い続けるか、I/O が多いか、スレッドの数)を最初に確認する。方式の優劣は、負荷で逆転する。
2. 「平均」の値だけでなく、**分布の裾(99 パーセンタイルなど)**が示されているかを確認する。
3. 実機の測定値は、**OS、カーネルのバージョン、CPU の構成**とセットで記録する。本講義の演習5が、Linux の予想と大きく違ったように、環境の違いは大きい。
