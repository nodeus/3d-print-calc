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
	ID       int64
	Code     string // напр. PETG-NIT-W4
	Type     string // PLA | PETG | ABS | ...
	Vendor   string
	ColorHex string
	Density  float64 // г/см³, напр. 1.27
	Diameter float64 // мм, напр. 1.75
	Note     string
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
}

// Job — печать/работа со снапшотом расчёта.
type Job struct {
	ID           int64
	Filename     string
	StartAt      string
	PrinterID    int64
	SpoolID      int64
	Status       string
	PrintSec     int64
	TotalSec     int64
	WeightG      float64
	LengthM      float64
	CostSnapshot string // JSON
}

// Order — заказ клиента.
type Order struct {
	ID           int64
	CustomerID   int64
	Currency     string
	FxRate       float64
	MarginProfit float64
	MarginDev    float64
	VAT          float64
	Status       string
	TotalBase    int64
}
