# changedate

指定ディレクトリ配下の画像・動画ファイルの更新日時を一括変更するCLIです。
Google Drive上で日付順に並べたいファイルのmtimeを揃える用途を想定しています。

## 対象

- `jpg`
- `jpeg`
- `png`
- `heic`
- `mp4`
- `mov`

## ビルド

```bash
go build -o changedate ./cmd/changedate
```

Windows向けバイナリをmacOS/Linuxからビルドする場合は次を使います。

```bash
GOOS=windows GOARCH=amd64 go build -o changedate.exe ./cmd/changedate
```

Windows上で直接ビルドする場合は次で十分です。

```powershell
go build -o changedate.exe ./cmd/changedate
```

## 使い方

`--files` を指定した場合は `--dir` より優先され、指定したファイルだけを対象にします。
複数指定は `--files a.jpg,b.mp4` または `--files a.jpg --files b.mp4` のどちらでも可能です。
`--dir` はシンボリックリンクのフォルダもたどります。リンク先の中身を含めて再帰的に対象を収集します。

`--mode csv` では `--csv` で指定したCSVを読み込み、各行の `path,datetime` に従って個別にmtimeを変更します。
`--mode export-csv` では対象ファイルをmtime順に並べたCSVを出力します。`--csv-out` を指定するとファイルに保存できます。
`--mode csv-autofill` では CSV の `datetime` をファイル名から自動補完します。
`--mode batch` では現状のCSV出力、CSV自動補完、dry-run、本更新をまとめて実行します。
`--mode batch` は `--dir` と `--backup-csv-dir` を指定して使います。
CSVの日時は `2024-01-01 10:00:00` に加えて `20240101_100000` 形式も受け付けます。
`--set-birthtime` または `--with-creation-time` を付けると、macOSでは `SetFile`、Windowsでは標準APIを使って作成日時も更新します。Linuxでは SMB 共有に対して `smbclient` を使います。
`--backup-csv-dir` を付けると、変更前の状態を日付付きファイル名のCSVとして退避してから更新します。
batch のバックアップCSVでは `path` は絶対パスで保存されます。
`--log-file` を付けると、進捗ログや警告、エラーを追記保存できます。既存ファイルは消さずに末尾へ追記します。
進捗ログは標準エラー出力に出します。CSV本体や dry-run の出力は標準出力のままです。
macOSで作成日時更新を行う場合は Xcode Command Line Tools が必要です。Linuxで指定する場合は `smbclient` と `--smb-share`、`--smb-root`、`--smb-auth-file` が必要です。SMB 認証ファイルは `smbclient -A` の形式で用意してください。更新日時を設定した後、同じ対象の作成日時を SMB 経由で設定します。SMB コマンドでエラーが起きた場合はエラーを返します。

Linux の SMB 共有での例:

```bash
./changedate \
  --dir /mnt/nas/photos \
  --mode batch \
  --backup-csv-dir /mnt/nas/toolChangeDate/backup \
  --set-birthtime \
  --smb-share //nas/share \
  --smb-root /mnt/nas \
  --smb-auth-file /path/to/smb-credentials \
  --timezone Asia/Tokyo
```

Linux の CIFS マウントでは、SMB 経由で変更した作成日時を `stat` やファイルマネージャーが古い値のまま表示する場合があります。NAS 側の値は `smbclient //nas/share -A /path/to/smb-credentials -c 'allinfo photos/file.jpg'` で確認できます。マウント側の表示を更新するには、共有を使うアプリを閉じてアンマウント・再マウントしてください。`x-systemd.automount` を使う場合は、アンマウント後に対応する automount ユニットを起動し直してください。

### Ubuntu での cron 実行

`scripts/changedate-ubuntu.sh` は、CIFS 共有が書き込み可能であることを確認し、ロックで重複実行を防いで batch 更新を実行します。`changedate-linux` をツール用ディレクトリに配置して使います。

環境に合わせて次の起動用スクリプトをローカルに保存してください。認証情報そのものは記述せず、SMB 認証ファイルのパスを指定します。

```bash
#!/usr/bin/env bash
set -euo pipefail
export CHANGEDATE_TOOL_DIR=/mnt/nas/share/toolChangeDate
export CHANGEDATE_PHOTO_DIR=/mnt/nas/share/photos
export CHANGEDATE_SMB_SHARE=//nas/share
export CHANGEDATE_SMB_ROOT=/mnt/nas/share
export CHANGEDATE_SMB_AUTH_FILE=/path/to/smb-credentials
exec /bin/bash /path/to/changedate/scripts/changedate-ubuntu.sh
```

必要なコマンドは `findmnt`、`flock`、`grep`、`date`、`smbclient` です。タイムゾーンは既定で `Asia/Tokyo`、ロックファイルは `/tmp/changedate-ubuntu.lock` です。`CHANGEDATE_TIMEZONE`、`CHANGEDATE_LOCK_FILE` で変更できます。NAS を再マウントするスクリプトと併用するときは、同じロックファイルを使ってください。

毎時30分に実行する cron の例:

```cron
30 * * * * /bin/bash /path/to/launcher.sh >> /path/to/local/cron.log 2>&1
```

cron のログは NAS 外のローカルに保存してください。マウント判定で停止した場合、NAS 側の実行ログは作成されず、このログに理由が記録されます。

`x-systemd.automount` を使う環境では、`findmnt -T` が同じパスに対して `autofs` と `cifs` の両方を返す場合があります。スクリプトはファイルシステム種別・マウントオプションの両方の照会を `-t cifs` で絞り込み、書き込み可能な CIFS を誤って未マウント扱いしないようにしています。CIFS が存在しない場合や読み取り専用の場合は処理を停止します。

マウント判定の回帰テスト（NAS ファイルは変更しません）:

```bash
python3 -m unittest discover -s scripts/tests -v
```

### 1. 一括日時設定

すべての対象ファイルに同じ日時を適用します。

```bash
./changedate \
  --dir ./photo \
  --mode all \
  --datetime "2024-01-01 10:00:00"
```

### 2. 連番日時設定

ファイル名昇順で処理し、開始日時から `--step` 秒ずつ加算して設定します。

```bash
./changedate \
  --dir ./photo \
  --mode sequence \
  --start "2024-01-01 10:00:00" \
  --step 60
```

### 3. ドライラン

実際には変更せず、変更予定だけを表示します。

```bash
./changedate \
  --dir ./photo \
  --mode sequence \
  --start "2024-01-01 10:00:00" \
  --step 60 \
  --dry-run
```

### 4. EXIF反映

JPEGファイルの `DateTimeOriginal` を読み、mtimeに反映します。
EXIFがないファイルはスキップします。
HEIC / MP4 / MOV のメタデータ取得は未対応です。

```bash
./changedate --dir ./photo --mode exif
```

### 5. 直接指定

単体ファイルや複数ファイルを直接指定できます。

```bash
./changedate \
  --files ./photo/a.jpg,./photo/b.mp4 \
  --mode sequence \
  --start "2024-01-01 10:00:00" \
  --step 60
```

### 6. CSV指定

CSVの例:

```csv
path,datetime
./photo/a.jpg,2024-01-01 10:00:00
./photo/b.mp4,2024-01-01 10:01:00
```

実行例:

```bash
./changedate --mode csv --csv ./changes.csv
```

### 7. CSV自動補完

ファイル名から可能な範囲で `datetime` を補完します。
`20260515_193217.JPG` は `2026-05-15 19:32:17` として扱います。
`PXL_20260514_065248267.jpg` のように完全な時刻として扱わないものは、同日内で `00:00:00` から1秒ずつ連番にします。
補完ルールは次の通りです。

- 判定対象はファイル名の basename です。
- `YYYYMMDD_HHMMSS` または `YYYYMMDD-HHMMSS` が含まれる場合は、その日時をそのまま `datetime` に入れます。
- `YYYYMMDD_HHMMSSmmm` または `YYYYMMDD-HHMMSSmmm` が含まれる場合は、末尾 3 桁をミリ秒として扱います。
- ファイル名のベース部分が Unix タイムスタンプの数字だけで構成されている場合は、それを日時として扱います。`10桁` は秒、`13桁` はミリ秒、`16桁` はマイクロ秒、`19桁` はナノ秒です。
- `YYYYMMDD` だけが含まれる場合は、その日付の `00:00:00` を基準にします。
- 日付だけが取れた行が同じ日付内に複数ある場合は、ファイル名昇順で `00:00:00` から 1 秒ずつ加算します。
- 日時を推定できないファイルはそのまま残します。
`--autofill-exclude-regex` を指定すると、正規表現に一致する basename は自動補完の対象外にできます。たとえば `^PXL` で `PXL_...` を除外できます。

```bash
./changedate \
  --mode csv-autofill \
  --csv ./changes.csv
```

`--dry-run` を付けると、更新後CSVを標準出力に表示します。
`--csv-out` を付けると別ファイルに保存できます。省略時は入力CSVを上書きします。

### 8. CSV出力

対象ファイルを現在のmtime順でCSVに書き出します。

```bash
./changedate \
  --dir ./photo \
  --mode export-csv \
  --csv-out ./changes.csv
```

`--csv-out` を省略すると標準出力に出ます。

`--mode csv` の実行時には `--dry-run` も有効で、変更予定のログだけを出して実ファイルは変更しません。
`--dry-run` の出力先頭には `path`, `before`, `after` のヘッダーを出します。
CSV に存在しないファイルパスが含まれている場合は、その行を `SKIP` として表示し、処理を続行します。

### 9. バックアップ付き更新

`--backup-csv-dir` を付けると、変更前のファイル一覧を `changedate-backup-YYYYMMDD-HHMMSS.csv` という名前で保存してから更新します。
バックアップCSVはロールバック用にそのまま `--mode csv` で使えます。
`--mode batch` ではこのバックアップCSVに加えて、自動補完後のCSVも同じディレクトリに保存します。

```bash
./changedate \
  --mode csv \
  --csv ./changes.autofill.csv \
  --set-birthtime \
  --backup-csv-dir ./backup
```

バックアップを残したまま更新するので、意図しない変更があれば保存済みCSVを使って戻せます。
`--set-birthtime` を付けて更新した場合、ロールバック時も同じフラグを付けると CSV に記録された日時で作成日時も合わせられます。

### 10. 一括実行

`--mode batch` で、よく使う流れの 1 から 4 までをまとめて実行できます。
この項目では、実行ログを残すために `--log-file` を指定する前提にしています。
`--backup-csv-dir` にはバックアップCSVの保存先を指定し、`--log-file` には進捗やエラーを追記保存するファイルを指定します。
`--log-file` は追記モードで開くため、同じファイルを繰り返し使えます。
`--dry-run` を付けると、最後の更新は行わず dry-run までで止めます。

```bash
./changedate \
  --dir ./photo \
  --mode batch \
  --backup-csv-dir ./backup \
  --set-birthtime \
  --autofill-exclude-regex '^PXL' \
  --log-file "./logs/changedate-$(date +%Y%m%d-%H%M%S).log"
```

実行すると次のファイルが残ります。

- `./backup/changedate-backup-YYYYMMDD-HHMMSS.NNNNNNNNN.csv`
- `./backup/changedate-backup-YYYYMMDD-HHMMSS.NNNNNNNNN.autofill.csv`
- `./logs/changedate-YYYYMMDD-HHMMSS.log`

前2つはロールバック用のCSV、最後の1つは進捗や警告、エラーを確認するためのログです。

Windowsで同じ内容を実行する場合は、PowerShell では次のように書けます。

```powershell
$ts = Get-Date -Format "yyyyMMdd-HHmmss"
.\changedate.exe `
  --dir .\photo `
  --mode batch `
  --backup-csv-dir .\backup `
  --set-birthtime `
  --autofill-exclude-regex '^PXL' `
  --log-file ".\logs\changedate-$ts.log"
```

`cmd.exe` で実行する場合は、`Get-Date` 相当を別コマンドで用意するか、ログファイル名を固定してください。`$(date ...)` は Windows の `cmd.exe` では使えません。

## よく使う流れ

頻度が高い想定のため、CSVを書き出して編集し、その内容で更新する流れをそのまま使える形で載せます。

### 1. 現状のCSVを出力

```bash
./changedate \
  --dir ./photo \
  --mode export-csv \
  --csv-out ./changes.csv
```

### 2. CSV更新

`./changes.csv` に対して `--mode csv-autofill` を使い、ファイル名から可能な範囲で `datetime` を自動補完します。
自動補完できなかった日時は手動で `datetime` を編集してください。
`--csv-out` を省略すると入力CSVを上書きします。

```bash
./changedate \
  --mode csv-autofill \
  --csv ./changes.csv \
  --csv-out ./changes.autofill.csv
```

### 3. 更新日時と作成日時を対象に変更するCSVで dry-run

`--set-birthtime` を付けると、macOSでは SetFile、Windowsでは標準API、Linux の SMB 共有では `smbclient` で作成日時も更新対象になります。Linux では上記の SMB オプションも指定してください。

```bash
./changedate \
  --mode csv \
  --csv ./changes.autofill.csv \
  --set-birthtime \
  --dry-run
```

### 4. 日時更新

dry-run の内容に問題がなければ、`10. 一括実行` の `--mode batch` を使って一括更新します。

`--backup-csv-dir` を付けた実行では、更新前のCSVと自動補完後のCSVが `./backup/` に保存されます。
ロールバックしたい場合は、バックアップCSVをそのまま使って `--mode csv` で戻せます。

## ログ

標準出力に以下の形式で出します。

```text
path/to/file.jpg    2024-01-01 09:00:00    2024-01-01 10:00:00
```

スキップ時は `SKIP` と理由を表示します。

## テスト

```bash
go test ./...
```

`internal/changedate` のテストでは、ダミーファイルを使って
ファイル収集、ドライラン、mtime変更、JPEG EXIFの読み取りを確認します。
