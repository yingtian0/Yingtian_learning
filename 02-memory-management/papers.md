# 第2回 参考文献

## 教科書(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Arpaci-Dusseau, *Operating Systems: Three Easy Pieces* (OSTEP) | 13〜24章(アドレス空間、アドレス変換、セグメンテーション、空き領域管理、ページング、TLB、多段ページテーブル、スワップ) | 無料で読める。仮想メモリの考え方が、段階的に積み上がる。各章の末尾の「ホームワーク」(シミュレータ)も有用 |
| Bryant & O'Hallaron, *CS:APP* | 9章(仮想メモリ) | プログラマ視点で、`malloc` の実装(暗黙の空きリスト、結合)まで書く演習が付く。演習4の前に読むとよい |
| Love, *Linux Kernel Development* | メモリ管理の章 | Linux 特有の実装(スラブ、ページキャッシュ、OOM killer)の入門 |
| Gorman, *Understanding the Linux Virtual Memory Manager* | 通読ではなく、必要な箇所の参照 | 詳しいが、カーネルのバージョンが古い。構造の理解に使い、細部は現行のドキュメントで確認する |

## 必読論文

1. **Denning, "Virtual Memory" (ACM Computing Surveys, 1970)**
   - 仮想メモリの概念を整理した古典。当時の問題設定を知ると、現代の仕組みの「理由」が見える。
2. **Denning, "The Working Set Model for Program Behavior" (CACM, 1968)**
   - 作業集合とスラッシングの概念の出典。§9 の説明の元になる。
3. **Wilson, Johnstone, Neely, Boles, "Dynamic Storage Allocation: A Survey and Critical Review" (IWMM, 1995)**
   - アロケータの研究が、実際の負荷を使わずに行われてきたことへの批判と、断片化の整理。長い(約80ページ)が、断片化の理解が深まる。
4. **Bonwick, "The Slab Allocator: An Object-Caching Kernel Memory Allocator" (USENIX, 1994)**
   - 同じ型のオブジェクトを、初期化済みのまま再利用するという発想。Linux のカーネル内アロケータの原型。
5. **Crotty, Leis, Pavlo, "Are You Sure You Want to Use MMAP in Your DBMS?" (CIDR, 2022)**
   - §7.3 の元論文。mmap の問題を、実測で示す。短く読みやすい。

## 発展

### アドレス変換とハードウェア
- **Basu et al., "Efficient Virtual Memory for Big Memory Servers" (ISCA, 2013)**: TLB ミスのコストを実測し、大きな領域には、ページングを使わず基底と上限で変換する方式を提案。
- **Barr, Cox, Rixner, "Translation Caching: Skip, Don't Walk (the Page Table)" (ISCA, 2010)**: ページウォーク中の途中の表のエントリを、専用のキャッシュに置く工夫。
- **Bhargava et al., "Accelerating Two-Dimensional Page Walks for Virtualized Systems" (ASPLOS, 2008)**: 仮想化での 24 回の参照の問題。

### アロケータ
- **Berger et al., "Hoard: A Scalable Memory Allocator for Multithreaded Applications" (ASPLOS, 2000)**: スレッドごとのヒープと、false sharing 対策(第1回の内容との接点)。
- **Leijen, Zorn, de Moura, "Mimalloc: Free List Sharding in Action" (2019)**: 空きリストの分割による、局所性の改善。
- **jemalloc と tcmalloc の設計文書**: 各プロジェクトの公式ドキュメント。サイズクラスの設計や、スレッドキャッシュの考え方。

### 安全性と信頼性
- **Serebryany et al., "AddressSanitizer: A Fast Address Sanity Checker" (USENIX ATC, 2012)**: シャドウメモリによる検出の仕組み。
- **Kim et al., "Flipping Bits in Memory Without Accessing Them" (ISCA, 2014)**: Rowhammer の原論文。

### 古典
- **Kilburn, Edwards, Lanigan, Sumner, "One-Level Storage System" (IRE Transactions on Electronic Computers, 1962)**: Atlas 計算機。仮想メモリの源流。
- **Belady, "A Study of Replacement Algorithms for a Virtual-Storage Computer" (IBM Systems Journal, 1966)**: 最適なページ置換(OPT)と、FIFO の異常(Belady の異常)。

## Go のメモリを理解するための資料

- **A Guide to the Go Garbage Collector**(go.dev/doc/gc-guide): GC のコスト、`GOGC`、`GOMEMLIMIT`、スタックとヒープ。
- **Go のランタイムのソース**: `runtime/malloc.go` の冒頭に、アロケータの設計が書かれている。mcache、mcentral、mheap の関係を、コードのコメントで確認できる。
- **`go build -gcflags=-m`**: エスケープ解析の結果を、コンパイラ自身に出力させる。

## 一次資料の探し方

- Linux のメモリ管理の最新の説明: kernel.org の `Documentation/admin-guide/mm/` と `Documentation/mm/`。
- x86-64 のページングの正確な仕様: Intel SDM Vol.3 の "Paging" の章、AMD APM Vol.2。
- ARM のメモリ管理: ARM Architecture Reference Manual の "The AArch64 Virtual Memory System Architecture"。
- 実機のキャッシュ・TLB 構成の測定: Chips and Cheese の解析記事など。公式値と実測値は異なることがあるので、両方確認する。

## 読み方のコツ

1. 古い論文(1960〜70年代)は、**当時の制約**(メモリの高さ、ディスクの遅さ)を最初に確認する。その制約が、今も当てはまるかを考える。
2. アロケータや OS の論文では、「**どんな負荷を想定して評価したか**」を必ず見る。負荷が違えば、結論は変わる。
3. 実測の数値は、**環境(機種、OS、バージョン)とセットで**記録する。本講義の数値も、Apple M5 の上のものである。
