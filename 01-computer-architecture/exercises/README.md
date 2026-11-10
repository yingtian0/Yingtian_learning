# 第1回 演習

すべて C(標準ライブラリと pthread のみ)。macOS(Apple Silicon)と Linux で動作確認済み。ビルドは `cc` で足りる。

```bash
cd 01-computer-architecture/exercises
```

## 演習の進め方(重要)

1. **実行前に予想を書く**(数値の桁、グラフの形)。
2. 実行して比べる。外れたら**なぜ外れたか**を調べる。これが学習の本体。
3. 結果と考察を `notes.md` などに残す(自分用)。

計測の注意:

- 他の重いプロセスを止める。ノートPCは電源接続、可能なら省電力モードを切る。
- 各計測は**3回以上**実行し、ばらつきを見る。
- 最適化レベル(`-O0/-O1/-O2`)で結果が変わることがある。何を測っているかを確認する癖をつける。

---

## 演習1: キャッシュレイテンシの階段

[ex1_cache_latency.c](ex1_cache_latency.c)

```bash
cc -O2 -o ex1 ex1_cache_latency.c && ./ex1
```

**課題**

1. 出力を表計算かグラフに起こし、階段の段差(各キャッシュ容量)を読み取る。
2. 自分の CPU の公式スペック(L1/L2/L3 容量)と照合する。
   - macOS: `sysctl hw.l1dcachesize hw.l2cachesize hw.cachelinesize`
   - Linux: `lscpu | grep -i cache`
3. **改造**: ノードを 64B ではなく **16B 間隔**にして(同じライン内に4ノード)再計測。結果はどう変わるか。なぜか。
4. **改造**: シャッフルをやめて、順次(`perm[i] = (i+1) % nodes`)にして再計測。プリフェッチの効果が見えるか。
5. 発展: 作業集合を 4MB に固定し、**2つの独立したチェーン**を交互にたどると、ns/アクセスはどうなるか(MLP の実測)。

## 演習2: false sharing

[ex2_false_sharing.c](ex2_false_sharing.c)

```bash
cc -O2 -pthread -o ex2 ex2_false_sharing.c && ./ex2
```

**課題**

1. `NTHREADS` を 1, 2, 4, 8 と変えて、packed と padded の時間をグラフにする。packed はどこで悪化するか。
2. `PAD` を 8, 16, 32, 64, 128 と変え、**どの値で解消するか**を調べる。自分の CPU のキャッシュライン幅と一致するか(Apple Silicon の一部は 128B)。
3. カウンタを `volatile` でなく `_Atomic uint64_t`(relaxed の `fetch_add`)にしたら、どうなるか。
4. 発展(Linux): `perf c2c record` / `perf c2c report` で false sharing を検出する。

## 演習3: 分岐予測

[ex3_branch_prediction.c](ex3_branch_prediction.c)

```bash
cc -O1 -o ex3 ex3_branch_prediction.c && ./ex3
```

**課題**

1. 実行して、ランダム vs ソート済みの比を記録する。
2. ソース中の `__asm__ volatile("" ::: "memory");` 行を**削除して**再ビルド・再計測。差がどうなるか。
3. 逆アセンブルで原因を確認する。
   - macOS: `otool -tV ex3 | less`(arm64 なら `csel` を探す)
   - Linux: `objdump -d ex3 | less`(x86 なら `cmov` を探す)
4. `-O2`、`-O3` でも試す。SIMD 化されるか。
5. データの値域を `rand() % 256` から `rand() % 2` や、「ほぼ常に true」(例: 95% が 200 以上)に変え、予測精度が時間にどう影響するか調べる。
6. 発展: 同じ処理を**分岐なし**(`sum += (data[i] >= 128) * data[i]` など)に書き換え、ランダム/ソート済みで時間が同じになることを確認する。

## 演習4: 空間局所性とストライド

[ex4_stride_matrix.c](ex4_stride_matrix.c)

```bash
cc -O1 -o ex4 ex4_stride_matrix.c && ./ex4
```

**課題**

1. 行優先と列優先の比を記録。`N` を 256 → 8192 と変え、比がいつ大きくなるか。キャッシュ容量との関係を述べよ。
2. ストライドごとの ns/アクセスを見て、**ライン境界(64B or 128B = 16 or 32 要素)**で段差が出るか確認する。
3. `N = 1024`(行長 = 4096B)と `N = 1025` で列優先の時間を比較する(conflict miss。図2参照)。
4. 発展: 行列積 `C = A × B` を 3重ループで書き、`ijk` と `ikj` の順序、さらに**ブロッキング**(タイリング)の効果を測る。

## 演習5: メモリ順序のリトマステスト

[ex5_litmus_store_buffer.c](ex5_litmus_store_buffer.c)

```bash
cc -O2 -pthread -o ex5_relaxed ex5_litmus_store_buffer.c && ./ex5_relaxed
cc -O2 -pthread -DSEQ_CST -o ex5_sc ex5_litmus_store_buffer.c && ./ex5_sc
```

**課題**

1. relaxed と seq_cst で、(0,0) の回数を比較する。
2. 逆アセンブルし、seq_cst の store がどんな命令になっているか確認する(x86: `xchg`/`mfence`、ARM: `stlr` など)。
3. **改造**: store と load の間に `atomic_thread_fence(memory_order_seq_cst)` を挟み(relaxed のまま)、(0,0) が消えることを確認する。
4. **改造**: Message Passing パターンを書く。
   ```
   T0: data = 42; flag = 1;
   T1: while (flag == 0) {}; r = data;
   ```
   relaxed だと `r = 0` が起こりうる(ARM では起こりやすい)か試す。release/acquire で修正する。
5. 発展: `herd7` / `litmus7`(herdtools7)で、同じテストをメモリモデルのシミュレータにかけ、実機の結果と比べる。

---

## 付録: Go での追試(任意)

C の結果を Go で再現し、**ランタイムが挟まる世界でも同じ物理法則が効く**ことを確認する。

1. 演習2を Go で書く(`sync.WaitGroup` と `[]struct{ n uint64; _ [120]byte }`)。`go test -bench` の `-cpu 1,2,4,8` で計測。
2. 演習1を Go で書く(`[]uint64` のポインタチェイス)。GC とスライスの境界チェックが、C と比べてどの程度の差を生むか。
3. `go test -race` を、演習5のような意図的な競合コードに対して走らせ、検出されることを確認する。

## 解答・参考結果

正解は1つではない(環境依存)。以下は参考例(Apple M5)。

| 演習 | 結果 |
|---|---|
| 1 | L1 ≈ 1ns、L2 ≈ 3〜8ns、DRAM ≈ 60〜84ns |
| 2 | packed 0.178s / padded 0.050s(4スレッド) |
| 3 | random 1.420s / sorted 0.155s |
| 4 | 列優先 / 行優先 ≈ 8倍 |
| 5 | relaxed: (0,0) が 499,812 / 500,000、seq_cst: 0 |

考察のポイントは講義本文 [README.md](../README.md) §10 を参照。
