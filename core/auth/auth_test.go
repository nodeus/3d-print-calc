package auth

import (
	"strings"
	"testing"
)

func TestHashVerifyRoundTrip(t *testing.T) {
	phc, err := Hash("admin123")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(phc, "$argon2id$") {
		t.Fatalf("не PHC: %.20q", phc)
	}
	if err := Verify(phc, "admin123"); err != nil {
		t.Fatalf("Verify свой: %v", err)
	}
	if err := Verify(phc, "wrong"); err == nil {
		t.Fatal("чужой пароль прошёл")
	}
	// Два хэша одного пароля различаются (случайная соль).
	phc2, _ := Hash("admin123")
	if phc == phc2 {
		t.Fatal("соль не случайна")
	}
	if err := Verify("мусор", "x"); err == nil {
		t.Fatal("мусор прошёл")
	}
}
