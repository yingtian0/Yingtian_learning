# 第15回 参考文献

> 文献は、著者・題名・発表の場・年で挙げる。**内容の要約は、概要であり、読むときに、原典で確認すること**。

## 教科書・読み物(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Beyer, Jones, Petoff, Murphy (編), *Site Reliability Engineering*(Google、無料公開) | SLO、エラーバジェット、「過負荷への対処」「連鎖した障害への対処」「ポストモーテム」の章 | **この回の、中心的な読み物**。本文 §3、§6、§7 の元 |
| Beyer, Murphy, Rensin, Kawahara, Thorne (編), *The Site Reliability Workbook*(Google、無料公開) | 「SLO の実装」「アラート」の章 | **多窓・多バーンレートの警報**(本文 §3.4)の、元 |
| Kleppmann, *Designing Data-Intensive Applications* | 8章(分散システムの、問題)、全体の総括 | 全 15 回の総括として |
| Gregg, *Systems Performance*(2nd) | 方法論、待ち行列の章 | 測り方 |
| Kurose & Ross、Tanenbaum & Bos など(各回の、教科書) | 各回の、一覧 | 復習 |
| *Release It!*(Nygard) | 安定性のパターン(サーキットブレーカー、バルクヘッド、タイムアウト) | 実務のパターン集 |
| Amazon Builders' Library の公開記事(「タイムアウト、再試行、ジッターつきバックオフ」「過負荷の回避」など) | 該当する記事 | **再試行と過負荷の、実務の知見**。本文 §6 |

## 必読論文

1. **Dean & Barroso, "The Tail at Scale" (CACM, 2013)**
   - 遅延の裾、ヘッジされたリクエスト、タイド・リクエスト。本文 §4。
2. **Bronson, Aghayev, Charapko, Zhu, "Metastable Failures in Distributed Systems" (HotOS, 2021)**、**Huang et al., "Metastable Failures in the Wild" (OSDI, 2022)**
   - **準安定な障害**。本文 §6.2。実際の障害の分析。
3. **Little, "A Proof for the Queuing Formula: L = λW" (Operations Research, 1961)**
   - リトルの法則。本文 §2.2、§5.2。
4. **Nichols & Jacobson, "Controlling Queue Delay" (ACM Queue, 2012)**
   - CoDel。待ち時間に基づく、破棄。本文 §6.3 の、方針 C の、洗練された形。
5. **Lamport, "Time, Clocks, and the Ordering of Events in a Distributed System" (CACM, 1978)**、**Fischer, Lynch, Paterson (1985)**、**Gilbert & Lynch (2002)**
   - 第12回から再掲。**不可能性**を、設計の、前提にする。
6. **Hamilton, "On Designing and Deploying Internet-Scale Services" (LISA, 2007)**
   - 大規模サービスの、運用の知恵。チェックリスト。
7. **Brewer, "Lessons from Giant-Scale Services" (IEEE Internet Computing, 2001)**
   - 収穫(harvest)と収量(yield)、縮退。
8. **Cook, "How Complex Systems Fail" (1998)**
   - 複雑なシステムの、障害の性質。短い。必ず読む。
9. **Allspaw, "Blameless PostMortems and a Just Culture" (2012)**
   - 非難しない事後分析。

## 発展

### 待ち行列と性能
- **Harchol-Balter, *Performance Modeling and Design of Computer Systems: Queueing Theory in Action***: 待ち行列理論の、実務者向けの、教科書。
- **Kingman, "The Single Server Queue in Heavy Traffic" (1961)**: 使用率と、待ち時間の、近似。ばらつきの影響。
- **Mitzenmacher, "The Power of Two Choices in Randomized Load Balancing" (IEEE TPDS, 2001)**: 第10回から再掲。

### 過負荷と、安定性
- **Welsh, Culler, "Adaptive Overload Control for Busy Internet Servers" (USITS, 2003)**: 適応的な過負荷制御。
- **Welsh, Culler, Brewer, "SEDA: An Architecture for Well-Conditioned, Scalable Internet Services" (SOSP, 2001)**: 段階的な、イベント駆動の設計、過負荷時の、グレースフルな劣化。
- **Maurer, "Fail at Scale" (ACM Queue, 2015)**: 大規模サービスの、障害の傾向と、対策(適応的 LIFO、制御された遅延、など)。
- **Floyd & Jacobson, "Random Early Detection Gateways for Congestion Avoidance" (1993)**: ネットワークでの、早期の破棄。第8回との接続。

### 信頼性とレジリエンス
- **Basiri et al., "Chaos Engineering" (IEEE Software, 2016)**: カオスエンジニアリング。
- **Alvaro et al., "Lineage-driven Fault Injection" (SIGMOD, 2015)**: 体系的な、障害の注入。
- **Gunawi et al., "Why Does the Cloud Stop Computing? Lessons from Hundreds of Service Outages" (SoCC, 2016)**: **多数の、公開された障害の、分析**。本文 §7.3 の、パターンの、根拠になりうる。
- **Yuan et al., "Simple Testing Can Prevent Most Critical Failures" (OSDI, 2014)**: 障害の多くは、エラー処理の、誤りに由来する。
- **Newcombe et al., "How Amazon Web Services Uses Formal Methods" (CACM, 2015)**: 形式手法の、実務。第13回から再掲。

### 事後報告の、一次資料(読み方の練習)
- **Dyn の障害(2016)**、**Facebook の障害(2021)**、**Cloudflare の障害(2019)の、各社の、公開された事後報告**: 第10回から再掲。**原因、検出の遅れ、復旧の手順、再発防止、の順に読む**。本講義は、**これらの、詳細を、検証していない**。

## 読み方のコツ

1. **「何と何を、交換したか」**を、毎回、書き出す。**15 回を通じた、問い**。
2. **平均ではなく、裾**を見る。**p99、p99.9、最大**。
3. **確かめられていないことを、確かめられていない、と書く**。本講義の、各回の「つまずいた点」の書き方を、自分の設計文書にも、使う。
4. **事後報告は、一次資料として、最良の教材**。**原因、検出、緩和、復旧、再発防止**の順に読み、**自分のシステムなら、どうなるか**を、考える。
5. **モデル(シミュレーション)は、構造を理解するための、道具**。実際の値は、測る。
6. **この講義の、確かめられていない点**(各回の本文の、「つまずいた点」)を、**論文と実験で、確かめる**。これが、次の一歩。
