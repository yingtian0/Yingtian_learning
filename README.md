# 講義全体シラバス

目標: OS・ネットワーク・分散システムを博士レベルで理解する。演習言語は Go + C。

| # | フォルダ | テーマ |
|---|---|---|
| 01 | [01-computer-architecture](01-computer-architecture/README.md) | コンピュータアーキテクチャ |
| 02 | [02-memory-management](02-memory-management/README.md) | メモリ管理 |
| 03 | [03-processes-and-scheduling](03-processes-and-scheduling/README.md) | プロセスとスケジューリング |
| 04 | [04-concurrency-and-synchronization](04-concurrency-and-synchronization/README.md) | 並行性と同期 |
| 05 | [05-filesystems-and-storage](05-filesystems-and-storage/README.md) | ファイルシステムとストレージ |
| 06 | [06-kernel-and-io](06-kernel-and-io/README.md) | カーネルとI/O |
| 07 | [07-network-stack](07-network-stack/README.md) | ネットワークスタック |
| 08 | [08-tcp-and-congestion-control](08-tcp-and-congestion-control/README.md) | TCPと輻輳制御 |
| 09 | [09-tls-http-and-web](09-tls-http-and-web/README.md) | TLS・HTTP・Web |
| 10 | [10-dns-cdn-and-load-balancing](10-dns-cdn-and-load-balancing/README.md) | DNS・CDN・ロードバランシング |
| 11 | [11-runtimes-gc-and-containers](11-runtimes-gc-and-containers/README.md) | ランタイム・GC・コンテナ |
| 12 | [12-distributed-systems-theory](12-distributed-systems-theory/README.md) | 分散システムの理論 |
| 13 | [13-consensus-and-replication](13-consensus-and-replication/README.md) | 合意とレプリケーション |
| 14 | [14-large-scale-data-infrastructure](14-large-scale-data-infrastructure/README.md) | 大規模データ基盤 |
| 15 | [15-capstone-planet-scale-design-and-sre](15-capstone-planet-scale-design-and-sre/README.md) | 総合: プラネットスケール設計とSRE |

## 状況

全 15 回、執筆済み(第1版)。各回の構成は、`README.md`(講義)、`diagrams/`、`exercises/`、`papers.md`、`quiz.md`。

**この講義の読み方**

- 演習は、実行前に予想を書き、実行して比べ、外れたら理由を調べる。
- 数値は、この Mac(macOS、Apple シリコン)での実測、または、モデルのシミュレーションの結果。実システムの値ではない。
- 各回の本文末尾の「つまずいた点」に、確かめられなかったことを、書いてある。次の一歩は、それを、論文と実験で、確かめること。
- コンテナ(名前空間、cgroup)の演習は、Linux 専用で、未実行(第11回 `exercises/linux_only/`)。

**全体を貫く 5 つの原則**(第15回 §12): 何もただでは手に入らない / 平均は嘘をつく / 測らなければ分からない / 障害は起きる / システムは部品の和ではない。
