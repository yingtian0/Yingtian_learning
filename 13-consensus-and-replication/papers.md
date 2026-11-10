# 第13回 参考文献

> 文献は、著者・題名・発表の場・年で挙げる。**内容の要約は、概要であり、読むときに、原典で確認すること**。

## 教科書・読み物(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Kleppmann, *Designing Data-Intensive Applications* | 5章(レプリケーション)、7章(トランザクション)、9章(一貫性と合意) | 実務者向けの、最良の導入 |
| Ongaro, *Consensus: Bridging Theory and Practice*(博士論文、無料公開) | 全体 | **Raft の、最も詳細な記述**。メンバーの変更、スナップショット、クライアントとの相互作用 |
| Raft の公式サイト(raft.github.io)と、可視化 | 全体 | 動く図で、挙動を確認できる |
| Attiya & Welch, *Distributed Computing* | 合意、複製の章 | 理論の厳密な扱い |
| Cachin, Guerraoui, Rodrigues, *Introduction to Reliable and Secure Distributed Programming* | 合意、ブロードキャスト | 抽象を、段階的に積む |
| Bernstein, Hadzilacos, Goodman, *Concurrency Control and Recovery in Database Systems*(無料公開) | コミットプロトコルの章 | 2PC の古典的な扱い |
| Jepsen の公開された分析(jepsen.io) | etcd、Consul、各データベース | **実装の、一貫性の違反の事例** |

## 必読論文

1. **Lamport, "The Part-Time Parliament" (ACM TOCS, 1998)**、**"Paxos Made Simple" (ACM SIGACT News, 2001)**
   - Paxos の原典と、平易な説明。本文 §2。後者から読む。
2. **Ongaro & Ousterhout, "In Search of an Understandable Consensus Algorithm" (USENIX ATC, 2014)**
   - Raft。本文 §3 の元。**Figure 2(規則の要約)と、Figure 8(commit の落とし穴)**を、必ず、読む。
3. **Schneider, "Implementing Fault-Tolerant Services Using the State Machine Approach: A Tutorial" (ACM Computing Surveys, 1990)**
   - 複製状態機械。本文 §1.2。
4. **Oki & Liskov, "Viewstamped Replication: A New Primary Copy Method to Support Highly-Available Distributed Systems" (PODC, 1988)**、**Liskov & Cowling, "Viewstamped Replication Revisited" (2012)**
   - Paxos と同時期の、独立な発見。Raft に近い。
5. **Chandra, Griesemer, Redstone, "Paxos Made Live: An Engineering Perspective" (PODC, 2007)**
   - **Paxos を、実際の(Chubby の)システムに実装したときの、困難**。本文 §10 の「実装の、細部のバグ」の、実例。
6. **van Renesse & Schneider, "Chain Replication for Supporting High Throughput and Availability" (OSDI, 2004)**
   - チェーン複製。本文 §6。
7. **Gray, "Notes on Data Base Operating Systems" (1978)**、**Gray & Lamport, "Consensus on Transaction Commit" (ACM TODS, 2006)**
   - 2PC と、Paxos Commit。本文 §7。
8. **Shapiro, Preguiça, Baquero, Zawirski, "Conflict-Free Replicated Data Types" (SSS, 2011)**
   - CRDT。本文 §8。
9. **Castro & Liskov, "Practical Byzantine Fault Tolerance" (OSDI, 1999)**
   - PBFT。本文 §5。

## 発展

### Paxos の系統
- **Howard, Malkhi, Spiegelman, "Flexible Paxos: Quorum Intersection Revisited" (OPODIS, 2016)**: 本文 §4.1。
- **Moraru, Andersen, Kaminsky, "There Is More Consensus in Egalitarian Parliaments" (EPaxos, SOSP, 2013)**: リーダーレスの合意。本文 §11 の論点 3。
- **Lamport, "Fast Paxos" (Distributed Computing, 2006)**、**"Generalized Consensus and Paxos" (2005)**: 往復を減らす拡張。
- **Junqueira, Reed, Serafini, "Zab: High-performance broadcast for primary-backup systems" (DSN, 2011)**: ZooKeeper の、合意。
- **Burrows, "The Chubby Lock Service for Loosely-Coupled Distributed Systems" (OSDI, 2006)**: Paxos の、実用の例。

### 実装の検証
- **Wilcox et al., "Verdi: A Framework for Implementing and Formally Verifying Distributed Systems" (PLDI, 2015)**: Raft の、形式検証。
- **Hawblitzel et al., "IronFleet: Proving Practical Distributed Systems Correct" (SOSP, 2015)**: Paxos 系の、形式検証。
- **Lamport, *Specifying Systems*(TLA+)**、**Newcombe et al., "How Amazon Web Services Uses Formal Methods" (CACM, 2015)**: 実務での、形式手法。
- **Kingsbury の Jepsen、Zhou et al., "FoundationDB: A Distributed Unbundled Transactional Key Value Store" (SIGMOD, 2021)** の、決定的シミュレーションの節: **障害の注入による検証**。本文 §10 の、限界の議論。

### 複製とトランザクション
- **Terrace & Freedman, "Object Storage on CRAQ: High-throughput chain replication for read-mostly workloads" (USENIX ATC, 2009)**: チェーン複製の拡張。
- **Corbett et al., "Spanner: Google's Globally-Distributed Database" (OSDI, 2012)**: Paxos グループ + 2PC。第14回でも使う。
- **Skeen, "Nonblocking Commit Protocols" (SIGMOD, 1981)**: 3PC。
- **Bailis et al., "Coordination Avoidance in Database Systems" (VLDB, 2015)**: どこまで、調整を避けられるか。

### CRDT と、調整の最小化
- **Hellerstein & Alvaro, "Keeping CALM: When Distributed Consistency is Easy" (CACM, 2020)**: 本文 §8、§11 の論点 7。
- **Almeida, Shoker, Baquero, "Delta State Replicated Data Types" (JPDC, 2018)**: 状態の全体でなく、差分を送る。
- **Kleppmann & Beresford, "A Conflict-Free Replicated JSON Datatype" (IEEE TPDS, 2017)**: 共同編集。

### ビザンチン
- **Lamport, Shostak, Pease, "The Byzantine Generals Problem" (ACM TOPLAS, 1982)**: 原典。
- **Yin et al., "HotStuff: BFT Consensus with Linearity and Responsiveness" (PODC, 2019)**: メッセージの複雑さを、線形に。
- **Nakamoto, "Bitcoin: A Peer-to-Peer Electronic Cash System" (2008)**、**Garay, Kiayias, Leonardos, "The Bitcoin Backbone Protocol" (EUROCRYPT, 2015)**: 開かれた参加での、合意。

## 読み方のコツ

1. **アルゴリズムの、規則を、1 つずつ外して**、何が壊れるかを、考える(演習1、2 の、壊した版)。**各規則が、どの、危険を防いでいるか**が、分かる。
2. **Raft の論文は、Figure 2 を、そのまま、実装の仕様として読む**。**書かれていない部分**(永続化の順序、再起動時の状態)に、バグが、入りやすい。
3. **「実装が、論文と、同じ」かを、疑う**。Paxos Made Live は、論文と実システムの、ギャップの記録。
4. **テストの限界**: **ランダムな障害の注入で、違反が見つからない**ことは、**正しさの証明ではない**(演習2 の C1)。**形式手法**と**決定的なシミュレーション**を、組み合わせる。
5. **モデルの仮定**(同期、非同期、故障の種類、メッセージの喪失)を、最初に確認する(第12回)。
6. **シミュレーションは、傾向を示すが、実際の値ではない**。演習の数値は、モデルの結果である。
7. **この講義の、確かめられていない点**(本文 §10)を、論文と、実験で、確かめる。
