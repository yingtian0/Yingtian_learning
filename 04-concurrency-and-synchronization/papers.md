# 第4回 参考文献

## 教科書(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Herlihy & Shavit, *The Art of Multiprocessor Programming* | 2章(相互排除)、3章(並行オブジェクト、線形化可能性)、5章(コンセンサス数)、7章(スピンロックと競合)、9章(リスト)、10章(キュー、メモリ回収、ABA 問題)、11章(スタック) | 本回の理論面の中心。**この分野の標準的な教科書**。各アルゴリズムの正しさの議論が丁寧 |
| Arpaci-Dusseau, *OSTEP* | 26〜34章(並行性の入門、スレッド API、ロック、ロックベースのデータ構造、条件変数、セマフォ、並行性のバグ、イベントベースの並行性) | 無料。実装を追いながら学べる。32章(並行性のバグ)は、Lu ら(2008)の研究を紹介している |
| McKenney, *Is Parallel Programming Hard, And, If So, What Can You Do About It?* | 全体(特に、ハードウェアの習慣、カウント、RCU) | 無料で公開されている。RCU の作者による、実践的な本 |
| Go の公式資料 | The Go Memory Model(go.dev/ref/mem)、Effective Go の並行性の節、Data Race Detector(go.dev/doc/articles/race_detector) | **Go で並行処理を書く人の必読**。メモリモデルは、happens-before の規則が正確に書かれている |

## 必読論文

1. **Dijkstra, "Solution of a Problem in Concurrent Programming Control" (CACM, 1965)**
   - 相互排除問題の、最初の解。ハードウェアの特別な命令なしで解いた。1ページ。
2. **Coffman, Elphick, Shoshani, "System Deadlocks" (ACM Computing Surveys, 1971)**
   - デッドロックの4つの必要条件の出典。
3. **Mellor-Crummey & Scott, "Algorithms for Scalable Synchronization on Shared-Memory Multiprocessors" (ACM TOCS, 1991)**
   - MCS ロックと、スケーラブルなバリア。キャッシュコヒーレンスの影響を、実測で示した。第1回の知識と、直結する。
4. **Herlihy & Wing, "Linearizability: A Correctness Condition for Concurrent Objects" (ACM TOPLAS, 1990)**
   - 線形化可能性の定義。並行データ構造の正しさの基準。
5. **Lu, Park, Seo, Zhou, "Learning from Mistakes: A Comprehensive Study on Real World Concurrency Bug Characteristics" (ASPLOS, 2008)**
   - 実際の並行バグ 105 件の分析。原子性の違反と順序の違反が大半、という知見。

## 発展

### 同期の理論
- **Herlihy, "Wait-Free Synchronization" (ACM TOPLAS, 1991)**: コンセンサス数と、CAS の万能性。
- **Lamport, "How to Make a Multiprocessor Computer That Correctly Executes Multiprocess Programs" (1979)**: 逐次一貫性。第1回で読んだものに、並行性の視点で戻る。
- **Boehm & Adve, "Foundations of the C++ Concurrency Memory Model" (PLDI, 2008)**: データ競合のないプログラムを逐次一貫にする(DRF-SC)保証の、言語レベルの確立。

### ロックフリーとメモリ回収
- **Treiber, "Systems Programming: Coping with Parallelism" (IBM Research Report RJ 5118, 1986)**: Treiber スタックの出典。
- **Michael & Scott, "Simple, Fast, and Practical Non-Blocking and Blocking Concurrent Queue Algorithms" (PODC, 1996)**: ロックフリーキューの標準。
- **Michael, "Hazard Pointers: Safe Memory Reclamation for Lock-Free Objects" (IEEE TPDS, 2004)**: メモリ回収問題の解の1つ。
- **McKenney & Slingwine, "Read-Copy Update: Using Execution History to Solve Concurrency Problems" (1998)**: RCU の原論文。

### OS の実装
- **Franke, Russell, Kirkwood, "Fuss, Futexes and Furwocks: Fast Userlevel Locking in Linux" (OLS, 2002)** と **Drepper, "Futexes Are Tricky" (2011)**: futex の仕組みと、正しく使う難しさ。

### ツールと測定
- **Serebryany & Iskhodzhanov, "ThreadSanitizer: Data Race Detection in Practice" (WBIA, 2009)**: Go の `-race` の元になった技術。
- **Gunther, *Guerrilla Capacity Planning*** (Universal Scalability Law): §9.2 の式の出典。
- **Tu, Liu, Song, Zhang, "Understanding Real-World Concurrency Bugs in Go" (ASPLOS, 2019)**: Go の実際の並行バグの分析。

### 別のアプローチ
- **Hoare, "Communicating Sequential Processes" (CACM, 1978)**: Go のチャネルの思想の源流。
- **Herlihy & Moss, "Transactional Memory: Architectural Support for Lock-Free Data Structures" (ISCA, 1993)**: トランザクショナルメモリの提案。

## 読み方のコツ

1. 並行アルゴリズムの論文は、**正しさの証明の構造**(何を不変条件としているか)を読む。細部を全部追わなくてよい。
2. 並行データ構造の論文の性能評価では、**競合の度合い**(スレッド数、操作の割合、データの大きさ)の設定を確認する。第4回の演習4で見たように、競合の度合いで、優劣が逆転する。
3. 並行バグの研究は、**どんなソフトウェア**(データベースか、Web サーバーか、アプリか)を調べたかで、傾向が違う。
4. ロックの性能の議論は、**ハードウェア**(コア数、キャッシュの構成、NUMA)とセットで読む。本講義の数字も、Apple M5 のものである。
