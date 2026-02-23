package auth

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("super-secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hash == "" || hash == "super-secret" {
		t.Fatalf("unexpected hash value: %q", hash)
	}

	if err := CheckPassword(hash, "super-secret"); err != nil {
		t.Fatalf("check password failed: %v", err)
	}
}

func TestCheckPasswordWrongPassword(t *testing.T) {
	hash, err := HashPassword("super-secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if err := CheckPassword(hash, "wrong-password"); err == nil {
		t.Fatal("expected error for wrong password")
	}
}
