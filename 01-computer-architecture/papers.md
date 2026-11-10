# 第1回 参考文献

## 教科書(通読 or 該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Hennessy & Patterson, *Computer Architecture: A Quantitative Approach* (6th) | Ch.1(性能と定量的設計)、Ch.2(メモリ階層)、Ch.3(ILP)、Ch.5(スレッドレベル並列) | 「定量的に設計判断する」姿勢。CPI の分解、AMAT、Amdahl が体系的に出てくる。付録 B(キャッシュ入門)も有用 |
| Bryant & O'Hallaron, *Computer Systems: A Programmer's Perspective* (CS:APP) | Ch.5(プログラム最適化)、Ch.6(メモリ階層) | プログラマ視点の最良の入門。キャッシュフレンドリーなコードの書き方 |
| Sorin, Hill, Wood, *A Primer on Memory Consistency and Cache Coherence* (2nd) | 全体(約200ページ) | SC/TSO/弱モデルとコヒーレンスを統一的に学べる。**第4回の予習にもなる** |

## 必読論文

1. **Wulf & McKee, "Hitting the Memory Wall: Implications of the Obvious" (1995)**
   - 数ページ。CPU と DRAM の速度差が指数的に開く、という予測。結論の細部より「議論の立て方」を見る。
2. **Lamport, "How to Make a Multiprocessor Computer That Correctly Executes Multiprocess Programs" (1979)**
   - 逐次一貫性の定義の出典。2ページ。
3. **Sewell et al., "x86-TSO: A Rigorous and Usable Programmer's Model for x86 Multiprocessors" (CACM, 2010)**
   - x86 のメモリモデルを形式化。store buffer による Store→Load 並べ替えを、本回の演習5と対応づけて読む。
4. **Drepper, "What Every Programmer Should Know About Memory" (2007)**
   - 長編(約100ページ)。DRAM、キャッシュ、NUMA、TLB が実機寄りに詳述される。やや古い(DDR2〜3時代)が、構造の理解には今も有効。
5. **Kocher et al., "Spectre Attacks: Exploiting Speculative Execution" (2018)** / **Lipp et al., "Meltdown" (2018)**
   - §9 の元論文。まず Spectre v1 の節(分岐予測の訓練 → 境界チェックの投機的回避)だけ読む。

## 発展(興味に応じて)

- **Tomasulo, "An Efficient Algorithm for Exploiting Multiple Arithmetic Units" (1967)**: 動的スケジューリングの原点。
- **Seznec & Michaud, "A case for (partially) TAgged GEometric history length branch prediction" (2006)**: TAGE。現代の分岐予測の基礎。
- **Williams, Waterman, Patterson, "Roofline: An Insightful Visual Performance Model for Multicore Architectures" (CACM, 2009)**: §11 の Roofline。
- **Mytkowicz et al., "Producing Wrong Data Without Doing Anything Obviously Wrong!" (ASPLOS 2009)**: 計測バイアス。環境変数のサイズやリンク順で性能が変わる。**博士レベルの計測の作法**。
- **Batty et al., "Mathematizing C++ Concurrency" (POPL 2011)**: C++11 メモリモデルの形式化。
- **Alglave et al., "Herding Cats: Modelling, Simulation, Testing, and Data Mining for Weak Memory" (TOPLAS 2014)**: ARM/POWER の実機計測と形式モデル。
- **Hennessy & Patterson, "A New Golden Age for Computer Architecture" (Turing Lecture, 2018)**: §12 の論点3。
- **Intel / Arm / Apple の最適化マニュアル**: 実機のパイプライン幅、キャッシュ構成、レイテンシ表。

## 一次資料の探し方

- 論文: Google Scholar、ACM DL、arXiv。ISCA / MICRO / ASPLOS / HPCA が主要会議。
- 実機の解析: Agner Fog "Optimization manuals"、Chips and Cheese、uops.info(命令ごとのレイテンシ/スループット)。
- メモリモデルのツール: herdtools7 (`herd7`, `litmus7`)。演習5のリトマステストを形式モデルでも検証できる。

## 読み方のコツ(博士課程スタイル)

1. まず **Abstract と Conclusion**、次に **図表**、最後に本文。
2. 各論文について「**主張 / 根拠 / 仮定 / 反例になりうる状況**」の4点を自分の言葉で1行ずつ書く。
3. 古い論文は「**今も成り立つか**」を最後に自問する(例: メモリの壁は HBM で解消したか)。
