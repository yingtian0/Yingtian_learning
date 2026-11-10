# 図1: 更新の喪失とアトミック命令

## 1. counter++ の3つの手順と、割り込み

```
counter++  は、実際には:
   1. LOAD   r ← [counter]
   2. ADD    r ← r + 1
   3. STORE  [counter] ← r
```

```mermaid
sequenceDiagram
    participant A as スレッド A
    participant M as メモリ counter
    participant B as スレッド B
    Note over M: counter = 5
    A->>M: LOAD (5)
    B->>M: LOAD (5)
    A->>A: ADD → 6
    B->>B: ADD → 6
    A->>M: STORE 6
    B->>M: STORE 6
    Note over M: 2回加算したのに counter = 6 (本来は 7)<br/>Bの加算が、Aの書き込みで上書きされて消えた
```

## 2. 実測(演習1: 8 スレッド × 200 万回、正しい合計 1600 万)

| 方式 | 結果 | 失われた割合 |
|---|---|---|
| 保護なし | 約 300 万〜400 万 | 75〜81% |
| mutex | 1600 万 | 0% |
| アトミック加算 | 1600 万 | 0% |
| スレッドごとのカウンタ | 1600 万 | 0% |

## 3. CAS(compare-and-swap)のやり直しループ

```mermaid
flowchart TB
    S["old ← 共有変数を読む"] --> C["new ← old から計算した新しい値"]
    C --> CAS{"CAS(共有変数, old, new)<br/>共有変数が、まだ old のままか?"}
    CAS -->|"はい: new に書き換わった"| OK["成功"]
    CAS -->|"いいえ: 他のスレッドが先に変えた"| S
```

- CAS 全体が、**1つの不可分な操作**である。
- 失敗したら、最新の値を読み直して、**やり直す**。これがロックフリーの基本の形(演習4の Treiber スタック)。

## 4. アトミック命令とキャッシュライン

```mermaid
sequenceDiagram
    participant C0 as コア 0
    participant L as 共有変数のキャッシュライン
    participant C1 as コア 1
    C0->>L: fetch_add (ラインを M 状態で所有)
    C1->>L: fetch_add (コア 0 のコピーを無効化し、ラインを転送)
    C0->>L: fetch_add (再び無効化・転送)
    Note over C0,C1: アトミック操作の1回ごとに、ラインが往復する<br/>(第1回の MESI と cache line ping-pong)<br/>→ コアを増やしても、全体の処理量は増えない
```

## 5. データ競合と競合状態

```mermaid
flowchart LR
    subgraph DR["データ競合 (data race)"]
        D1["同じ場所に、同時にアクセス"]
        D2["少なくとも片方が書き込み"]
        D3["順序づけ (happens-before) がない"]
    end
    subgraph RC["競合状態 (race condition)"]
        R1["実行の順序・タイミングで<br/>結果の正しさが変わる"]
    end
    DR -->|"たいてい原因になる"| RC
    RC -.->|"データ競合なしでも起きる<br/>(bank の check-then-act)"| RC
```

| | `-race` で見つかる | 例 |
|---|---|---|
| データ競合 | はい | 保護なしの `counter++` |
| 競合状態(データ競合なし) | **いいえ** | 確認(ロック)→ 引き出し(ロック) の間の割り込み |

## 6. check-then-act (演習6)

```mermaid
sequenceDiagram
    participant A as goroutine A
    participant X as 口座 (残高 100)
    participant B as goroutine B
    A->>X: Balance() → 100 (ロックして放す)
    B->>X: Balance() → 100 (ロックして放す)
    A->>X: Withdraw(100) → 残高 0
    B->>X: Withdraw(100) → 残高 -100
    Note over X: 各操作は mutex で守られているが、<br/>「確認と引き出し」が1つの操作になっていない
```

修正: 確認と引き出しを、**1つのロックの中**でやる(`TryWithdraw`)。

| 実行 | マイナスの口座 (全 20,000) |
|---|---|
| 通常(5回) | 0〜4 |
| `-race` 付き(3回) | 179〜613 (race detector は何も報告しない) |
| 修正版 | 0 |
