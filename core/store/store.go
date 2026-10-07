// Package store — Repository-ИНТЕРФЕЙСЫ домена. SQL запрещён.
//
// Реализация — в store-sqlite/. Будущий сервер/Postgres реализует те же
// интерфейсы без изменения core.
package store

import "3dprint-calc/core/models"

// Materials — справочник пластика и партий.
type Materials interface {
	UpsertByCode(m models.Material) (int64, error)
	GetByCode(code string) (models.Material, error)
	List() ([]models.Material, error)
	AddSpool(s models.Spool) (int64, error)
}

// Printers — парк принтеров.
type Printers interface {
	Save(p models.Printer) (int64, error)
	List() ([]models.Printer, error)
}

// Jobs — история печати. Импорт идемпотентен по filename+start_at.
type Jobs interface {
	Upsert(j models.Job) (int64, error)
	List(limit int) ([]models.Job, error)
}
