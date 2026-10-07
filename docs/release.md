# Релизная сборка

Канонический способ — инструмент `tools/release` (версия — из `desktop/version.go`,
константа `Version`; дата прошивается ldflags в `BuildDate`):

```
go run ./tools/release -platforms windows/amd64
go run ./tools/release -platforms windows/amd64,linux/amd64 -version 0.2.0 -date 2026-10-08
```

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
