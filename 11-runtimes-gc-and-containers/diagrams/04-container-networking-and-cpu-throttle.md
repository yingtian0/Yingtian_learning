# 図4: コンテナのネットワークと、CPU スロットリング

> ネットワークの構成は、公開された仕様の知識による (未実行)。CPU スロットリングの数値は、演習5 のモデルの結果であり、実システムの値ではない。

## 1. ブリッジ + veth

```mermaid
flowchart LR
    subgraph HOST["ホスト"]
        subgraph NA["コンテナ A の NET 名前空間"]
            EA["eth0"]
        end
        subgraph NB["コンテナ B の NET 名前空間"]
            EB["eth0"]
        end
        VA["vethA"] --- BR["仮想ブリッジ (docker0 など)"]
        VB["vethB"] --- BR
        EA --- VA
        EB --- VB
        BR --> NAT["NAT (第7回) → 物理 NIC"]
    end
    NAT --> NET["外部ネットワーク"]
```

- コンテナ同士: ブリッジで、L2 で通信。
- 外へ: **NAT**。外から入る: ポートの公開 (DNAT)。

## 2. 複数ホストをまたぐ

| 方式 | 仕組み | 代償 |
|---|---|---|
| オーバーレイ (VXLAN など) | パケットを、別のパケットで包む | ヘッダ分の **MTU の減少** (第7回の MTU の黒穴)、カプセル化のコスト |
| ルーティング型 | ホストが、コンテナのサブネットを BGP で広告 (第10回) | ネットワーク機器の協力が要る |

## 3. Kubernetes の通信

```mermaid
flowchart LR
    U["外部"] --> ING["Ingress (L7 の入口)"]
    ING --> SVC["Service: ClusterIP (仮想 IP)"]
    SVC -->|"kube-proxy / eBPF が L4 で振り分け (第10回)"| P1["Pod 1"]
    SVC --> P2["Pod 2"]
    SVC --> P3["Pod 3"]
    DNS["CoreDNS: service.ns.svc.cluster.local → ClusterIP"] -.-> SVC
```

## 4. CFS 帯域制御 (`cpu.max = 200000 100000`)

```
 周期 100 ms

 8 スレッドが同時に走る場合 (8 コアのホスト):

 CPU を使う  ████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░
 時間        0                25 ms                                              100 ms
             └ 8 × 25 ms = 200 ms を使い切る ┘ └──── 残りの 75 ms は、全スレッドが停止 ────┘
```

「2 CPU」は、**周期あたりの、合計の量**であり、**瞬間**の話ではない。

## 5. シミュレーション (演習5: 上限 200 ms / 100 ms、1 件 = CPU 20 ms、T = 8)

| 平均の使用率 | p50 | p99 | スロットリングされた周期 |
|---|---|---|---|
| 30% | 20 ms | 20 ms | 0.1% |
| 50% | 20 | 22 | 1.9% |
| 70% | 20 | 55 | **14.6%** |
| 90% | 20 | **218** | **57.3%** |

- 平均が上限より低くても (70%)、**瞬間的な重なり**で、スロットリングが起きる。
- スレッド数 T を増やすと、スロットリングは増えたが、このモデルでは、p99 は T=2 より、同じか良かった (T=2 は、待ち行列で遅れる)。「スレッドが多いほど悪い」は、示せなかった。

## 6. Kubernetes の設定とカーネルの設定

| Kubernetes | カーネル | 超えると |
|---|---|---|
| `requests.cpu` | `cpu.weight` + スケジューリング | (取り合いのときの比率) |
| `limits.cpu` | `cpu.max` | スロットリング |
| `limits.memory` | `memory.max` | **OOM、exit 137** |

## 7. 終了の流れ

```mermaid
sequenceDiagram
    participant K as Kubernetes
    participant A as アプリ (PID 1)
    K->>A: SIGTERM
    Note over A: 新しい接続を止める。処理中のリクエストを終える
    alt 猶予 (既定 30 秒) の中で終了
        A-->>K: 正常終了
    else 終了しない
        K->>A: SIGKILL (強制)
    end
```

PID 1 は、`SIGTERM` の既定の処理が、通常のプロセスと違う。Go では、`signal.Notify` で、明示的に処理する。
