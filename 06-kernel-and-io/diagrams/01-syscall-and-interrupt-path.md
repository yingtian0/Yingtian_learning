# 図1: システムコールと割り込みの経路

## 1. システムコール1回の流れ

```mermaid
sequenceDiagram
    participant A as アプリ (ユーザーモード)
    participant L as C ライブラリ
    participant K as カーネル
    A->>L: read(fd, buf, n)
    L->>K: システムコール番号と引数をレジスタに入れて syscall / svc 命令
    Note over K: カーネルモードに切り替わる
    K->>K: 引数の検証 (buf はアプリ自身の領域か? 書き込める権限は?)
    K->>K: fd → 開いたファイル → inode をたどる
    K->>K: 処理 (ページキャッシュから copy_to_user)
    K-->>L: 結果を返してユーザーモードへ戻る
    L-->>A: 戻り値 (または errno)
```

固定のコスト(運ぶ量によらない)と、運ぶ量に比例するコスト(コピー)の和になる。

| 操作 | 実測(Apple M5) |
|---|---|
| 関数呼び出し | 約 0.5 ns |
| `clock_gettime`(カーネルに入らない) | 約 15 ns |
| `getppid` | 約 85〜88 ns |
| `read(/dev/zero, 1B)` | 約 241〜245 ns |

## 2. 1回で運ぶ量と、速度(演習1)

| バッファ | 速度 | 1回あたり |
|---|---|---|
| 1 B | 4.1 MB/s | 247 ns |
| 16 B | 64.5 MB/s | 248 ns |
| 256 B | 976.5 MB/s | 262 ns |
| 4 KiB | 9,492 MB/s | 432 ns |
| 64 KiB | 20,906 MB/s | 3,135 ns |
| 1 MiB | 22,064 MB/s | 47,523 ns |

1〜256 B では、1回あたりの時間がほぼ一定(約 250 ns)。**時間は、回数で決まっている**。1 B と 1 MiB で、速度は約 5,400 倍違う。

## 3. 割り込みの前半と後半

```mermaid
flowchart LR
    DEV["デバイスが CPU に割り込み"] --> TOP["前半 (top half)<br/>ごく短く、最小限のことだけ<br/>(状態の確認、後半の予約)"]
    TOP --> BOTTOM["後半 (bottom half)<br/>softirq / workqueue<br/>時間のかかる処理<br/>(プロトコルの処理など)"]
```

前半は、他の割り込みを受けにくい状態で動くので、短くする。

## 4. DMA とリングバッファ

```mermaid
flowchart TB
    subgraph メモリ
        R["受信リング<br/>[バッファ0][バッファ1][バッファ2]..."]
    end
    K["カーネル"] -->|"① 受信用バッファの場所 (記述子) をリングに並べる"| R
    NIC["ネットワークカード"] -->|"② パケットが届いたら DMA で、次のバッファに直接書き込む (CPU は使わない)"| R
    NIC -->|"③ 割り込みで知らせる"| K
```

## 5. ネットワーク受信の全経路

```mermaid
sequenceDiagram
    participant NIC as ネットワークカード
    participant RAM as リングバッファ (メモリ)
    participant K as カーネル
    participant S as ソケットの受信バッファ
    participant A as アプリ (read で眠っている)
    NIC->>RAM: DMA でパケットを書く
    NIC->>K: 割り込み
    K->>K: 前半 → 後半 (IP、TCP の処理、どのソケットのパケットか特定)
    K->>S: データを受信バッファへ
    K->>A: 眠っているアプリを実行可能にする
    Note over A: 実際に走るのは、スケジューラが選んだとき (第3回)
    A->>S: read() で受信バッファからアプリのバッファへコピー
```

## 6. 割り込みの嵐と、ポーリングへの切り替え

```mermaid
flowchart TB
    LOW["到着が少ない"] --> INT["割り込みで知る<br/>(CPU を無駄にしない)"]
    HIGH["到着が多い"] --> POLL["割り込みを止めて、<br/>CPU がデバイスを見に行く (ポーリング)<br/>= Linux の NAPI"]
    HIGH --> COAL["割り込みをまとめる (coalescing)<br/>レイテンシを少し犠牲にして、スループットを上げる"]
    LL["割り込みだけで処理し続けると"] --> LIVE["受信ライブロック:<br/>パケットの受け取りだけで CPU が埋まり、<br/>アプリが走れなくなる (Mogul & Ramakrishnan)"]
```
