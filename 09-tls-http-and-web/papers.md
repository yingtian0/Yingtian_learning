# 第9回 参考文献

## 教科書・読み物(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Kurose & Ross, *Computer Networking: A Top-Down Approach* | 2章(アプリケーション層: HTTP、Web キャッシュ)、8章(ネットワークセキュリティ: 暗号、TLS) | HTTP の基本と、暗号の部品の整理 |
| Grigorik, *High Performance Browser Networking*(無料公開) | TLS、HTTP/1.x、HTTP/2、QUIC の章 | **ウェブの性能を、往復の数で見る**。本回の視点に近い。HTTP/3 以前の記述だが、基礎は有効 |
| Ristić, *Bulletproof TLS and PKI* | TLS の章、PKI の章 | 実務での設定と、証明書の運用 |
| Ferguson, Schneier, Kohno, *Cryptography Engineering* | 前半 | 暗号の部品を、実装者の視点で |
| RFC 8446(TLS 1.3) | 全体(特に、ハンドシェイクの章) | **一次資料**。図が分かりやすい |
| RFC 9110(HTTP の意味)、RFC 9112(HTTP/1.1)、RFC 9113(HTTP/2)、RFC 9114(HTTP/3) | 該当部 | 2022 年に整理された、現在の HTTP の規格 |
| RFC 7541(HPACK)、RFC 9204(QPACK) | 付録の例 | ヘッダ圧縮の、具体的なバイト列の例 |
| RFC 9000、9001、9002(QUIC、QUIC の TLS、QUIC の損失検出) | 該当部 | QUIC の設計 |
| RFC 5280(X.509 証明書)、RFC 6962 / RFC 9162(Certificate Transparency)、RFC 8555(ACME)、RFC 6797(HSTS) | 該当部 | PKI の一次資料 |

## 必読論文

1. **Diffie & Hellman, "New Directions in Cryptography" (IEEE Trans. Information Theory, 1976)**
   - 鍵交換の原論文。共通の秘密を、盗聴者に知られずに作れる、という発想。
2. **Rescorla, "The Transport Layer Security (TLS) Protocol Version 1.3" (RFC 8446, 2018)**
   - TLS 1.3 の仕様。**§2(プロトコルの概観)**と、§4(ハンドシェイク)を読む。
3. **Cremers, Horvat, Hoyland, Scott, van der Merwe, "A Comprehensive Symbolic Analysis of TLS 1.3" (CCS, 2017)**
   - TLS 1.3 の、形式的な検証(Tamarin)。プロトコルの設計を、数学的に検証する営みの例。
4. **Durumeric et al., "The Matter of Heartbleed" (IMC, 2014)**
   - 実装のバグ(Heartbleed)の、インターネット全体への影響の実測。パッチの適用の遅さ、証明書の再発行の少なさ。
5. **Kettle, "HTTP Desync Attacks: Request Smuggling Reborn" (Black Hat USA, 2019)**
   - 本文 §10 の、スマグリングの現代的な研究。原理と、実際のシステムでの影響。

## 発展

### TLS と暗号
- **Bhargavan et al., "Verified Models and Reference Implementations for the TLS 1.3 Standard Candidate" (IEEE S&P, 2017)**: もう1つの、TLS 1.3 の形式的な検証。
- **Langley, Hamburg, Turner, "Elliptic Curves for Security" (RFC 7748, 2016)**: X25519 の仕様。
- **Böck, Somorovsky, Young, "Return Of Bleichenbacher's Oracle Threat (ROBOT)" (USENIX Security, 2018)**: RSA 鍵交換が、長く攻撃され続ける理由。
- **Kelsey, "Compression and Information Leakage of Plaintext" (FSE, 2002)** と、CRIME(2012)、BREACH(2013)の発表: §9.4 の、圧縮と暗号の問題の、背景。

### PKI
- **Aas et al., "Let's Encrypt: An Automated Certificate Authority to Encrypt the Entire Web" (CCS, 2019)**: ACME と、自動化された証明書の運用の、設計と経験。
- **Larisch, Choffnes, Levin, Maggs, Mislove, Wilson, "CRLite: A Scalable System for Pushing All TLS Revocations to All Browsers" (IEEE S&P, 2017)**: 失効の問題への、1 つの答え。
- **Laurie, Langley, Kasper, "Certificate Transparency" (RFC 6962, 2013)**。

### HTTP
- **Belshe, Peon, Thomson, "Hypertext Transfer Protocol Version 2 (HTTP/2)" (RFC 7540, 2015)**(RFC 9113 に置き換えられた): HTTP/2 の設計の動機の章が、有用。
- **Langley et al., "The QUIC Transport Protocol: Design and Internet-Scale Deployment" (SIGCOMM, 2017)**: QUIC の設計と、Google での展開の、実測。第8回でも挙げた。
- **Linhart, Klein, Heled, Orrin, "HTTP Request Smuggling" (2005)**: スマグリングの最初の報告。
- Marx et al., "Same Standards, Different Decisions: A Study of QUIC and HTTP/3 Implementation Diversity" (EPIQ, 2020): 実装ごとの違い。

### ウェブの性能
- **web.dev**、**HTTP Archive の Web Almanac**: ウェブの実際のサイトの統計(HTTP/2 と HTTP/3 の採用率、ページの大きさ、リクエスト数)。毎年更新される。

## 読み方のコツ

1. セキュリティの論文は、**脅威モデル**(攻撃者が、何をできると仮定しているか)を、最初に確認する。同じプロトコルでも、受動的な盗聴者と、能動的な中間者では、守れる範囲が違う。
2. RFC は、**図と、メッセージの一覧**から読む。TLS 1.3 の RFC 8446 は、ハンドシェイクの図が、全体の理解の中心になる。
3. 性能の数値は、**RTT、帯域、損失、サーバーの負荷**の条件を確認する。本講義の演習2は、**帯域の制限と、実際の損失がない、往復だけが支配する環境**の結果である。
4. 暗号の「強さ」の議論では、**実装の質**(サイドチャネル、バグ)が、アルゴリズムの強さと、同じくらい重要であることを、忘れない(Heartbleed)。
5. 形式検証は、**検証した範囲**(プロトコルの設計のみか、実装も含むか)を、確認する。
