// Package auth — локальная аутентификация: хэширование паролей (argon2id)
// и проверка. Токенов/сессий нет: сессия — текущий пользователь в App.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	saltLen    = 16
	argonMem   = 64 * 1024
	argonIter  = 1
	argonPar   = 4
	argonKeyLn = 32
)

// Hash возвращает PHC-строку $argon2id$... для хранения в users.pass_argon2.
func Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("ERR_AUTH_SALT: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonIter, argonMem, argonPar, argonKeyLn)
	return fmt.Sprintf("$argon2id$m=%d,t=%d,p=%d$%s$%s",
		argonMem, argonIter, argonPar,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash)), nil
}

// Verify сверяет пароль с PHC-строкой. Не различает «нет пользователя»
// и «неверный пароль» снаружи — обе ошибки как ERR_AUTH_DENIED.
func Verify(phc, password string) error {
	parts := strings.Split(phc, "$")
	// $argon2id$[v=..$]params$salt$hash — 5 частей без версии, 6 с версией.
	var saltB64, hashB64 string
	switch {
	case len(parts) == 5 && parts[1] == "argon2id":
		saltB64, hashB64 = parts[3], parts[4]
	case len(parts) == 6 && parts[1] == "argon2id":
		saltB64, hashB64 = parts[4], parts[5]
	default:
		return fmt.Errorf("ERR_AUTH_DENIED: неверный формат хэша")
	}
	salt, err := base64.RawStdEncoding.DecodeString(saltB64)
	if err != nil {
		return fmt.Errorf("ERR_AUTH_DENIED: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(hashB64)
	if err != nil {
		return fmt.Errorf("ERR_AUTH_DENIED: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt, argonIter, argonMem, argonPar, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return fmt.Errorf("ERR_AUTH_DENIED: неверный логин или пароль")
	}
	return nil
}
