# 図2: fd、開いたファイル、inode、ディレクトリ

## 1. 3つの層

```mermaid
flowchart LR
    subgraph プロセスA["プロセス A"]
        FD3["fd 3"]
        FD4["fd 4"]
        FD5["fd 5 (fd 3 を dup)"]
    end
    subgraph カーネル["カーネル"]
        OF1["開いたファイル 1<br/>オフセット = 2<br/>フラグ (読み取り専用 など)"]
        OF2["開いたファイル 2<br/>オフセット = 0"]
        IN["inode<br/>サイズ・権限・リンク数<br/>データブロックの場所"]
    end
    FD3 --> OF1
    FD5 --> OF1
    FD4 --> OF2
    OF1 --> IN
    OF2 --> IN
    IN --> DB["データブロック"]
```

| 層 | 持つもの | 共有されるとき |
|---|---|---|
| fd | 整数。プロセスごとの表の添字 | プロセスごと |
| 開いたファイル | オフセット、開いたときのフラグ | `dup`、`fork` で共有。別に `open` すると別 |
| inode | ファイルの実体(メタデータ、ブロックの場所) | 同じファイルなら全員 |

演習1の結果: fd1 で2バイト読んだ後の位置は、fd1 = 2、別に `open` した fd2 = 0、`dup` した fd3 = 2。

## 2. 名前は inode の外にある

```mermaid
flowchart TB
    DIR["ディレクトリ (名前 → inode 番号 の表)"]
    DIR -->|"a.txt → 12362757"| I1["inode 12362757<br/>リンク数 = 2"]
    DIR -->|"b.txt → 12362757 (ハードリンク)"| I1
    DIR -->|"s.txt → 12362760 (シンボリックリンク)"| I2["inode 12362760<br/>中身 = 文字列 '/…/a.txt' (65 バイト)"]
    I2 -.->|"パス名を辿る"| DIR
    I1 --> B["データブロック"]
```

| 操作 | 何が起きるか |
|---|---|
| `link(a, b)` | ディレクトリに、同じ inode を指す名前を足す。リンク数 +1 |
| `symlink(a, s)` | パス名の文字列を中身に持つ、**新しい inode** を作る |
| `unlink(a)` | ディレクトリから名前を除き、リンク数 −1 |
| リンク数 = 0 かつ開いている fd なし | inode とデータブロックを解放 |

## 3. 「消したのに領域が戻らない」

```mermaid
sequenceDiagram
    participant P as プロセス (ログを書き続けている)
    participant D as ディレクトリ
    participant I as inode (リンク数 1)
    participant U as 運用担当者
    P->>I: open (fd を保持)
    U->>D: rm app.log → 名前を除く
    D->>I: リンク数 = 0
    Note over I: fd が残っているので、inode とデータは解放されない<br/>df の使用量は減らない
    P->>I: write (まだ書ける)
    P->>I: close
    Note over I: ここで初めて領域が解放される
```

演習1: 開いたまま `unlink` した後、`fstat` は **リンク数 0**、`pread` では中身が読めた。

見つけ方(Linux): `lsof +L1`。対策: プロセスにログを閉じさせる(ログローテーションの通知、再起動)。

## 4. inode からデータブロックを指す方法

**ポインタ方式**(ext2/3、ブロック 4KB、ポインタ 4 バイト):

```mermaid
flowchart LR
    INODE["inode"] --> D["直接ポインタ × 12<br/>(48 KB)"]
    INODE --> S["1段間接 → 1024 ブロック<br/>(4 MB)"]
    INODE --> DD["2段間接 → 1024² ブロック<br/>(4 GB)"]
    INODE --> T["3段間接 → 1024³ ブロック<br/>(4 TB)"]
```

**エクステント方式**(ext4、XFS、APFS など):

```
エクステント = (ファイル内の開始位置, ディスク上の開始ブロック, 連続するブロック数)
例: (0, 5000, 2048)  → ファイルの先頭 2048 ブロックは、ディスクのブロック 5000 から連続
```

大きな連続ファイルが、数個の項目で表せる。

## 5. スパースファイル(演習1)

```
ファイル: [ 穴 (書かれていない: ブロックなし, 読むとゼロ) ........ ][x]
           0                                                1 GiB   +1 バイト
st_size  = 1,073,741,825 バイト
実使用   = 4,096 バイト (末尾の 1 ブロックだけ)
```

`ls -l` のサイズと `du` の値が食い違う理由。

## 6. VFS

```mermaid
flowchart TB
    APP["アプリ: open / read / write / close"] --> VFS["VFS (共通の層)"]
    VFS --> EXT4["ext4"]
    VFS --> XFS["XFS"]
    VFS --> APFS["APFS"]
    VFS --> NFS["NFS (ネットワーク)"]
    VFS --> PROC["/proc (仮想)"]
    EXT4 --> BLK["ブロック層 → ドライバ → ストレージ"]
    XFS --> BLK
    APFS --> BLK
```
