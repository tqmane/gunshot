# 提供 IPA の互換性資料

[ドキュメント索引](../README.md) · [詳細な解析記録](index.md)

今回の整理では、提供された3版の main executable と主要 framework を読み取り、下表の instance-method metadata を再抽出しました。確認範囲は selector・encoding・Mach-O の最小 OS です。7.96.0 の全フックや call graph を監査したものではなく、認証・アップロード・Live Photo 再生の実機確認でもありません。

| 項目 | 7.20.2 | 7.92.0 | 7.96.0 |
| --- | --- | --- | --- |
| 主要 framework | `ModuleFramework` | `GooglePhotos_GeneratedFramework` | 同左 |
| main / framework の `LC_BUILD_VERSION` 最小 OS | iOS 16.1 | iOS 18.0 | iOS 18.0 |
| `PHSAccountManagerImpl.photosSSOService` | 未検出 | `@16@0:8` | `@16@0:8` |
| 同クラスの `ssoService` | `@16@0:8` | `@16@0:8` | `@16@0:8` |
| `GMUAssetUploadRequest` の完了 | `…errorCode:`、`v36@0:8B16@20q28` | `…error:`、`v36@0:8B16@20@28` | 同左 |
| `GMULivePhotoSingleUploadRequest.didCompleteWithError:resultantMediaItem:` | `v32@0:8@16@24` | 同左 | 同左 |
| 詳細画面の `getBackupStatusModelData` | 未検出 | `@16@0:8` | `@16@0:8` |
| 詳細画面の `modelForBackedupStatus` | `@16@0:8` | `@16@0:8` | `@16@0:8` |
| `PHSUserItemsSynchronizer.fetchData` | `v16@0:8` | 同左 | 同左 |

「未検出」は対象クラスの直接の instance-method list にないという意味です。継承・category・class method まで存在しないと断定するものではありません。7.20.2 の Info.plist は最小 OS を 10.0 と宣言していますが、両 Mach-O のビルド指定は 16.1 です。Gunshot 自体の deployment target とも別です。

[限定した metadata と入力 SHA-256](objc/compatibility-contracts.json) に結果を記録しています。既存の [7.92.0 全件索引](objc/README.md)と[7.20.2 の詳細監査](google-photos-7.20.2.md)は、その対象版の資料として維持します。IPA / 実行ファイル本体はリポジトリに追加しません。

実装は引き続き必要な API を実行時に検査します。`auditedHostVersion` が示す従来の参照版は 7.20.2 / 7.92.0 のままで、この限定的な比較だけで 7.96.0 を検証済みに変更しません。

## 再確認

IPA を展開し、main と該当 framework に `scripts/extract-objc-metadata.py` をそれぞれ実行します。

```sh
python3 scripts/extract-objc-metadata.py /path/to/GooglePhotos /tmp/main-methods.json
python3 scripts/extract-objc-metadata.py /path/to/framework-executable /tmp/framework-methods.json
```

出力の対象 class / selector と encoding を `compatibility-contracts.json` に照合し、元バイナリの SHA-256 も確認してください。parser の対象範囲は[抽出の説明](objc/README.md#抽出範囲と限界)に記載しています。
