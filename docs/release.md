# Релизная сборка

Версия — константа `Version` в `desktop/version.go`.
Дата сборки прошивается ldflags (иначе `dev`):

```
cd desktop
wails build -ldflags "-X main.BuildDate=2026-10-08"
```

Кросс-платформа (из той же папки):

```
wails build -platform windows/amd64 -ldflags "-X main.BuildDate=2026-10-08"
wails build -platform linux/amd64 -ldflags "-X main.BuildDate=2026-10-08"
wails build -platform darwin/universal -ldflags "-X main.BuildDate=2026-10-08"
```

Версия видна в заголовке окна и на вкладке «Информация».
GitFlow: `release/<version>` от `develop` → merge в `main` + тег `vX.Y.Z` +
back-merge в `develop`. Подписи exe и автообновления нет — обновление
пользователей идёт через backup.zip (см. `docs/updating.md`).
