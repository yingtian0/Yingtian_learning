# 図2: TCP の状態遷移と、TIME_WAIT

## 1. 状態遷移図

```mermaid
stateDiagram-v2
    [*] --> CLOSED
    CLOSED --> LISTEN: サーバー: listen()
    CLOSED --> SYN_SENT: クライアント: connect() / SYN を送る
    LISTEN --> SYN_RCVD: SYN を受け取る / SYN+ACK を送る
    SYN_SENT --> ESTABLISHED: SYN+ACK を受け取る / ACK を送る
    SYN_RCVD --> ESTABLISHED: ACK を受け取る
    ESTABLISHED --> FIN_WAIT_1: 先に close() / FIN を送る
    ESTABLISHED --> CLOSE_WAIT: FIN を受け取る / ACK を送る
    FIN_WAIT_1 --> FIN_WAIT_2: ACK を受け取る
    FIN_WAIT_2 --> TIME_WAIT: FIN を受け取る / ACK を送る
    CLOSE_WAIT --> LAST_ACK: アプリが close() / FIN を送る
    LAST_ACK --> CLOSED: ACK を受け取る
    TIME_WAIT --> CLOSED: 2 × MSL の経過
```

## 2. 閉じる手順(先に閉じる側と、後で閉じる側)

```mermaid
sequenceDiagram
    participant A as 先に閉じる側
    participant B as 後で閉じる側
    Note over A,B: ESTABLISHED
    A->>B: FIN (アプリが close した)
    Note over A: FIN_WAIT_1
    B->>A: ACK
    Note over A: FIN_WAIT_2
    Note over B: CLOSE_WAIT<br/>(自分のアプリが close するのを待つ)
    B->>A: FIN (アプリが close した)
    Note over B: LAST_ACK
    A->>B: ACK
    Note over A: TIME_WAIT (2 × MSL)
    Note over B: CLOSED
```

## 3. 状態が溜まるときの、原因

| 状態 | 溜まる原因 | 疑う場所 |
|---|---|---|
| **CLOSE_WAIT** | 相手は閉じたが、**自分のアプリが close していない** | **自分のアプリのバグ** (ソケットのリーク) |
| **TIME_WAIT** | **自分から閉じた**接続が、2 × MSL の間残る | 短い接続を、大量に張っている (接続プール、keep-alive) |
| FIN_WAIT_2 | 相手が、FIN を返さない | 相手のアプリが close していない (相手の CLOSE_WAIT) |
| SYN_RCVD | SYN フラッド、または accept が追いつかない | 攻撃、アプリの accept の遅れ |
| ESTABLISHED が多すぎる | 長寿命の接続が多い | 想定どおりか、リークか |

## 4. TIME_WAIT

```
 TIME_WAIT の長さ = 2 × MSL
   この環境 (macOS): net.inet.tcp.msl = 15,000 ms → 30 秒
   Linux: 通常 60 秒 (固定)
```

理由:

1. 最後の ACK が失われたとき、相手が FIN を再送してきたら、再び ACK を返せるようにする。
2. 古い接続の、遅れて届いたセグメントが、同じ 5 つ組の新しい接続に紛れ込むのを防ぐ。

## 5. 先に閉じた側にだけ、TIME_WAIT が残る(演習1)

```mermaid
flowchart TB
    subgraph S["サーバーが先に close した場合"]
        S1["サーバー側 (ポート 443 など) に TIME_WAIT が残る"] --> S2["SO_REUSEADDR なしでは、同じポートで再起動できない<br/>→ address already in use"]
    end
    subgraph C["クライアントが先に close した場合"]
        C1["クライアント側 (使い捨てのポート) に TIME_WAIT が残る"] --> C2["サーバーは、すぐ再起動できる"]
    end
```

| 先に閉じた側 | SO_REUSEADDR なしで再起動 | あり |
|---|---|---|
| サーバー | **失敗: address already in use** | bind できた |
| クライアント | bind できた | bind できた |

## 6. lsof で観察した状態(演習1、クライアントが先に close)

| 時点 | 状態 |
|---|---|
| listen した直後 | `LISTEN` |
| 接続の確立 | `ESTABLISHED` × 2 (サーバー側、クライアント側) |
| クライアントが close した後 | サーバー側が `CLOSE_WAIT` |
| サーバーも close した後 | `LISTEN` だけ |

`FIN_WAIT_2` と `TIME_WAIT` は、プロセスの管理から外れた(カーネルだけが持つ)ので、`lsof` には出ない。

## 7. エフェメラルポートの枯渇

```
 エフェメラルポートの範囲 (macOS の既定): 49152 〜 65535 = 約 16,000 個
 短い接続を、1 秒に 1,000 本、こちらから閉じて張り続けると:
   TIME_WAIT (30 秒) × 1,000 本/秒 = 30,000 個 > 16,000 個
   → 新しい接続が失敗する (can't assign requested address)  ← 第6回で実際に経験した
```

対策: 接続の使い回し、接続プール、`SO_LINGER`(0)で RST で閉じる(ただし、データを捨てる)、範囲の拡大。
