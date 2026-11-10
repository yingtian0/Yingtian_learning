# 第5回 参考文献

## 教科書・読み物(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Arpaci-Dusseau, *OSTEP* | 36〜45章(I/O デバイス、HDD、RAID、ファイルとディレクトリ、ファイルシステムの実装、FFS、クラッシュ一貫性(fsck とジャーナリング)、LFS、SSD、データの完全性) | 本回の中心。42章(クラッシュ一貫性)は、本文 §6 の元になる説明が、段階的に書かれている |
| Kleppmann, *Designing Data-Intensive Applications*(初版) | 3章(ストレージと検索: LSM-tree と B-tree) | データ構造の比較が、実務の視点で整理されている |
| Petrov, *Database Internals* | 前半(ストレージエンジン: B-tree、LSM-tree) | ページ構造、WAL、コンパクションの詳細 |
| Luu, "Files are hard"(2015、ブログ) | 全体 | ファイルへの書き込みで、実際に何が起きうるかを、具体例で示している |
| Corbet, "Ensuring data reaches disk"(LWN、2011) | 全体 | `fsync`、ディレクトリの同期、ドライブのキャッシュの扱いの解説 |
| `man 2 fsync`、`man 2 fcntl`(macOS の `F_FULLFSYNC`)、`man 2 rename` | 該当部 | 一次資料。macOS の `fsync` が、ドライブのキャッシュまで流さないことが書いてある |

## 必読論文

1. **McKusick, Joy, Leffler, Fabry, "A Fast File System for UNIX" (ACM TOCS, 1984)**
   - FFS。シリンダグループ、ブロックサイズ、局所性の考え方。現代のファイルシステムの、多くの設計の源流。
2. **Rosenblum & Ousterhout, "The Design and Implementation of a Log-Structured File System" (ACM TOCS, 1992)**
   - すべての書き込みを、ログとして順次に行う。LSM-tree と CoW ファイルシステムの源流。
3. **Pillai, Chidambaram, Alagappan, Al-Kiswany, Arpaci-Dusseau, Arpaci-Dusseau, "All File Systems Are Not Created Equal: On the Complexity of Crafting Crash-Consistent Applications" (OSDI, 2014)**
   - アプリが、ファイルシステムの暗黙の順序に依存して、クラッシュで壊れうることを示した。本文 §6.7 の元論文。
4. **O'Neil, Cheng, Gawlick, O'Neil, "The Log-Structured Merge-Tree (LSM-Tree)" (Acta Informatica, 1996)**
   - LSM-tree の原論文。
5. **Chang et al., "Bigtable: A Distributed Storage System for Structured Data" (OSDI, 2006)**
   - SSTable、メモリテーブル、コンパクションを、大規模なシステムで実際に使った例。第14回で再読する。

## 発展

### クラッシュ一貫性
- **Prabhakaran, Arpaci-Dusseau, Arpaci-Dusseau, "Analysis and Evolution of Journaling File Systems" (USENIX ATC, 2005)**: ジャーナルのモード(ordered など)の分析。
- **Mohan, Martinez, Kadekodi, Agarwal, Chidambaram, "Finding Crash-Consistency Bugs with Bounded Black-Box Crash Testing" (OSDI, 2018)**: クラッシュの時点を網羅して試す手法(ACE と CrashMonkey)。
- **Chen, Ziegler, Chajed, Chlipala, Kaashoek, Zeldovich, "Using Crash Hoare Logic for Certifying the FSCQ File System" (SOSP, 2015)**: ファイルシステムの形式検証。
- **Rebello et al., "Can Applications Recover from fsync Failures?" (USENIX ATC, 2020)**: 本文 §5.3 の、`fsync` の失敗の扱いを、各ファイルシステムと各アプリで調べた研究。
- **Zheng et al., "Understanding the Robustness of SSDs under Power Fault" (FAST, 2013)**: 実際の SSD に、電源断を与えたとき、何が壊れるか。

### SSD
- **Agrawal et al., "Design Tradeoffs for SSD Performance" (USENIX ATC, 2008)**: SSD の内部構造と、性能の関係。
- **Bjørling et al., "ZNS: Avoiding the Block Interface Tax for Flash-based SSDs" (USENIX ATC, 2021)**: ガベージコレクションをホストに移す、ゾーン型の SSD。

### データ構造
- **Comer, "The Ubiquitous B-Tree" (ACM Computing Surveys, 1979)**: B-tree の古典的な解説。
- **Graefe, "Modern B-Tree Techniques" (Foundations and Trends in Databases, 2011)**: 実際のデータベースの B-tree の技術。
- **Bloom, "Space/Time Trade-offs in Hash Coding with Allowable Errors" (CACM, 1970)**: ブルームフィルタの原論文。
- **Athanassoulis et al., "Designing Access Methods: The RUM Conjecture" (EDBT, 2016)**: 読み出し、更新、メモリのトレードオフ。
- **Dayan, Athanassoulis, Idreos, "Monkey: Optimal Navigable Key-Value Store" (SIGMOD, 2017)** と **Dayan & Idreos, "Dostoevsky: Better Space-Time Trade-Offs for LSM-Tree Based Key-Value Stores" (SIGMOD, 2018)**: LSM-tree の設計空間の理論。
- **Lu et al., "WiscKey: Separating Keys from Values in SSD-conscious Storage" (FAST, 2016)**: キーと値を分けて、書き込み増幅を減らす。

### データの完全性
- **Bairavasundaram et al., "An Analysis of Data Corruption in the Storage Stack" (FAST, 2008)**: 実際のディスクで、サイレントなデータ破損が、どれくらい起きるか。

## 読み方のコツ

1. ストレージの論文は、**測定に使ったデバイス**(HDD か SSD か、世代、キャッシュの設定)を最初に確認する。SSD の登場の前後で、結論が逆になる論文がある。
2. 耐久性の議論では、**何を障害として想定しているか**(プロセスの死、OS のクラッシュ、電源断、ディスクの故障)を、必ず区別する。本講義の演習3は、プロセスの死だけを扱っている。
3. 性能の数値は、**キャッシュが効いているか**(ページキャッシュ、ドライブのキャッシュ)を確認する。キャッシュを避けたつもりでも、避けられていないことがある。
4. 実機の測定値は、**OS とファイルシステムとデバイス**とセットで記録する。本講義の数字は、macOS、APFS、Apple M5 の内蔵 SSD のものである。
