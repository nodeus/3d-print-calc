// Package sqlite — реализация core/store.Repository на SQLite.
//
// Драйвер: modernc.org/sqlite (pure Go, без CGO) — обязательное требование
// для кросс-билда (см. AGENTS.md). Схема — только через migrations/.
package sqlite
