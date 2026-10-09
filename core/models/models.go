// Package models — доменные сущности калькулятора (без логики хранения).
package models

// Role — роль пользователя в локальной системе.
type Role string

const (
	RoleAdmin   Role = "admin"
	RoleManager Role = "manager"
	RoleViewer  Role = "viewer"
)

// MakerType — тип исполнителя: дизайнер, моделлер, печатник/оператор.
type MakerType string

const (
	MakerDesigner  MakerType = "designer"
	MakerModeller  MakerType = "modeller"
	MakerPrinterOp MakerType = "printer_op"
)

// User — локальный пользователь (менеджер). Пароль — только argon2-хэш.
type User struct {
	ID           int64
	Login        string
	PasswordHash string
	Role         Role
	Locale       string // ru | en
}

// Customer — заказчик.
type Customer struct {
	ID       int64
	Name     string
	Contact  string
	Currency string // ISO-код
	Note     string
}

// Maker — дизайнер/моделлер/печатник со ставкой.
type Maker struct {
	ID       int64
	Type     MakerType
	Name     string
	RatePerH int64 // минорные единицы/час
	Currency string
}

// Material — справочник пластика (код уникален).
type Material struct {
	ID                  int64
	Code                string // напр. PETG-NIT-W4
	Type                string // PLA | PETG | ABS | ...
	Vendor              string
	ColorHex            string
	Density             float64 // г/см³, напр. 1.27
	Diameter            float64 // мм, напр. 1.75
	DefaultSpoolWeightG float64 // вес новых катушек, г (дефолт 1000)
	Note                string
	BoughtAt            string // дата закупа, '' = отсутствует
	Archived            bool   // архивные скрыты из остатков и селекта калькулятора
}

// Spool — закупочная партия (катушка) с ценой и остатком.
type Spool struct {
	ID          int64
	MaterialID  int64
	Price       int64 // минорные единицы
	Currency    string
	FxRate      float64
	BoughtAt    string // ISO-дата
	WeightInitG float64
	WeightLeftG float64
	Archived    bool
}

// Printer — принтер со своим расчётом амортизации.
type Printer struct {
	ID                int64
	Name              string
	Model             string
	PurchasePrice     int64
	Currency          string
	CommissioningDate string // ISO-дата
	LifeYears         int
	HoursPerDay       float64
	CalendarMode      string // calendar | workdays
	AmortMethod       string // calendar | workdays | resource
	PowerKW           float64
	MaintPerHour      int64
	ResourceHours     float64
	MoonrakerURL      string // http://host:7125, пусто = не настроен
}

// Job — печать/работа со снапшотом расчёта.
type Job struct {
	ID              int64
	Filename        string
	StartAt         string
	EndAt           string
	PrinterID       int64
	SpoolID         int64
	Status          string
	Origin          string // import | manual | moonraker (удаление из UI — только manual)
	Edited          int    // 1 = импортированная правилась вручную (бейдж «изменена»)
	OrigSnapshot    string // первозданный снапшот+поля для отката (JSON)
	PrintSec        int64
	TotalSec        int64
	EstimatedSec    int64
	SizeBytes       int64
	FilamentTotalMm float64
	WeightG         float64
	LengthM         float64
	Slicer          string
	CostSnapshot    string // JSON
}

// Order — заказ клиента.
type Order struct {
	ID            int64
	CustomerID    int64
	Number        string // номер по шаблону (ordnum); пусто у старых — фолбэк #id
	Title         string // название («Клиент — ДД.ММ.ГГГГ» по умолчанию)
	Currency      string
	FxRate        float64
	MarginProfit  float64 // legacy, больше не редактируется (посчитано в калькуляторе)
	MarginDev     float64 // legacy, см. выше
	VAT           float64 // legacy, см. выше
	DiscountPct   float64 // скидка % от суммы позиций (применяется первой)
	DiscountFixed int64   // скидка фикс. суммой (применяется второй, минорные)
	Status        string  // draft | quoted | in_work | done | paid | cancelled
	TotalBase     int64
	CreatedAt     string // ISO-дата
}

// OrderItem — позиция заказа (qty может быть дробным).
type OrderItem struct {
	ID         int64
	OrderID    int64
	JobID      int64
	Title      string
	Qty        float64
	Cost       int64 // минорные единицы
	Price      int64 // минорные единицы
	PaidManual bool  // ручная отметка «Оплачен» (сумм не меняет)
}

// Payment — оплата по заказу (целиком или по позиции).
type Payment struct {
	ID        int64
	OrderID   int64
	OrderItem int64 // 0 = весь заказ, иначе позиция
	Amount    int64 // минорные единицы
	PaidAt    string
	Note      string
}
