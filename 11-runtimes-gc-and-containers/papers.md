# 第11回 参考文献

> 文献は、著者・題名・発表の場・年で挙げる。**内容の要約は、著者の記憶に基づく概要であり、読むときに、原典で確認すること**。バージョンに依存する記述(Go の GC の詳細、Go 1.25 の `GOMAXPROCS` など)は、公式のドキュメントと、リリースノートで、最新を確認する。

## 教科書・読み物(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Jones, Hosking, Moss, *The Garbage Collection Handbook* (2nd) | 2〜6章(基本のアルゴリズム)、15章(並行 GC) | **GC の標準的な教科書**。本文 §2〜§3 の元 |
| Jones & Lins, *Garbage Collection: Algorithms for Automatic Dynamic Memory Management* | 全体 | 古典 |
| Go 公式 *A Guide to the Go Garbage Collector* (go.dev/doc/gc-guide) | 全体 | `GOGC`、`GOMEMLIMIT`、コストのモデル。**本文 §4〜§5 の一次資料**。最新に更新される |
| Liz Rice, *Container Security* | 名前空間、cgroup、ケーパビリティ、seccomp の章 | コンテナの仕組みを、プログラムで、確かめながら学べる |
| Burns, Beda, Hightower 他, *Kubernetes: Up & Running* | リソース、プローブ、Service | 運用の基本 |
| Beyer et al., *Site Reliability Engineering*(Google) | 「Borg の、クラスタ管理」に関する章、「連鎖した障害への対処」 | 第15回でも使う |
| Linux カーネルの文書: `Documentation/admin-guide/cgroup-v2.rst`、`scheduler/sched-bwc.rst` | cpu、memory の章、CFS 帯域制御 | **cgroup v2 と、スロットリングの、一次資料**。本文 §6.3、§7 の元 |
| `man 7 namespaces`、`man 7 cgroups`、`man 2 clone` | 全体 | 名前空間の一次資料 |

## 必読論文

1. **McCarthy, "Recursive Functions of Symbolic Expressions and Their Computation by Machine, Part I" (CACM, 1960)**
   - ガベージコレクションの、最初の記述。マーク & スイープの原型。
2. **Cheney, "A Nonrecursive List Compacting Algorithm" (CACM, 1970)**
   - 本文 §2.4 の、コピー GC(半空間)。再帰を使わない。
3. **Ungar, "Generation Scavenging: A Non-Disruptive High Performance Storage Reclamation Algorithm" (ACM SIGSOFT/SIGPLAN, 1984)**
   - **世代別 GC の、原型**。弱い世代仮説に基づく。
4. **Dijkstra, Lamport, Martin, Scholten, Steffens, "On-the-Fly Garbage Collection: An Exercise in Cooperation" (CACM, 1978)**
   - **並行 GC と、3 色のマーキング**の、元の論文。本文 §3 の、挿入型バリアの源流。
5. **Yuasa, "Real-Time Garbage Collection on General-Purpose Machines" (Journal of Systems and Software, 1990)**
   - 削除型の書き込みバリア。
6. **Bacon, Cheng, Rajan, "A Real-Time Garbage Collector with Low Overhead and Consistent Utilization" (POPL, 2003)**
   - リアルタイム GC。**停止の上限を保証する**試み。本文 §9 の論点 1。
7. **Agache et al., "Firecracker: Lightweight Virtualization for Serverless Applications" (NSDI, 2020)**
   - マイクロVM。隔離と密度の交換(本文 §6.5)。
8. **Verma et al., "Large-scale cluster management at Google with Borg" (EuroSys, 2015)**
   - コンテナによる、クラスタ管理の、実際の運用。Kubernetes の、前身。
9. **Burns, Grant, Oppenheimer, Brewer, Wilkes, "Borg, Omega, and Kubernetes" (ACM Queue, 2016)**
   - 3 世代の、設計の振り返り。

## 発展

### GC
- **Blackburn & McKinley, "Immix: A Mark-Region Garbage Collector with Space Efficiency, Fast Collection, and Mutator Performance" (PLDI, 2008)**: マーク & スイープと、コピーの、中間。
- **Tene, Iyengar, Wolf, "C4: The Continuously Concurrent Compacting Collector" (ISMM, 2011)**: 並行に、**動かす(コンパクション)**GC。
- **Yang et al., "Deep Dive into ZGC: A Modern Garbage Collector in OpenJDK" (ACM TOPLAS, 2022)**: 現代的な、低停止の GC の設計。
- **Go の GC に関する、公式の設計文書**(`golang.org/s/go15gcpacing`、ハイブリッド書き込みバリアの提案(Austin Clements、Rick Hudson)、`GOMEMLIMIT` の設計文書): 本文 §3.3、§4、§5.4 の、**詳細な、一次資料**。
- **Bacon, Cheng, Rajan, "The Metronome: A Simpler Approach to Garbage Collection in Real-Time Systems"** (2003): リアルタイム GC の、別の方式。
- **Cai et al., "Distilling the Real Cost of Production Garbage Collectors" (ISPASS, 2022)**: **GC の、実際のコストの、測り方**(停止時間だけでは、不十分)。本文 §4.4 の問題意識に近い。

### メモリの管理
- **Wilson et al., "Uniprocessor Garbage Collection Techniques" (IWMM, 1992)**: GC の、古典的なサーベイ。
- **Hertz & Berger, "Quantifying the Performance of Garbage Collection vs. Explicit Memory Management" (OOPSLA, 2005)**: GC と手動管理の、実測の比較。本文 §9 の論点 4。
- **Jung et al.、Rust の所有権の、形式化(RustBelt, POPL, 2018)**: コンパイル時の、メモリ管理の理論的な基礎。

### コンテナと隔離
- **Young et al., "The True Cost of Containing: A gVisor Case Study" (HotCloud, 2019)**: ユーザ空間のカーネルの、コスト。
- **Soltesz et al., "Container-based Operating System Virtualization: A Scalable, High-performance Alternative to Hypervisors" (EuroSys, 2007)**: コンテナの、初期の設計の評価。
- **Felter et al., "An Updated Performance Comparison of Virtual Machines and Linux Containers" (ISPASS, 2015)**: VM とコンテナの、性能の比較。
- **Dan Luu 他、CPU スロットリングに関する、公開された事例の分析**(「CPU limit を、設定すべきか」の議論): 本文 §7 の、実務上の含意の背景。**ブログ等の二次資料なので、条件を、確認して読む**。
- **Linux カーネルの `sched-bwc.rst` と、CFS 帯域制御の、改良に関する、公開されたパッチの議論**: スロットリングの、不具合と、修正の歴史。

## 読み方のコツ

1. **GC の論文は、「何を測ったか」を確認する**。スループット(総実行時間)、停止時間の最大、停止時間の分布、メモリの使用量、は、**別の指標**。一つの指標だけの優位は、他を犠牲にしている可能性がある(演習2: 停止は短いが、p99 が悪い)。
2. **ベンチマークの、ヒープの特性**(寿命の分布、ポインタの密度、生きている量)を確認する。弱い世代仮説が成り立たない作業負荷では、世代別は、有利でない。
3. **コンテナの論文や記事は、カーネルのバージョン、cgroup のバージョン(v1 か v2)**を確認する。CFS 帯域制御の挙動は、カーネルのバージョンで変わっている。
4. **「ベンチマークで速い」は、「本番の、複数のテナントが混ざった環境で速い」とは、違う**。隣のコンテナの影響(ノイジーネイバー)は、単独のベンチマークでは、見えない。
5. **シミュレーションは、傾向を示すが、実際の値ではない**。演習3、5 の数値は、モデルの結果である。
6. **この講義の、確かめられていない仮説**(本文 §10)は、論文や、公式の文書で、裏づけを探し、可能なら、自分で、実験する。
