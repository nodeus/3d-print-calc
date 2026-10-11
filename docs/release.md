# Релизная сборка

Канонический способ — инструмент `tools/release` (версия — из `desktop/version.go`,
константа `Version`; дата прошивается ldflags в `BuildDate`):

```
go run ./tools/release -platforms windows/amd64
go run ./tools/release -platforms windows/amd64,linux/amd64 -version 1.0.0 -date 2026-10-08 -upx=false
```

Windows-exe пакуются UPX (`utils/upx.exe` в корне репо, в git не идёт):
обычно −40…−60% размера. Минусы UPX, о которых надо знать:

- некоторые антивирусы ругаются на UPX-упакованные файлы (ложные срабатывания);
- чуть более медленный холодный старт (распаковка в память);
- отладка по такому бинарнику затруднена — для диагностики собирайте с `-upx=false`.

Раскладка артефактов (в git не коммитится):

```
release/<version>/<ГГГГ-ММ-ДД>/<os>-<arch>/
  3dprint-calc(.exe)
  build-info.txt   # версия, дата, платформа, git-коммит
  SHA256SUMS
```

Вручную (без инструмента) — то же самое напрямую:

```
cd desktop
wails build -platform windows/amd64 -ldflags "-X main.BuildDate=2026-10-08"
```

Версия видна в заголовке окна и на вкладке «Информация».
GitFlow: `release/<version>` от `develop` → merge в `main` + тег `vX.Y.Z` +
back-merge в `develop`. Подписи exe и автообновления нет — обновление
пользователей идёт через backup.zip (см. `docs/updating.md`).

Перед тегом обновить `CHANGELOG.md`: только пользовательские изменения
(`feat`/`fix`/`refactor`/`docs`/`perf`), без внутреннего (`chore`/`build`/`ci`,
`test`) и служебного (мёржи, релизные коммиты). Язык записей — как в истории
коммитов релиза (см. скилл `changelog-generator`).
