# 第12回 参考文献

> 文献は、著者・題名・発表の場・年で挙げる。**内容の要約は、概要であり、読むときに、原典で確認すること**。

## 教科書・読み物(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Kleppmann, *Designing Data-Intensive Applications* | 8章(分散システムの問題)、9章(一貫性と合意) | **最も読みやすい、実務者向けの導入**。部分的な故障、時計、一貫性モデルを、一冊で |
| van Steen & Tanenbaum, *Distributed Systems* (無料公開) | 時計、一貫性、故障耐性の章 | 標準的な教科書 |
| Attiya & Welch, *Distributed Computing: Fundamentals, Simulations, and Advanced Topics* | モデル、不可能性、一貫性 | **理論の、厳密な扱い** |
| Lynch, *Distributed Algorithms* | 非同期モデル、合意 | 理論の大著 |
| Cachin, Guerraoui, Rodrigues, *Introduction to Reliable and Secure Distributed Programming* | 故障検出、ブロードキャスト、合意 | 抽象を、段階的に積む |
| Beyer et al., *Site Reliability Engineering*(Google) | 分散合意、分散システムの監視 | 第15回でも使う |
| Jepsen の公開された分析(jepsen.io) | 各データベースの分析記事 | **実システムの、一貫性の違反の事例**。本文 §6.3 の、検査の実際 |

## 必読論文

1. **Lamport, "Time, Clocks, and the Ordering of Events in a Distributed System" (CACM, 1978)**
   - happened-before と Lamport 時計。分散システムの、出発点。**必ず、原典を読む**。
2. **Fidge, "Timestamps in Message-Passing Systems That Preserve the Partial Ordering" (1988)**、**Mattern, "Virtual Time and Global States of Distributed Systems" (1989)**
   - ベクタークロックの、それぞれ独立の提案。
3. **Fischer, Lynch, Paterson, "Impossibility of Distributed Consensus with One Faulty Process" (JACM, 1985)**
   - FLP の不可能性。本文 §5.2。主張の正確な形を、原典で確認する。
4. **Dwork, Lynch, Stockmeyer, "Consensus in the Presence of Partial Synchrony" (JACM, 1988)**
   - **部分同期**のモデル。Paxos や Raft の、理論的な足場。
5. **Chandra & Toueg, "Unreliable Failure Detectors for Reliable Distributed Systems" (JACM, 1996)**
   - 故障検出器の、完全性と正確性。本文 §4。
6. **Herlihy & Wing, "Linearizability: A Correctness Condition for Concurrent Objects" (TOPLAS, 1990)**
   - 線形化可能性の定義。演習4 の検査器の、基礎。
7. **Gilbert & Lynch, "Brewer's Conjecture and the Feasibility of Consistent, Available, Partition-Tolerant Web Services" (SIGACT News, 2002)**
   - CAP 定理の、証明と、用語の定義。本文 §7.1 の、「正確な主張」の、出典。
8. **Corbett et al., "Spanner: Google's Globally-Distributed Database" (OSDI, 2012)**
   - TrueTime と、外部整合性。本文 §3.8。
9. **DeCandia et al., "Dynamo: Amazon's Highly Available Key-value Store" (SOSP, 2007)**
   - 結果整合性、ベクタークロック、競合の保持、一貫性ハッシュ(第10回)、クォーラム。第14回でも使う。

## 発展

### 時間
- **Kulkarni, Demirbas, Madappa, Avva, Leone, "Logical Physical Clocks" (OPODIS, 2014)**: HLC。本文 §3.7。
- **Liskov, "Practical Uses of Synchronized Clocks in Distributed Systems" (PODC, 1991)**: 時計を、安全に使う方法。リース。
- **Gray & Cheriton, "Leases: An Efficient Fault-Tolerant Mechanism for Distributed File Cache Consistency" (SOSP, 1989)**: 時計を使った、期限つきの権利。
- **Kleppmann の、分散ロックに関する、公開された議論**(「ロックの有効期限と、時計」): 時計に頼る危険。
- **Brewer, "Spanner, TrueTime and the CAP Theorem" (2017)**: TrueTime が、CAP を、どう扱うか。

### 故障検出
- **Hayashibara et al., "The φ Accrual Failure Detector" (SRDS, 2004)**: 演習3 の、φ 検出器。**原典の主張と、演習3 の結果の、違いを、考える**(本文 §8)。
- **Das, Gupta, Motivala, "SWIM: Scalable Weakly-consistent Infection-style Process Group Membership Protocol" (DSN, 2002)**: 大規模な、故障検出。

### 一貫性
- **Terry et al., "Session Guarantees for Weakly Consistent Replicated Data" (PDIS, 1994)**: セッション保証。
- **Lamport, "How to Make a Multiprocessor Computer That Correctly Executes Multiprocess Programs" (IEEE Trans. Computers, 1979)**: 逐次一貫性。
- **Viotti & Vukolić, "Consistency in Non-Transactional Distributed Storage Systems" (ACM Computing Surveys, 2016)**: 一貫性モデルの、体系的な整理。
- **Bailis et al., "Highly Available Transactions: Virtues and Limitations" (VLDB, 2014)**: 可用性と、トランザクションの、保証の関係。
- **Shapiro et al., "Conflict-Free Replicated Data Types" (SSS, 2011)**: CRDT。本文 §9 の論点 3。
- **Hellerstein & Alvaro, "Keeping CALM: When Distributed Consistency is Easy" (CACM, 2020)**: CALM 定理の解説。
- **Wing & Gong, "Testing and Verifying Concurrent Objects" (JPDC, 1993)**: 演習4 の、検査の方法。
- **Kingsbury & Alvaro, "Elle: Inferring Isolation Anomalies from Experimental Observations" (VLDB, 2020)**: 実システムの、履歴の検査の、実用的な手法。

### CAP
- **Brewer, "CAP Twelve Years Later: How the 'Rules' Have Changed" (IEEE Computer, 2012)**: 提唱者自身の、見直し。
- **Abadi, "Consistency Tradeoffs in Modern Distributed Database System Design" (IEEE Computer, 2012)**: PACELC。本文 §7.4。
- **Kleppmann, "A Critique of the CAP Theorem" (2015)**: CAP の用語の、曖昧さへの批判。本文 §7.2 の、誤解の背景。
- **Fox & Brewer, "Harvest, Yield, and Scalable Tolerant Systems" (HotOS, 1999)**: 可用性の、より細かい捉え方。

### 一般
- **Deutsch ら, "Fallacies of Distributed Computing"**: 本文 §1.2。
- **Lamport, "The Part-Time Parliament" (TOCS, 1998)**、**"Paxos Made Simple" (2001)**: 第13回の予習。
- **Lampson, "Hints for Computer System Design" (SOSP, 1983)**: 設計の知恵。

## 読み方のコツ

1. **モデルの仮定**(同期か非同期か、故障の種類、通信の信頼性)を、**最初に**確認する。同じ主張も、モデルが違えば、成り立たない。
2. **定理の主張を、正確に言い直す**練習をする。FLP と CAP は、**最も、誤解される**定理である。「〜は不可能」の、**何が**、**どのモデルで**、不可能なのか。
3. **「一貫性」という言葉**が、どの意味か(線形化可能性、ACID の C、結果整合性)を、確認する。
4. **実システムの論文**(Spanner、Dynamo)は、**自分たちの環境の、前提**(GPS、データセンター内の、低い遅延)に、依存している。**前提を、取り出して**読む。
5. **シミュレーションは、傾向を示すが、実際の値ではない**。演習の数値は、モデルの結果である。
6. この講義の、**確かめられていない仮説**(本文 §8)を、論文や、実験で、確かめる。
