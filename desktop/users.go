package main

import (
	"errors"

	"3dprint-calc/core/auth"
	"3dprint-calc/core/models"
)

// Пользователи и сессии. Сессия — текущий пользователь процесса.
// Роли: admin > manager > viewer. Чтение — всем вошедшим, запись — manager+,
// управление пользователями — admin.

// UserDTO — наружу без хэша.
type UserDTO struct {
	ID     int64  `json:"id"`
	Login  string `json:"login"`
	Role   string `json:"role"`
	Locale string `json:"locale"`
}

func toUserDTO(u models.User) UserDTO {
	return UserDTO{ID: u.ID, Login: u.Login, Role: string(u.Role), Locale: u.Locale}
}

// Login проверяет пароль и открывает сессию.
func (a *App) Login(login, password string) (UserDTO, error) {
	u, err := a.db.Users().GetByLogin(login)
	if err != nil {
		return UserDTO{}, err
	}
	if err := auth.Verify(u.PasswordHash, password); err != nil {
		return UserDTO{}, err
	}
	a.currentUser = &u
	return toUserDTO(u), nil
}

// Logout закрывает сессию.
func (a *App) Logout() {
	a.currentUser = nil
}

// CurrentUser возвращает текущую сессию (nil — гость).
func (a *App) CurrentUser() *UserDTO {
	if a.currentUser == nil {
		return nil
	}
	dto := toUserDTO(*a.currentUser)
	return &dto
}

var errDenied = errors.New("ERR_DENIED: недостаточно прав")

// need проверяет роль текущей сессии. Пустая сессия — гость (только чтение).
func (a *App) need(want models.Role) error {
	if a.currentUser == nil {
		return errDenied
	}
	got := a.currentUser.Role
	if want == models.RoleManager && (got == models.RoleManager || got == models.RoleAdmin) {
		return nil
	}
	if want == models.RoleAdmin && got == models.RoleAdmin {
		return nil
	}
	if want == models.RoleViewer {
		return nil
	}
	return errDenied
}

// ListUsers — список пользователей (admin).
func (a *App) ListUsers() ([]UserDTO, error) {
	if err := a.need(models.RoleAdmin); err != nil {
		return nil, err
	}
	list, err := a.db.Users().List()
	if err != nil {
		return nil, err
	}
	out := make([]UserDTO, 0, len(list))
	for _, u := range list {
		out = append(out, toUserDTO(u))
	}
	return out, nil
}

// CreateUser — новый пользователь (admin). Роль по умолчанию manager.
func (a *App) CreateUser(login, password, role string) (int64, error) {
	if err := a.need(models.RoleAdmin); err != nil {
		return 0, err
	}
	if login == "" || len(password) < 4 {
		return 0, errors.New("ERR_USER_CREATE: логин пуст или пароль короче 4")
	}
	r := models.Role(role)
	if r != models.RoleAdmin && r != models.RoleManager && r != models.RoleViewer {
		r = models.RoleManager
	}
	phc, err := auth.Hash(password)
	if err != nil {
		return 0, err
	}
	return a.db.Users().Create(models.User{Login: login, PasswordHash: phc, Role: r, Locale: "ru"})
}

// SetUserRole — смена роли (admin).
func (a *App) SetUserRole(id int64, role string) error {
	if err := a.need(models.RoleAdmin); err != nil {
		return err
	}
	return a.db.Users().SetRole(id, models.Role(role))
}

// DeleteUser — удалить пользователя (admin). Нельзя себя и последнего админа.
func (a *App) DeleteUser(id int64) error {
	if err := a.need(models.RoleAdmin); err != nil {
		return err
	}
	if a.currentUser != nil && a.currentUser.ID == id {
		return errors.New("ERR_USER_DELETE: нельзя удалить себя")
	}
	list, err := a.db.Users().List()
	if err != nil {
		return err
	}
	admins := 0
	targetIsAdmin := false
	for _, u := range list {
		if u.Role == models.RoleAdmin {
			admins++
			if u.ID == id {
				targetIsAdmin = true
			}
		}
	}
	if targetIsAdmin && admins <= 1 {
		return errors.New("ERR_USER_DELETE: нельзя удалить последнего админа")
	}
	return a.db.Users().Delete(id)
}

// ChangeMyPassword — смена своего пароля по старому.
func (a *App) ChangeMyPassword(old, pw string) error {
	if a.currentUser == nil {
		return errDenied
	}
	if len(pw) < 4 {
		return errors.New("ERR_USER_PASSWORD: пароль короче 4")
	}
	u, err := a.db.Users().GetByLogin(a.currentUser.Login)
	if err != nil {
		return err
	}
	if err := auth.Verify(u.PasswordHash, old); err != nil {
		return err
	}
	phc, err := auth.Hash(pw)
	if err != nil {
		return err
	}
	return a.db.Users().SetPassword(u.ID, phc)
}
