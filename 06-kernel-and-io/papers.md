# 第6回 参考文献

## 教科書・読み物(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Stevens, Fenner, Rudoff, *UNIX Network Programming, Vol. 1* | 6章(I/O 多重化: select と poll)、I/O モデルの比較 | 5つの I/O モデル(ブロッキング、ノンブロッキング、多重化、シグナル駆動、非同期)の古典的な整理。本文 §3 の元 |
| Love, *Linux Kernel Development* | システムコール、割り込みと割り込みハンドラ、割り込みの下半分 | 本文 §1〜2 の、Linux での実装 |
| Gregg, *Systems Performance*(2nd) | CPU、ファイルシステム、ネットワークの章 | 実務での観測の方法論(USE メソッド、`strace`、`perf`) |
| Gregg, *BPF Performance Tools* | 全体(特に、導入と、システムコールの観測) | eBPF での観測の実践 |
| Kegel, "The C10K problem"(1999、web) | 全体 | C10K の問題提起と、当時の解決策の整理。歴史的に必読 |
| `man 2 poll`、`man 7 epoll`、`man 2 kqueue`、`man 2 sendfile`、`man 7 socket`、`man 2 io_uring_setup` | 該当部 | 一次資料。OS ごとの引数と挙動の違いが書いてある |

## 必読論文

1. **Lemon, "Kqueue: A Generic and Scalable Event Notification Facility" (USENIX ATC, 2001)**
   - `kqueue` の設計。`select` と `poll` の O(n) の問題と、それを解く、登録型の通知。
2. **Mogul & Ramakrishnan, "Eliminating Receive Livelock in an Interrupt-driven Kernel" (USENIX ATC, 1996; ACM TOCS, 1997)**
   - 本文 §2.4 の元論文。割り込みが CPU を食い尽くす問題と、ポーリングへの切り替え。
3. **Axboe, "Efficient IO with io_uring" (2019、kernel.dk の文書)**
   - `io_uring` の設計者による解説。提出キューと完了キュー、ポーリングのモード。
4. **Soares & Stumm, "FlexSC: Flexible System Call Scheduling with Exception-Less System Calls" (OSDI, 2010)**
   - システムコールを、共有メモリ経由で、まとめて出す方式。`io_uring` の先駆け。システムコールのコスト(直接と、間接のキャッシュ汚染)の分析も含む。
5. **Pai, Druschel, Zwaenepoel, "Flash: An Efficient and Portable Web Server" (USENIX ATC, 1999)**
   - イベント駆動のサーバーで、ディスク I/O のブロックを、ヘルパープロセスで解決する(AMPED)。本文 §3.2 の「通常のファイルには準備通知が効かない」への、古典的な答え。

## 発展

### イベント駆動 vs スレッド
- **Welsh, Culler, Brewer, "SEDA: An Architecture for Well-Conditioned, Scalable Internet Services" (SOSP, 2001)**: ステージ化されたイベント駆動。
- **von Behren, Condit, Brewer, "Why Events Are a Bad Idea (for High-Concurrency Servers)" (HotOS, 2003)**: イベント駆動に対する、スレッド側からの反論。
- **Nystrom, "What Color is Your Function?" (2015、ブログ)**: 同期と非同期の関数を、型として区別することの代償。

### カーネルバイパス
- **Belay et al., "IX: A Protected Dataplane Operating System for High Throughput and Low Latency" (OSDI, 2014)** と **Peter et al., "Arrakis: The Operating System is the Control Plane" (OSDI, 2014)**: OS を、制御の層と、データの層に分ける。
- **Zhang et al., "The Demikernel Datapath OS Architecture for Microsecond-scale Datacenter Systems" (SOSP, 2021)**: マイクロ秒の単位のシステムのための OS。
- **Rizzo, "netmap: A Novel Framework for Fast Packet I/O" (USENIX ATC, 2012)**: パケット I/O の高速な枠組み。

### eBPF
- **McCanne & Jacobson, "The BSD Packet Filter: A New Architecture for User-level Packet Capture" (USENIX Winter, 1993)**: BPF の原論文。
- eBPF の公式ドキュメント(ebpf.io)、kernel.org の `Documentation/bpf/`。

### Linux の資料
- LWN.net の `io_uring` に関する記事(2019 年、"Ringing in a new asynchronous I/O API")。
- Go のランタイムのソース: `runtime/netpoll.go`、`runtime/netpoll_epoll.go`、`runtime/netpoll_kqueue.go`。ネットポーラの実装。

## 読み方のコツ

1. I/O の性能の論文は、**測定の環境**(NIC の速度、コア数、カーネルのバージョン、カーネルの設定)を、最初に確認する。10 年前の結果が、今も成り立つとは限らない。
2. 「速い」の論文は、**何と比べて、どの条件で**を確認する。本講義の演習2のように、「活発な接続の割合」で、結果が逆転する。
3. **カーネルのバージョン**で、挙動(特に `epoll` の細部、`io_uring` の機能)は、大きく変わる。
4. 実機の数値は、**OS とセットで**記録する。本講義の数字は、macOS(kqueue)のもので、Linux(epoll、io_uring)では測っていない。
