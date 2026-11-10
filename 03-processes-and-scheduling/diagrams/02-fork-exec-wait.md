# 図2: fork / exec / wait とパイプ

## 1. シェルが `ls` を実行する流れ

```mermaid
sequenceDiagram
    participant S as シェル (親)
    participant C as 子プロセス
    participant K as カーネル
    S->>K: fork()
    K-->>S: 子の PID
    K-->>C: 0 (子は親の複製)
    Note over C: ここで fd の付け替えなどができる
    C->>K: exec("ls")
    K->>C: アドレス空間を ls で置き換える (PID は同じ)
    S->>K: waitpid(子)
    C->>K: exit(0)
    K-->>S: 終了状態を渡す (ここで子の情報が消える)
```

## 2. `a | b` の fd のつながり

```mermaid
flowchart LR
    subgraph プロセスa
        A1["fd 1 (標準出力)"]
    end
    subgraph カーネル
        P["パイプ<br/>(バッファ)"]
    end
    subgraph プロセスb
        B0["fd 0 (標準入力)"]
    end
    A1 -->|"書き端 (dup2 で fd 1 へ)"| P
    P -->|"読み端 (dup2 で fd 0 へ)"| B0
```

手順:

1. `pipe(fds)` → `fds[0]` が読み端、`fds[1]` が書き端。
2. `a` を `fork`。子で `dup2(fds[1], 1)`、パイプの fd を `close`、`exec(a)`。
3. `b` を `fork`。子で `dup2(fds[0], 0)`、パイプの fd を `close`、`exec(b)`。
4. **親は `fds[0]` と `fds[1]` の両方を `close`。**
5. 親は、2つの子を `wait`。

**手順 4 を忘れると**: 親が書き端を持ち続けるので、`b` は EOF を受け取れず、データを待ち続ける。パイプラインが終わらない。

## 3. リダイレクト `sort < in.txt > out.txt`

```mermaid
flowchart TB
    F["fork()"] --> C["子の中で"]
    C --> O1["open(in.txt) → dup2 で fd 0 へ"]
    O1 --> O2["open(out.txt) → dup2 で fd 1 へ"]
    O2 --> E["exec(sort)"]
    E --> R["sort は、標準入力を読み標準出力へ書くだけ<br/>(ファイルのことは知らない)"]
```

## 4. ゾンビと孤児

```mermaid
stateDiagram-v2
    [*] --> 実行中: fork
    実行中 --> ゾンビ: exit (親はまだ wait していない)
    ゾンビ --> 消滅: 親が wait → 終了状態を受け取る
    実行中 --> 孤児: 親が先に終了
    孤児 --> 実行中_PID1の子: PID 1 に引き取られる (ppid=1)
    実行中_PID1の子 --> ゾンビ: exit
    ゾンビ --> 消滅: PID 1 が wait
```

| 状況 | `ps` での見え方 | 問題 |
|---|---|---|
| ゾンビ | STAT が Z、`<defunct>` | PID が占有され続ける。親が `wait` を呼ぶと消える |
| 孤児 | PPID が 1(macOS は launchd) | 通常は問題ない。PID 1 が回収する |
| コンテナの PID 1 が回収しない | Z が溜まる | PID の枯渇。`tini` などの init を使う |

## 5. 演習3の出力(Apple M5)

```
--- 子が終了し、親が wait する前 ---
  PID  PPID STAT COMM
36949 36948 Z    <defunct>
--- waitpid の後 (終了コード = 7) ---
(pid 36949 はもう存在しない)
```

```
孫: 起動直後         ppid=36954
孫: 親が終了した後   ppid=1  (1 = init / launchd に引き取られた)
```
