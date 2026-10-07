package sqlite

import (
	"database/sql"
	"fmt"

	"3dprint-calc/core/models"
	"3dprint-calc/core/store"
)

var (
	_ store.Users     = (*usersRepo)(nil)
	_ store.Customers = (*customersRepo)(nil)
	_ store.Orders    = (*ordersRepo)(nil)
)

// Users возвращает репозиторий пользователей.
func (d *DB) Users() store.Users { return &usersRepo{d.sql} }

// Customers возвращает репозиторий заказчиков.
func (d *DB) Customers() store.Customers { return &customersRepo{d.sql} }

// Orders возвращает репозиторий заказов.
func (d *DB) Orders() store.Orders { return &ordersRepo{d.sql} }

type usersRepo struct{ db *sql.DB }
type customersRepo struct{ db *sql.DB }
type ordersRepo struct{ db *sql.DB }

// Create создаёт пользователя (PasswordHash — готовый PHC-хэш из core/auth).
func (r *usersRepo) Create(u models.User) (int64, error) {
	res, err := r.db.Exec(`INSERT INTO users (login, pass_argon2, role, locale) VALUES (?, ?, ?, ?)`,
		u.Login, u.PasswordHash, string(u.Role), u.Locale)
	if err != nil {
		return 0, fmt.Errorf("ERR_USER_CREATE: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// GetByLogin возвращает пользователя по логину.
func (r *usersRepo) GetByLogin(login string) (models.User, error) {
	var u models.User
	var role string
	err := r.db.QueryRow(`SELECT id, login, pass_argon2, role, locale FROM users WHERE login=?`, login).
		Scan(&u.ID, &u.Login, &u.PasswordHash, &role, &u.Locale)
	if err != nil {
		return u, fmt.Errorf("ERR_AUTH_DENIED: %w", err)
	}
	u.Role = models.Role(role)
	return u, nil
}

// List возвращает всех пользователей (без хэшей — наружу хэш не отдаём).
func (r *usersRepo) List() ([]models.User, error) {
	rows, err := r.db.Query(`SELECT id, login, role, locale FROM users ORDER BY login`)
	if err != nil {
		return nil, fmt.Errorf("ERR_USER_LIST: %w", err)
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		var u models.User
		var role string
		if err := rows.Scan(&u.ID, &u.Login, &role, &u.Locale); err != nil {
			return nil, fmt.Errorf("ERR_USER_LIST: %w", err)
		}
		u.Role = models.Role(role)
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetRole меняет роль.
func (r *usersRepo) SetRole(id int64, role models.Role) error {
	if role != models.RoleAdmin && role != models.RoleManager && role != models.RoleViewer {
		return fmt.Errorf("ERR_USER_ROLE: неизвестная роль %q", role)
	}
	if _, err := r.db.Exec(`UPDATE users SET role=? WHERE id=?`, string(role), id); err != nil {
		return fmt.Errorf("ERR_USER_ROLE: %w", err)
	}
	return nil
}

// SetPassword меняет пароль (phc — готовый хэш).
func (r *usersRepo) SetPassword(id int64, phc string) error {
	if _, err := r.db.Exec(`UPDATE users SET pass_argon2=? WHERE id=?`, phc, id); err != nil {
		return fmt.Errorf("ERR_USER_PASSWORD: %w", err)
	}
	return nil
}

// Count возвращает число пользователей (для сида админа и гейта логина).
func (r *usersRepo) Count() (int, error) {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("ERR_USER_COUNT: %w", err)
	}
	return n, nil
}

// Delete удаляет пользователя. Guards (не себя, не последнего админа) — в App.
func (r *usersRepo) Delete(id int64) error {
	if _, err := r.db.Exec(`DELETE FROM users WHERE id=?`, id); err != nil {
		return fmt.Errorf("ERR_USER_DELETE: %w", err)
	}
	return nil
}

// Save создаёт (ID=0) или обновляет заказчика.
func (r *customersRepo) Save(c models.Customer) (int64, error) {
	if c.ID > 0 {
		if _, err := r.db.Exec(`UPDATE customers SET name=?, contact=?, currency=?, note=? WHERE id=?`,
			c.Name, c.Contact, c.Currency, c.Note, c.ID); err != nil {
			return 0, fmt.Errorf("ERR_CUSTOMER_SAVE: %w", err)
		}
		return c.ID, nil
	}
	res, err := r.db.Exec(`INSERT INTO customers (name, contact, currency, note) VALUES (?, ?, ?, ?)`,
		c.Name, c.Contact, c.Currency, c.Note)
	if err != nil {
		return 0, fmt.Errorf("ERR_CUSTOMER_SAVE: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// List возвращает всех заказчиков.
func (r *customersRepo) List() ([]models.Customer, error) {
	rows, err := r.db.Query(`SELECT id, name, contact, currency, note FROM customers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("ERR_CUSTOMER_LIST: %w", err)
	}
	defer rows.Close()
	var out []models.Customer
	for rows.Next() {
		var c models.Customer
		if err := rows.Scan(&c.ID, &c.Name, &c.Contact, &c.Currency, &c.Note); err != nil {
			return nil, fmt.Errorf("ERR_CUSTOMER_LIST: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Delete удаляет заказчика. Блок: есть заказы.
func (r *customersRepo) Delete(id int64) error {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM orders WHERE customer_id=?`, id).Scan(&n); err != nil {
		return fmt.Errorf("ERR_CUSTOMER_DELETE: %w", err)
	}
	if n > 0 {
		return fmt.Errorf("ERR_CUSTOMER_USED: %d заказов ссылаются на заказчика", n)
	}
	if _, err := r.db.Exec(`DELETE FROM customers WHERE id=?`, id); err != nil {
		return fmt.Errorf("ERR_CUSTOMER_DELETE: %w", err)
	}
	return nil
}

// orderStatuses — допустимые статусы заказа.
var orderStatuses = map[string]bool{
	"draft": true, "quoted": true, "in_work": true,
	"done": true, "paid": true, "cancelled": true,
}

// Create создаёт заказ в статусе draft.
func (r *ordersRepo) Create(o models.Order) (int64, error) {
	if o.Status == "" {
		o.Status = "draft"
	}
	if !orderStatuses[o.Status] {
		return 0, fmt.Errorf("ERR_ORDER_STATUS: неизвестный статус %q", o.Status)
	}
	if o.Currency == "" {
		o.Currency = "RUB"
	}
	res, err := r.db.Exec(`INSERT INTO orders (customer_id, number, currency, fx_rate, margin_profit,
		margin_dev, vat, status, total_base, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		nullInt(o.CustomerID), o.Number, o.Currency, o.FxRate, o.MarginProfit, o.MarginDev,
		o.VAT, o.Status, o.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("ERR_ORDER_CREATE: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// Get возвращает заказ.
func (r *ordersRepo) Get(id int64) (models.Order, error) {
	var o models.Order
	var custID sql.NullInt64
	err := r.db.QueryRow(`SELECT id, customer_id, number, currency, fx_rate, margin_profit,
		margin_dev, vat, status, total_base, created_at FROM orders WHERE id=?`, id).
		Scan(&o.ID, &custID, &o.Number, &o.Currency, &o.FxRate, &o.MarginProfit,
			&o.MarginDev, &o.VAT, &o.Status, &o.TotalBase, &o.CreatedAt)
	if err != nil {
		return o, fmt.Errorf("ERR_ORDER_GET: %w", err)
	}
	o.CustomerID = custID.Int64
	return o, nil
}

// List возвращает все заказы (новые первыми).
func (r *ordersRepo) List() ([]models.Order, error) {
	rows, err := r.db.Query(`SELECT id, customer_id, number, currency, fx_rate, margin_profit,
		margin_dev, vat, status, total_base, created_at FROM orders ORDER BY id DESC`)
	if err != nil {
		return nil, fmt.Errorf("ERR_ORDER_LIST: %w", err)
	}
	defer rows.Close()
	var out []models.Order
	for rows.Next() {
		var o models.Order
		var custID sql.NullInt64
		if err := rows.Scan(&o.ID, &custID, &o.Number, &o.Currency, &o.FxRate, &o.MarginProfit,
			&o.MarginDev, &o.VAT, &o.Status, &o.TotalBase, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("ERR_ORDER_LIST: %w", err)
		}
		o.CustomerID = custID.Int64
		out = append(out, o)
	}
	return out, rows.Err()
}

// Update правит шапку заказа (статус — только через SetStatus).
func (r *ordersRepo) Update(o models.Order) error {
	if o.ID == 0 {
		return fmt.Errorf("ERR_ORDER_SAVE: пустой id")
	}
	if _, err := r.db.Exec(`UPDATE orders SET customer_id=?, currency=?, fx_rate=?,
		margin_profit=?, margin_dev=?, vat=? WHERE id=?`,
		nullInt(o.CustomerID), o.Currency, o.FxRate, o.MarginProfit, o.MarginDev, o.VAT, o.ID); err != nil {
		return fmt.Errorf("ERR_ORDER_SAVE: %w", err)
	}
	return nil
}

// Delete удаляет заказ каскадом: позиции + оплаты. Подтверждение — в UI.
func (r *ordersRepo) Delete(id int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("ERR_ORDER_DELETE: %w", err)
	}
	defer tx.Rollback()
	for _, q := range []string{
		`DELETE FROM payments WHERE order_id=?`,
		`DELETE FROM order_items WHERE order_id=?`,
		`DELETE FROM orders WHERE id=?`,
	} {
		if _, err := tx.Exec(q, id); err != nil {
			return fmt.Errorf("ERR_ORDER_DELETE: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ERR_ORDER_DELETE: %w", err)
	}
	return nil
}

// SetStatus меняет статус заказа.
func (r *ordersRepo) SetStatus(id int64, status string) error {
	if !orderStatuses[status] {
		return fmt.Errorf("ERR_ORDER_STATUS: неизвестный статус %q", status)
	}
	if _, err := r.db.Exec(`UPDATE orders SET status=? WHERE id=?`, status, id); err != nil {
		return fmt.Errorf("ERR_ORDER_STATUS: %w", err)
	}
	return nil
}

// AddItem добавляет позицию и пересчитывает итог заказа.
func (r *ordersRepo) AddItem(it models.OrderItem) (int64, error) {
	res, err := r.db.Exec(`INSERT INTO order_items (order_id, job_id, title, qty, cost, price)
		VALUES (?, ?, ?, ?, ?, ?)`,
		it.OrderID, nullInt(it.JobID), it.Title, it.Qty, it.Cost, it.Price)
	if err != nil {
		return 0, fmt.Errorf("ERR_ORDER_ITEM: %w", err)
	}
	id, _ := res.LastInsertId()
	if _, err := r.RecalcTotal(it.OrderID); err != nil {
		return 0, err
	}
	return id, nil
}

// Items возвращает позиции заказа.
func (r *ordersRepo) Items(orderID int64) ([]models.OrderItem, error) {
	rows, err := r.db.Query(`SELECT id, order_id, job_id, title, qty, cost, price
		FROM order_items WHERE order_id=? ORDER BY id`, orderID)
	if err != nil {
		return nil, fmt.Errorf("ERR_ORDER_ITEM: %w", err)
	}
	defer rows.Close()
	var out []models.OrderItem
	for rows.Next() {
		var it models.OrderItem
		var jobID sql.NullInt64
		if err := rows.Scan(&it.ID, &it.OrderID, &jobID, &it.Title, &it.Qty, &it.Cost, &it.Price); err != nil {
			return nil, fmt.Errorf("ERR_ORDER_ITEM: %w", err)
		}
		it.JobID = jobID.Int64
		out = append(out, it)
	}
	return out, rows.Err()
}

// AddPayment фиксирует оплату.
func (r *ordersRepo) AddPayment(p models.Payment) (int64, error) {
	if p.Amount <= 0 {
		return 0, fmt.Errorf("ERR_PAYMENT: сумма обязана быть положительной")
	}
	res, err := r.db.Exec(`INSERT INTO payments (order_id, amount, paid_at, note) VALUES (?, ?, ?, ?)`,
		p.OrderID, p.Amount, p.PaidAt, p.Note)
	if err != nil {
		return 0, fmt.Errorf("ERR_PAYMENT: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// Payments возвращает оплаты заказа.
func (r *ordersRepo) Payments(orderID int64) ([]models.Payment, error) {
	rows, err := r.db.Query(`SELECT id, order_id, amount, paid_at, note FROM payments
		WHERE order_id=? ORDER BY id`, orderID)
	if err != nil {
		return nil, fmt.Errorf("ERR_PAYMENT: %w", err)
	}
	defer rows.Close()
	var out []models.Payment
	for rows.Next() {
		var p models.Payment
		if err := rows.Scan(&p.ID, &p.OrderID, &p.Amount, &p.PaidAt, &p.Note); err != nil {
			return nil, fmt.Errorf("ERR_PAYMENT: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PaidTotal возвращает сумму оплат заказа.
func (r *ordersRepo) PaidTotal(orderID int64) (int64, error) {
	var sum sql.NullInt64
	if err := r.db.QueryRow(`SELECT SUM(amount) FROM payments WHERE order_id=?`, orderID).Scan(&sum); err != nil {
		return 0, fmt.Errorf("ERR_PAYMENT: %w", err)
	}
	return sum.Int64, nil
}

// RecalcTotal пересчитывает total_base как SUM(price*qty) и возвращает его.
func (r *ordersRepo) RecalcTotal(orderID int64) (int64, error) {
	var total sql.NullFloat64
	if err := r.db.QueryRow(`SELECT SUM(price*qty) FROM order_items WHERE order_id=?`, orderID).Scan(&total); err != nil {
		return 0, fmt.Errorf("ERR_ORDER_TOTAL: %w", err)
	}
	t := int64(total.Float64 + 0.5)
	if _, err := r.db.Exec(`UPDATE orders SET total_base=? WHERE id=?`, t, orderID); err != nil {
		return 0, fmt.Errorf("ERR_ORDER_TOTAL: %w", err)
	}
	return t, nil
}
