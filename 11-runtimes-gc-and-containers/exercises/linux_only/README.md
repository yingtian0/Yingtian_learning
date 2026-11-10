# Linux 専用の演習 (この Mac では、実行していない)

コンテナの正体 (名前空間と cgroup) は、Linux カーネルの機能です。macOS では実行できません。
ここのコードは、**Linux 上で、あなたが実行して確認する**ためのものです。この講義の作成時には、次の確認だけをしました。

- `ns_demo.c`: コンパイルも実行もしていない (macOS には clone と CLONE_NEW* が無いため)。書いた内容は未確認
- `cgroup_probe.go`: `GOOS=linux go vet` で、コンパイルが通ることの確認のみ (未実行)

| ファイル | 内容 | 期待すること (未確認の予想) |
|---|---|---|
| `ns_demo.c` | clone で、PID・UTS・マウント・ユーザの名前空間を作る | 子の中では PID が 1。hostname を変えても、親に影響しない |
| `cgroup_probe.go` | cgroup v2 の cpu.max、memory.max、cpu.stat を読む | コンテナの中で、設定した上限が見える。`nr_throttled` が、スロットリングの回数 |

## 追加の課題 (Linux 上で)

1. `docker run --cpus=2 -m 256m golang:1.22 ...` で `cgroup_probe.go` を実行し、`cpu.max` と `memory.max` の値を確認する。
2. ex5 のシミュレーションと、同じ条件 (CPU 上限 2、短い処理を多数) で、負荷を掛け、`cpu.stat` の `nr_throttled` が、増えるかを見る。
3. メモリ上限を、ヒープより小さくして、`exit code 137` (SIGKILL) で終わるのを確認する。`GOMEMLIMIT` を、上限の 90% 程度に設定すると、どうなるか。
