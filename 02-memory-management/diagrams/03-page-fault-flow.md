# 図3: ページフォルトの流れ

## 1. 遅延割り当て(demand paging)

`mmap` や `malloc` の直後は、対応するページテーブルのエントリが空(Present = 0)である。

```mermaid
sequenceDiagram
    participant App as プログラム
    participant MMU as CPU / MMU
    participant OS as OS
    participant RAM as 物理メモリ
    App->>OS: mmap(1 GiB)
    OS->>OS: 「この範囲を使ってよい」と記録するだけ
    OS-->>App: すぐ成功 (物理メモリはまだ割り当てない)
    App->>MMU: ページ P に初めて書き込む
    MMU->>OS: ページフォルト (Present = 0)
    OS->>OS: P は確保済みの範囲か確認
    OS->>RAM: 空きフレームを取り、ゼロで埋める
    OS->>MMU: P → フレーム を登録 (Present = 1)
    OS-->>App: 命令をやり直す
    App->>RAM: 書き込み成功
```

## 2. 無効なアクセス

```mermaid
flowchart TB
    A["ページフォルト発生"] --> B{"そのアドレスは<br/>確保済みの範囲か?"}
    B -->|"いいえ"| X["SIGSEGV<br/>(セグメンテーションフォルト)"]
    B -->|"はい"| C{"アクセスの種類は<br/>許されているか?"}
    C -->|"いいえ (例: 読み取り専用に書く)"| D{"COW 対象か?"}
    D -->|"いいえ"| X
    D -->|"はい"| E["ページをコピーして、<br/>書き込み可能に変える"]
    C -->|"はい"| F{"内容はどこにある?"}
    F -->|"新規 (匿名)"| G["ゼロ埋めしたフレームを割り当てる<br/>【マイナーフォルト】"]
    F -->|"ファイル (ページキャッシュにあり)"| H["キャッシュのページを対応づける<br/>【マイナーフォルト】"]
    F -->|"ファイル / スワップ (ディスク上)"| I["ディスクから読み込む<br/>【メジャーフォルト】"]
```

## 3. fork と Copy-on-Write

```mermaid
sequenceDiagram
    participant P as 親
    participant C as 子
    participant OS as OS
    participant F as 物理フレーム X
    P->>OS: fork()
    OS->>OS: ページテーブルを子へ複製
    OS->>OS: 書き込み可能なページを、親子とも読み取り専用に変更
    Note over P,F: 親と子が同じフレーム X を共有している
    C->>F: X を読む → 共有のまま成功
    C->>OS: X に書く → 読み取り専用違反 (フォルト)
    OS->>OS: 新しいフレーム Y を取り、X の内容をコピー
    OS->>C: 子の X → Y を、書き込み可能で登録
    C->>F: Y に書く
    Note over P,F: 親の X は変わらない。親が後で書くときも、<br/>共有が残っていればコピーが起きる
```

## 4. 3種類のフォルトの比較

| 種類 | 起きる場面 | 代表的なコスト(今回の M5 での目安) |
|---|---|---|
| マイナー(ゼロ埋め) | 初めて触れる匿名ページ | 約 2〜3 µs/ページ |
| マイナー(COW) | fork 後の書き込み | 約 3 µs/ページ |
| メジャー | ディスクから読む | SSD で数十 µs〜、HDD で数 ms |
| 無効 | 範囲外、権限違反 | プロセスの強制終了 |
