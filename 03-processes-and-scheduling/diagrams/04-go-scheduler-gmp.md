# 図4: Go のスケジューラ (G-M-P)

## 1. 登場人物

| 記号 | 名前 | 意味 |
|---|---|---|
| G | goroutine | 実行したい仕事。スタックは 2 KB から |
| M | machine | OS のスレッド。実際に命令を実行する |
| P | processor | 実行の権利。ローカルの実行可能キューを持つ。数は `GOMAXPROCS` |

**M が G を実行するには、P を持っている必要がある。**

## 2. 全体構造

```mermaid
flowchart TB
    subgraph P1["P1"]
        Q1["ローカルキュー<br/>G G G"]
    end
    subgraph P2["P2"]
        Q2["ローカルキュー<br/>G"]
    end
    subgraph P3["P3"]
        Q3["ローカルキュー<br/>(空)"]
    end
    GQ["グローバルキュー<br/>G G"]
    M1["M1 (OS スレッド)"] --- P1
    M2["M2 (OS スレッド)"] --- P2
    M3["M3 (OS スレッド)"] --- P3
    P3 -.->|"空なので<br/>他の P から半分を盗む"| Q1
    P3 -.->|"グローバルも確認"| GQ
    NP["ネットポーラ<br/>(I/O 待ちの goroutine)"] -->|"準備できたら戻す"| Q2
```

- 各 P のローカルキューは、ロックなしで素早く扱える。
- 空になった P は、他の P のキューから**半分を盗む**(work stealing)。
- ネットワーク I/O の待ちは、OS のスレッドを占有せず、ネットポーラ(epoll など。第6回)が管理する。

## 3. ブロッキングなシステムコール

```mermaid
sequenceDiagram
    participant G as goroutine G
    participant M1 as M1
    participant P as P
    participant M2 as M2 (別のスレッド)
    G->>M1: ブロッキングなシステムコールに入る
    Note over M1: M1 はカーネルの中で止まる
    P-->>P: 一定時間 (数十 µs) たっても戻らない
    P->>M2: ランタイムが P を M1 から取り上げて M2 に渡す
    M2->>P: 他の goroutine を実行し続ける
    M1->>M1: システムコールが戻る
    Note over M1: P がなければ、G を実行可能キューへ置き、M1 は眠る
```

## 4. プリエンプション

```mermaid
flowchart LR
    S["sysmon (監視スレッド)"] -->|"G が 10ms 以上走っている"| SIG["その G の M に<br/>シグナル (SIGURG) を送る"]
    SIG --> H["シグナルハンドラが<br/>安全な位置で G を止める"]
    H --> R["G を実行可能キューへ戻し、<br/>別の G を走らせる"]
```

| Go のバージョン | 方式 | 関数呼び出しのない無限ループ |
|---|---|---|
| 1.13 以前 | 協調的(関数の入り口で確認) | 止められない |
| 1.14 以降 | 非同期(シグナル) | 止められる |

演習6の結果(`GOMAXPROCS(1)`、無限ループの goroutine がいる状態で main が 100 ms 眠る):

- 既定: main は 112 ms 後に再開した。
- `GODEBUG=asyncpreemptoff=1`: 4 秒待っても再開しなかった。

## 5. 切り替えコストの比較(Apple M5)

| 切り替えの種類 | 1回あたり |
|---|---|
| goroutine(同じ OS スレッドの上) | 約 100 ns |
| OS スレッド(同じプロセス) | 約 1.4〜1.7 µs |
| OS プロセス | 約 2.0〜2.2 µs |
| goroutine を専用の OS スレッドに固定(`LockOSThread`) | 約 8 µs |

goroutine の切り替えが速いのは、**カーネルに入らず**、ユーザー空間の中で、レジスタを少し保存して移るだけだからである。
