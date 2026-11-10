# 第7回 参考文献

## 教科書・読み物(該当章)

| 文献 | 読む範囲 | 読みどころ |
|---|---|---|
| Kurose & Ross, *Computer Networking: A Top-Down Approach* | 1章(概観、層構造、カプセル化)、4〜5章(ネットワーク層: IP、NAT、IPv6、ルーティング)、6章(リンク層: イーサネット、ARP、スイッチ) | 本回全体の、標準的な教科書。上から下へ、層を降りていく |
| Stevens, *TCP/IP Illustrated, Vol. 1*(2nd、Fall との共著) | IP、ICMP、ARP、UDP、断片化の章 | 実際のパケットの16進ダンプを読みながら進む。**パケットを読む力を付ける**には最良 |
| Peterson & Davie, *Computer Networks: A Systems Approach*(無料公開) | 1〜3章 | 設計の理由に重点がある。本回の「なぜ」に近い |
| Gregg, *Systems Performance*(2nd) | ネットワークの章 | 実務での観測の方法論 |
| RFC 791(IP)、RFC 792(ICMP)、RFC 768(UDP)、RFC 826(ARP) | 該当部 | **一次資料**。短く、読みやすい。IP と UDP は特に |
| RFC 1071(インターネットチェックサムの計算)、RFC 1624(差分更新) | 全体 | 演習1の元。実装のコツが書いてある |
| RFC 1191(経路 MTU 探索)、RFC 4821(パケット化層の PMTUD) | 該当部 | §3.8〜3.9 の元。ICMP に頼らない PMTUD |

## 必読論文

1. **Saltzer, Reed, Clark, "End-to-End Arguments in System Design" (ACM TOCS, 1984)**
   - 本回の設計原則。ネットワークだけでなく、システム設計全般に通じる。短く、必読。
2. **Clark, "The Design Philosophy of the DARPA Internet Protocols" (SIGCOMM, 1988)**
   - インターネットの設計の目標の、優先順位(生き残ること、多様なサービス、多様なネットワーク…)と、その帰結。なぜ IP がベストエフォートか。
3. **Mogul & Deering, "Path MTU Discovery" (RFC 1191, 1990)**
   - §3.8 の経路 MTU 探索の定義。
4. **Stone & Partridge, "When the CRC and TCP Checksum Disagree" (SIGCOMM, 2000)**
   - 実際のネットワークで、リンク層の CRC を通った後でも、TCP のチェックサムが誤りを検出することが、予想以上にあることを示した。チェックサムの弱さを、実測で見る。
5. **Honda, Nishida, Greenhalgh, Handley, Tokuda, "Is It Still Possible to Extend TCP?" (IMC, 2011)**
   - 実際のインターネットで、途中の機器(ミドルボックス)が、TCP の拡張を、どれだけ壊すか。硬直化(ossification)の実測。

## 発展

### 設計と進化
- **Beck, "On the Hourglass Model" (CACM, 2019)**: 砂時計モデルの、再検討。
- **Handley, "Why the Internet Only Just Works" (BT Technology Journal, 2006)**: インターネットが、なぜ変化しにくいか。
- Google の IPv6 採用の統計(公開データ)。IPv6 への移行が、どれだけ進んでいるかを、実測で見る。

### ルーティングの検索
- **Gupta, Lin, McKeown, "Routing Lookups in Hardware at Memory Access Speeds" (INFOCOM, 1998)**: DIR-24-8。演習3の `dir248` の元。
- **Degermark, Brodnik, Carlsson, Pink, "Small Forwarding Tables for Fast Routing Lookups" (SIGCOMM, 1997)**: キャッシュに載るほど小さな表。
- **Asai & Ohara, "Poptrie: A Compressed Trie with Population Count for Fast and Scalable Software IP Routing Table Lookup" (SIGCOMM, 2015)**: ソフトウェアでの、現代的な最長一致。
- **Waldvogel, Varghese, Turner, Plattner, "Scalable High Speed IP Routing Lookups" (SIGCOMM, 1997)**: プレフィックス長に対する二分探索。

### 高速なパケット処理
- **Kohler, Morris, Chen, Jannotti, Kaashoek, "The Click Modular Router" (ACM TOCS, 2000)**: ソフトウェアのルーターの構成。
- **Dobrescu et al., "RouteBricks: Exploiting Parallelism to Scale Software Routers" (SOSP, 2009)**: 汎用のサーバーで、ルーターを作る。
- **Bosshart et al., "P4: Programming Protocol-Independent Packet Processors" (ACM CCR, 2014)** と **Bosshart et al., "Forwarding Metamorphosis: Fast Programmable Match-Action Processing in Hardware for SDN" (SIGCOMM, 2013)**: プログラマブルなデータプレーン。
- **Rizzo, "netmap" (USENIX ATC, 2012)**、第6回の DPDK 関連。

### データセンターのネットワーク
- **Al-Fares, Loukissas, Vahdat, "A Scalable, Commodity Data Center Network Architecture" (SIGCOMM, 2008)**: fat-tree。第10回、第14回で再読する。

## 読み方のコツ

1. ネットワークの RFC は、**短い**ものが多い。**ヘッダの図**と、**フィールドの説明**だけでも、まず読む。
2. 論文の測定では、**いつ、どの経路で、どのネットワークで**測ったかを確認する。インターネットの状態は、年々変わる。
3. パケットを理解するには、**実際のパケットを読む**のが最善。`tcpdump -XX` や、Wireshark で、自分のマシンの通信を観察する(管理者権限が要る)。本講義の演習1は、その準備として、パケットを自分で組み立てる形にした。
4. 「速い」の議論では、**パケット数**(pps)と**バイト数**(bps)を区別する。ルーターの性能は、通常、pps で決まる。
