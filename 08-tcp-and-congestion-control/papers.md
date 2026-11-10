# 第8回 参考文献

## 教科書・読み物(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Kurose & Ross, *Computer Networking: A Top-Down Approach* | 3章(トランスポート層: 信頼性のあるデータ転送の原理、TCP、輻輳制御) | 本回の標準的な教科書。信頼性の原理(停止待ち、スライディングウィンドウ)から、段階的に TCP へ |
| Stevens, *TCP/IP Illustrated, Vol. 1*(2nd) | TCP の章(接続の確立と終了、タイムアウトと再送、高速再送、輻輳制御) | **実際のパケットの動き**を、時系列で読める。TIME_WAIT、Nagle、遅延 ACK の記述が詳しい |
| Peterson & Davie, *Computer Networks: A Systems Approach*(無料公開) | 5〜6章(エンドツーエンドのプロトコル、輻輳制御と資源の配分) | 設計の理由に重点がある |
| RFC 9293(TCP)、RFC 5681(輻輳制御)、RFC 6298(RTO の計算)、RFC 6928(初期ウィンドウ 10)、RFC 2018(SACK)、RFC 7323(ウィンドウ拡大、タイムスタンプ) | 該当部 | **一次資料**。RFC 9293 は、TCP の仕様を、2022 年に統合・更新したもの |
| RFC 8312(CUBIC)、RFC 9000(QUIC) | 該当部 | CUBIC の式、QUIC の設計 |

## 必読論文

1. **Jacobson, "Congestion Avoidance and Control" (SIGCOMM, 1988)**
   - スロースタートと、輻輳回避、RTT の推定。**現代の TCP の、ほぼ全ての出発点**。1980 年代の、輻輳崩壊の記述も含む。必読。
2. **Chiu & Jain, "Analysis of the Increase and Decrease Algorithms for Congestion Avoidance in Computer Networks" (Computer Networks and ISDN Systems, 1989)**
   - AIMD が、公平と効率に収束する理由。ベクトル図で、直観的に分かる。
3. **Mathis, Semke, Mahdavi, Ott, "The Macroscopic Behavior of the TCP Congestion Avoidance Algorithm" (ACM CCR, 1997)**
   - 本文 §6.5 の式の出典。短く、読みやすい。
4. **Cardwell, Cheng, Gunn, Yeganeh, Jacobson, "BBR: Congestion-Based Congestion Control" (ACM Queue, 2016)**
   - BBR の設計と動機。損失ベースの方式の限界(バッファブロート、ランダムな損失への弱さ)の、整理が明快。
5. **Ha, Rhee, Xu, "CUBIC: A New TCP-Friendly High-Speed TCP Variant" (ACM SIGOPS Operating Systems Review, 2008)**
   - CUBIC の設計。3 次関数での成長と、TCP フレンドリーな領域。

## 発展

### 輻輳制御の理論と評価
- **Hock, Bless, Zitterbart, "Experimental Evaluation of BBR Congestion Control" (ICNP, 2017)** と **Ware, Mukerjee, Seshan, Sherry, "Modeling BBR's Interactions with Loss-Based Congestion Control" (IMC, 2019)**: BBR と、Reno / CUBIC の共存の実験とモデル。本文 §6.8 の、シミュレータの注意の、根拠。
- **Padhye, Firoiu, Towsley, Kurose, "Modeling TCP Throughput: A Simple Model and its Empirical Validation" (SIGCOMM, 1998)**: タイムアウトも含む、より精密なスループットのモデル。
- **Briscoe, "Flow Rate Fairness: Dismantling a Religion" (ACM CCR, 2007)**: フローごとの速度の公平、という前提への批判。
- **Winstein & Balakrishnan, "TCP ex Machina: Computer-Generated Congestion Control" (SIGCOMM, 2013)**(Remy): 輻輳制御のアルゴリズムを、最適化で生成する。
- **Dong et al., "PCC: Re-architecting Congestion Control for Consistent High Performance" (NSDI, 2015)** と **"PCC Vivace" (NSDI, 2018)**: オンラインの学習。

### バッファと遅延
- **Appenzeller, Keslassy, McKeown, "Sizing Router Buffers" (SIGCOMM, 2004)**: 多数のフローが通るルーターでは、バッファは BDP ÷ √(フロー数) で足りる。
- **Gettys & Nichols, "Bufferbloat: Dark Buffers in the Internet" (ACM Queue, 2011)**: バッファブロートの、問題提起。
- **Nichols & Jacobson, "Controlling Queue Delay" (ACM Queue, 2012)**: CoDel。

### データセンター
- **Alizadeh et al., "Data Center TCP (DCTCP)" (SIGCOMM, 2010)**: ECN を使い、輻輳の程度に比例して縮める。
- **Montazeri, Li, Alizadeh, Ousterhout, "Homa: A Receiver-Driven Low-Latency Transport Protocol Using Network Priorities" (SIGCOMM, 2018)**: 受信側が主導する、データセンター向けのトランスポート。

### QUIC とその先
- **Langley et al., "The QUIC Transport Protocol: Design and Internet-Scale Deployment" (SIGCOMM, 2017)**: QUIC の設計と、Google での展開の経験。
- **RFC 8684(Multipath TCP)**: 複数経路を同時に使う TCP。

### 運用
- **Nagle, "Congestion Control in IP/TCP Internetworks" (RFC 896, 1984)**: Nagle のアルゴリズムの原典。
- **Stevens らによる TCP の解説(RFC 1122 の、遅延 ACK の記述)**。
- Linux の、`Documentation/networking/ip-sysctl.rst`: `tcp_*` の設定項目の、一次資料。

## 読み方のコツ

1. 輻輳制御の論文は、**どのネットワーク条件**(帯域、RTT、バッファの深さ、損失の種類、フローの数)で評価したかを、最初に確認する。条件が違うと、結論が逆になる(BBR の共存が、典型)。
2. 性能の数値は、**シミュレーションか実機か**を区別する。本講義の演習4は、シミュレーションで、定性的な傾向の確認のためのものである。
3. 「**公平**」の定義を、必ず確認する(フローごとの速度か、利用者ごとか、遅延か)。
4. TCP の挙動は、**OS とバージョンで、大きく違う**。実機の測定は、カーネルのバージョンと、輻輳制御の設定(`sysctl`)を、必ず記録する。
