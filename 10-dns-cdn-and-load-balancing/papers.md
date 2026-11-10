# 第10回 参考文献

## 教科書・読み物(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Kurose & Ross, *Computer Networking: A Top-Down Approach* | 2章(DNS、CDN)、5章(BGP、AS 間ルーティング) | DNS の階層と、BGP の基本の、標準的な説明 |
| Peterson & Davie, *Computer Networks: A Systems Approach*(無料公開) | 3章(インターネットワーキング、BGP)、9章(アプリケーション: DNS、CDN) | 設計の理由に重点がある |
| Liu & Albitz, *DNS and BIND* | 全体(特に、ゾーン、委任、キャッシュ) | DNS の運用の古典 |
| Gregg, *Systems Performance*(2nd) | ネットワークの章 | 実務の観測 |
| Beyer et al., *Site Reliability Engineering*(Google) | 「負荷の分散」の章(データセンター内、フロントエンド)、「過負荷への対処」の章、「連鎖した障害への対処」の章 | **実際の大規模システムでの、ロードバランシングと、障害の連鎖**。第15回でも使う |
| RFC 1034、RFC 1035(DNS の概念と仕様) | 全体 | **一次資料**。短く、読みやすい。メッセージの形式は、演習1の元 |
| RFC 4271(BGP-4)、RFC 7454(BGP の運用のセキュリティ)、RFC 6811(ROV)、RFC 6480(RPKI) | 該当部 | BGP と RPKI の一次資料 |
| RFC 2181(DNS の明確化)、RFC 2308(否定キャッシュ)、RFC 8914 など | 該当部 | キャッシュと TTL の、正確な意味 |

## 必読論文

1. **Mockapetris, "Domain Names — Concepts and Facilities" (RFC 1034, 1987)** と **"— Implementation and Specification" (RFC 1035, 1987)**
   - DNS の設計。**階層と委任**で、集中管理を避け、世界規模に拡張する、という設計の動機が分かる。
2. **Gao & Rexford, "Stable Internet Routing Without Global Coordination" (IEEE/ACM Transactions on Networking, 2001)**
   - 本文 §5.2 の、顧客/ピア/プロバイダの経済的な関係と、BGP が、**全体の調整なしに、安定する**ための条件。
3. **Mitzenmacher, "The Power of Two Choices in Randomized Load Balancing" (IEEE TPDS, 2001)**
   - 本文 §3.3 の P2C。ランダムに 2 つ選ぶだけで、最大の負荷が、指数的に小さくなる。
4. **Karger et al., "Consistent Hashing and Random Trees: Distributed Caching Protocols for Relieving Hot Spots on the World Wide Web" (STOC, 1997)**
   - 一貫性ハッシュの原論文。CDN の元になった、Akamai の創業者らによる。
5. **Eisenbud et al., "Maglev: A Fast and Reliable Software Network Load Balancer" (NSDI, 2016)**
   - Google の、ソフトウェアによる L4 ロードバランサ。ECMP と、一貫性ハッシュの組み合わせ。

## 発展

### DNS
- **Ager et al., "Comparing DNS Resolvers in the Wild" (IMC, 2010)** と、近年の公開リゾルバの測定研究: リゾルバの性能と、TTL の扱い。
- **Moura et al., "Cache Me If You Can: Effects of DNS Time-to-Live" (IMC, 2019)**: TTL の設定が、実際の遅延と負荷に、どう影響するか。本文 §2.5 の、実測的な根拠。
- **Arends et al., "DNS Security Introduction and Requirements" (RFC 4033, 2005)**: DNSSEC。
- **Hu et al., "Specification for DNS over Transport Layer Security (DoT)" (RFC 7858)**、**Hoffman & McManus, "DNS Queries over HTTPS (DoH)" (RFC 8484)**。
- **Dyn の障害(2016)の、公開された事後報告と、分析**: DNS プロバイダへの依存の危険。

### ロードバランシングと CDN
- **Dean & Barroso, "The Tail at Scale" (CACM, 2013)**: 遅延の裾(第15回)と、ヘッジされたリクエスト。
- **Nygren, Sitaraman, Sun, "The Akamai Network: A Platform for High-Performance Internet Applications" (ACM SIGOPS OSR, 2010)**: 大規模な CDN の、設計と運用。
- **Einziger, Friedman, Manes, "TinyLFU: A Highly Efficient Cache Admission Policy" (ACM Transactions on Storage, 2017)**: LRU と LFU の中間。本文 §4.2 の、実際の方式。
- **Berger et al., "Practical Bounds on Optimal Caching with Variable Object Sizes" (SIGMETRICS, 2018)**: 物件の大きさが、ばらつくときの、最適なキャッシュの上限。
- **Breslau et al., "Web Caching and Zipf-like Distributions: Evidence and Implications" (INFOCOM, 1999)**: ウェブのアクセスが、Zipf 的であることの、実証。本文 §4.2 の元。
- **Appenzeller 他、Facebook の Katran、Envoy のドキュメント**: 実際のロードバランサの設計。

### BGP とルーティングのセキュリティ
- **Ballani, Francis, Zhang, "A Study of Prefix Hijacking and Interception in the Internet" (SIGCOMM, 2007)**: 乗っ取りの影響の、実測と分析。
- **Gilad et al., "Origin Validation and RPKI deployment" 系の測定研究(NDSS など)**: ROV の普及と、効果。
- **Labovitz et al., "Internet Inter-Domain Traffic" (SIGCOMM, 2010)**: インターネットの、トラフィックの集中の測定。本文 §9 の論点 8 の背景。
- **Facebook の障害(2021)の、公開された事後報告(Facebook Engineering のブログ)**、**Cloudflare による、同障害の技術的な解説**。
- **YouTube の乗っ取り(2008)の、RIPE NCC による、公開された分析**。

### エニーキャスト
- **Li et al., "Internet Anycast: Performance, Problems, and Potential" (SIGCOMM, 2018)**: エニーキャストの、実際の性能と、問題。
- **Schlinker et al., "Engineering Egress with Edge Fabric" (SIGCOMM, 2017)**、**"Internet Performance from Facebook's Edge" (IMC, 2019)**: 大手の、エッジの運用の、実際。

## 読み方のコツ

1. DNS の測定研究は、**測定した時期**(年)を確認する。リゾルバの挙動(TTL の扱い、DoH の普及)は、年々変わる。
2. BGP の論文は、**トポロジーの前提**(実際のインターネットの AS の関係のデータを使ったか、合成か)を確認する。本講義の演習4は、13 個の AS の、手で作った簡易モデルである。
3. ロードバランシングやキャッシュの論文は、**負荷の仮定**(サービス時間の分布、アクセスの分布、人気の時間変化)を確認する。仮定が違えば、優劣は逆転する(演習2 の、指数分布と、裾の重い分布)。
4. 実際の障害の事後報告(post-mortem)は、**一次資料として、最良の教材**である。原因、検出の遅れ、復旧の手順、再発防止、の順に読む。
5. **シミュレーションは、傾向を示すが、実際の値ではない**。演習2、4、5 の数値は、モデルの結果である。
