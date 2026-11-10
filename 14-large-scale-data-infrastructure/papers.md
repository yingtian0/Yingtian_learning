# 第14回 参考文献

> 文献は、著者・題名・発表の場・年で挙げる。**内容の要約は、概要であり、読むときに、原典で確認すること**。各システムの現在の挙動は、論文の発表後に、変わっている。**公式の文書で、最新を確認する**。

## 教科書・読み物(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Kleppmann, *Designing Data-Intensive Applications* | 5〜6章(複製、パーティショニング)、10〜11章(バッチ、ストリーム) | **この回の、最良の、総合的な読み物** |
| Beyer et al., *Site Reliability Engineering*(Google) | データ処理、データの完全性の章 | 運用の視点。第15回でも使う |
| Petrov, *Database Internals* | 分散システムの部(複製、アンチエントロピー、合意) | 実装寄りの解説 |
| Jepsen の公開された分析(jepsen.io) | 各データベース | 一貫性の、実際の挙動 |
| Kreps, "The Log: What every software engineer should know about real-time data's unifying abstraction"(2013 のブログ記事) | 全体 | 本文 §7.1「ログという抽象」の元 |

## 必読論文

1. **Ghemawat, Gobioff, Leung, "The Google File System" (SOSP, 2003)**
   - GFS。本文 §3。単一マスター、大きなチャンク、緩い一貫性。
2. **Chang et al., "Bigtable: A Distributed Storage System for Structured Data" (OSDI, 2006)**
   - Bigtable。本文 §4。タブレット、SSTable、Chubby。
3. **Dean & Ghemawat, "MapReduce: Simplified Data Processing on Large Clusters" (OSDI, 2004)**
   - MapReduce。本文 §8。**バックアップタスク**(投機的実行)の節を、必ず、読む。
4. **DeCandia et al., "Dynamo: Amazon's Highly Available Key-value Store" (SOSP, 2007)**
   - Dynamo。本文 §5。**あいまいなクォーラム、ヒント付き引き継ぎ、ベクタークロック、Merkle 木**。
5. **Corbett et al., "Spanner: Google's Globally-Distributed Database" (OSDI, 2012)**
   - Spanner。本文 §6。第12〜13回の、TrueTime と 2PC の、実際の姿。
6. **Lakshman & Malik, "Cassandra: A Decentralized Structured Storage System" (ACM SIGOPS OSR, 2010)**
   - Dynamo と Bigtable の、組み合わせ。
7. **Kreps, Narkhede, Rao, "Kafka: a Distributed Messaging System for Log Processing" (NetDB, 2011)**
   - Kafka の、設計。本文 §7。
8. **Karger et al., "Consistent Hashing and Random Trees" (STOC, 1997)**
   - 第10回から再掲。リバランス。
9. **Dean & Barroso, "The Tail at Scale" (CACM, 2013)**
   - **遅いタスクと、遅延の裾**。本文 §8.2。**第15回の、中心**。先に読んでおく。

## 発展

### 分散ファイルシステム、ストレージ
- **Shvachko et al., "The Hadoop Distributed File System" (MSST, 2010)**: HDFS。
- **Weil et al., "Ceph: A Scalable, High-Performance Distributed File System" (OSDI, 2006)**、**"CRUSH: Controlled, Scalable, Decentralized Placement of Replicated Data" (SC, 2006)**: メタデータサーバーの、分散。配置の、アルゴリズム。
- **Muralidhar et al., "f4: Facebook's Warm BLOB Storage System" (OSDI, 2014)**: イレイジャーコーディングの、実運用。
- **Huang et al., "Erasure Coding in Windows Azure Storage" (USENIX ATC, 2012)**: ローカルな、パリティで、修復の読み出しを減らす(LRC)。本文 §3.3 の、「修復の読み出しが k 倍」への、対処。
- **Plank, "Erasure Codes for Storage Systems: A Brief Primer" (;login:, 2013)**、**Reed & Solomon, "Polynomial Codes over Certain Finite Fields" (1960)**: 符号の基礎。
- **Ford et al., "Availability in Globally Distributed Storage Systems" (OSDI, 2010)**: **相関する故障の、実測**。本文 §3.3 の、「独立でない」の根拠。

### 一貫性とレプリケーション
- **Bailis et al., "Probabilistically Bounded Staleness for Practical Partial Quorums" (VLDB, 2012)**: **Dynamo 型の、古い読みの確率を、モデル化**。本文 §5.3 の、演習1 B の、より精密な扱い。
- **Cooper et al., "PNUTS: Yahoo!'s Hosted Data Serving Platform" (VLDB, 2008)**: 別の設計(行ごとの、タイムライン一貫性)。
- **Lloyd et al., "Don't Settle for Eventual: Scalable Causal Consistency for Wide-Area Storage with COPS" (SOSP, 2011)**: 因果一貫性。
- **Merkle, "A Digital Signature Based on a Conventional Encryption Function" (CRYPTO, 1987)**: Merkle 木の、原典。

### トランザクションと、分散データベース
- **Thomson et al., "Calvin: Fast Distributed Transactions for Partitioned Database Systems" (SIGMOD, 2012)**: 決定的な、順序づけによる、2PC の回避。
- **Taft et al., "CockroachDB: The Resilient Geo-Distributed SQL Database" (SIGMOD, 2020)**、**Huang et al., "TiDB: A Raft-based HTAP Database" (VLDB, 2020)**: Spanner に影響された、オープンソースの設計。
- **Cooper et al., "Benchmarking Cloud Serving Systems with YCSB" (SoCC, 2010)**: ベンチマークの、標準。

### ログ、ストリーム、バッチ
- **Zaharia et al., "Resilient Distributed Datasets: A Fault-Tolerant Abstraction for In-Memory Cluster Computing" (NSDI, 2012)**: Spark。
- **Akidau et al., "The Dataflow Model" (VLDB, 2015)**: ストリームとバッチの、統一。
- **Carbone et al., "Apache Flink: Stream and Batch Processing in a Single Engine" (IEEE Data Eng. Bull., 2015)**: exactly-once の状態管理。
- **Kleppmann & Kreps, "Kafka, Samza and the Unix Philosophy of Distributed Data" (IEEE Data Eng. Bull., 2015)**。

### 偏りと、遅いタスク
- **Zaharia et al., "Improving MapReduce Performance in Heterogeneous Environments" (OSDI, 2008)**: LATE。**投機的実行の、改良**(本文 §8.3 の、閾値の問題)。
- **Ananthanarayanan et al., "Reining in the Outliers in Map-Reduce Clusters using Mantri" (OSDI, 2010)**: **遅いタスクの、原因の、実測**(マシンの問題と、データの偏りの、両方)。本文 §8.3 の、「遅い原因が、データの偏りなら」の、実例。
- **Kwon et al., "A Study of Skew in MapReduce Applications" (2011)**、**SkewTune (SIGMOD, 2012)**: データの偏りの、対処。

## 読み方のコツ

1. **論文の「設計の目標と前提」を、最初に読む**。GFS は、「巨大なファイルの追記」を、前提にする。**前提が違えば、設計は、不適切**(小さなファイルが、大量にあるなら、GFS の単一マスターは、ボトルネック)。
2. **各システムが、「何を、何と、交換したか」**を、表にする(本文 §9)。**交換されたものを、自分のアプリの要件と、照らす**。
3. **論文の発表後の、変更を、確認する**(Spanner、Kafka、HDFS は、大きく変わっている)。
4. **性能の数値は、その論文の、ワークロードと、ハードウェアの数値**。**自分の環境では、違う**。
5. **障害の相関**(Ford et al.)を、意識する。**独立な故障を、仮定した、耐久性の数値は、楽観的**。
6. **シミュレーションは、傾向を示すが、実際の値ではない**。演習の数値は、モデルの結果である。
7. **この講義の、確かめられていない点**(本文 §10)を、論文と、実験で、確かめる。
