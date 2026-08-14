# Go 1.26の`runtime/secret`は何をどこまで消すのか

Go 1.26で実験的に導入された[`runtime/secret`](https://pkg.go.dev/runtime/secret)について、`secret.Do`内で使用した値がプロセスのcore dumpに残るかを検証するリポジトリです。

## 検証したいこと

`secret.Do`は、その呼び出しツリーで使用された一時領域について、次の消去を保証しています。

- レジスタ：`Do`が返る前
- スタック：`Do`が返る前
- ヒープ：対象のallocationが到達不能になり、GCが認識した後

一方、外部へコピーした値、グローバル変数、到達可能なヒープallocationなどは保護されません。このリポジトリでは、これらの境界を対照実験で確認します。

## 実験の仕組み

1. `cmd/subject`が決定論的な4096バイトのマーカーを生成する
2. ケースごとに、スタック・ヒープ・グローバル領域などへマーカーを配置する
3. 観測対象の時点で`READY`を出力し、プロセスを停止状態にする
4. 別プロセスから`gcore`でcore dumpを取得する
5. `cmd/scanner`がcore dump内のマーカー先頭256バイトを検索する

マーカー本体はソースコード、実行ファイル、引数、環境変数、ログには格納しません。subjectとscannerが同じアルゴリズムから個別に再生成します。

heapケースではsliceを一度package-level変数へ代入し、コンパイラに明示的にescapeさせます。到達不能なheapを調べるケースでは、その参照を`nil`へ戻してから観測します。また、対象allocationの直前・直後に作ったanchorを到達可能なまま残し、対象を含むspan全体がscavengeされただけで「消去された」と誤判定することを防ぎます。ビルド時のescape analysisは`escape-analysis.txt`へ保存されます。

## 検証ケース

| ケース | 状態 | 主な観測目的 |
|---|---|---|
| `plain-stack-live` | 通常関数の生存中stack | scannerがliveな値を検出できること |
| `plain-stack-returned` | 通常関数のreturn後 | 通常のstackに残留するか |
| `secret-stack-live` | `Do`の実行中 | `runtime/secret`がliveな値を隠す機能ではないこと |
| `secret-stack-returned` | `Do`のreturn後 | stackが消去されるか |
| `plain-heap-after-gc` | 通常allocation、GC後 | 通常のGCとの違い |
| `secret-heap-live` | `Do`内の到達可能なheap | liveなheap値は残ること |
| `secret-heap-before-gc` | `Do`終了後、GC前 | heap消去がGCに依存すること |
| `secret-heap-after-gc` | `Do`終了後、GC後 | 到達不能なheapが消去されるか |
| `secret-heap-escaped` | `Do`外から到達可能 | 到達可能なallocationは残ること |
| `secret-copy-out` | 外部allocationへコピー | `Do`外のコピーは残ること |
| `secret-global` | グローバル配列へ書き込み | グローバル領域は対象外であること |
| `secret-panic` | `Do`内でpanic | panic時にもstackが消去されるか |

`plain-stack-returned`や`secret-heap-before-gc`は、コンパイラやGCのタイミングによって結果が変わり得る観測ケースです。単に「0件なら成功」とは判定しません。

## 実行環境

- Linux/amd64またはLinux/arm64
- Go 1.26以上
- `gcore`（gdbに同梱）
- ptrace可能な実行環境

`runtime/secret`は実験的APIであり、Go 1互換性保証の対象外です。ビルドには`GOEXPERIMENT=runtimesecret`が必要です。

### Dockerで実行

```bash
docker compose run --rm validation
```

core dump取得に必要な`SYS_PTRACE` capabilityとseccomp設定は`compose.yaml`に定義されています。core dump自体は既定で削除され、検索件数と先頭offsetだけが`artifacts/manual/`へ保存されます。

core dumpも残す場合：

```bash
docker compose run --rm -e KEEP_CORES=1 validation
```

### ホストで実行

```bash
sudo apt-get install gdb
GOEXPERIMENT=runtimesecret go test ./...
./scripts/run.sh
```

UbuntuのYama設定などによってattachが拒否される場合は、root権限で`gcore`を実行します。

```bash
USE_SUDO_GCORE=1 ./scripts/run.sh
```

## 出力

```text
artifacts/manual/
├── summary.tsv
├── escape-analysis.txt
├── plain-stack-live.json
├── plain-stack-live.log
└── ...
```

`summary.tsv`には、各core dumpから見つかったマーカーの件数が記録されます。GitHub Actionsでも同じ検証を実行し、TSV・JSON・ログをworkflow artifactとして保存します。

## 注意点

- 「見つからなかった」は、あらゆる秘密情報の消去を証明するものではありません。
- core dumpからの検索では、レジスタ消去の因果関係を十分に検証できません。レジスタは逆アセンブルとGo runtimeの実装を別途確認する必要があります。
- コンパイラによるescapeやstack frameの再利用が結果に影響します。記事化するときは`-gcflags='-m=2'`の結果も併記します。
- マーカーは検証専用です。暗号学的な乱数生成器ではありません。
