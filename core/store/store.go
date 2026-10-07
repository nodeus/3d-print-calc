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
	Update(m models.Material) error // всё кроме кода (код — ключ и сидит в снапшотах)
	Delete(id int64) error          // блок, если есть катушки
	AddSpool(s models.Spool) (int64, error)
	ListSpools(materialID int64) ([]models.Spool, error)
	UpdateSpool(s models.Spool) error                                     // цена/валюта/остаток партии
	SetMaterialPrice(materialID, priceMinor int64, currency string) error // цена первой катушки (создаст при отсутствии)
}

// Printers — парк принтеров.
type Printers interface {
	Save(p models.Printer) (int64, error)
	List() ([]models.Printer, error)
	Update(p models.Printer) error // все поля по id
	Delete(id int64) error         // блок, если есть работы
	SetMoonrakerURL(id int64, url string) error
}

// Jobs — история печати. Импорт идемпотентен по filename+start_at.
type Jobs interface {
	Upsert(j models.Job) (int64, error)
	List(limit int) ([]models.Job, error)
	DeleteJob(id int64) error                         // только origin=manual
	UpdateManual(j models.Job, material string) error // правка + автопересчёт, только manual
}

// Users — локальные пользователи и роли.
type Users interface {
	Create(u models.User) (int64, error) // PasswordHash — готовый PHC-хэш
	GetByLogin(login string) (models.User, error)
	List() ([]models.User, error)
	SetRole(id int64, role models.Role) error
	SetPassword(id int64, phc string) error
	Delete(id int64) error // guards (не себя/не последнего админа) — в App
	Count() (int, error)
}

// Customers — заказчики.
type Customers interface {
	Save(c models.Customer) (int64, error) // insert (ID=0) или update
	List() ([]models.Customer, error)
	Delete(id int64) error // блок, если есть заказы
}

// Orders — заказы, позиции, оплаты.
type Orders interface {
	Create(o models.Order) (int64, error)
	Get(id int64) (models.Order, error)
	List() ([]models.Order, error)
	Update(o models.Order) error // клиент/валюта/маржи/НДС (статус — через SetStatus)
	Delete(id int64) error       // каскад: позиции + оплаты
	SetStatus(id int64, status string) error
	AddItem(it models.OrderItem) (int64, error)
	Items(orderID int64) ([]models.OrderItem, error)
	AddPayment(p models.Payment) (int64, error)
	Payments(orderID int64) ([]models.Payment, error)
	PaidTotal(orderID int64) (int64, error)
	RecalcTotal(orderID int64) (int64, error) // price*qty → orders.total_base
}
