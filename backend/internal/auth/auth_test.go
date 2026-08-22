package auth

import (
	"strings"
	"testing"
	"time"
)

func TestHashAndCheckPassword(t *testing.T) {
	const pw = "helyes jelszó 123"

	h, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if strings.Contains(h, pw) {
		t.Fatal("hash contains the plaintext password")
	}
	if err := CheckPassword(h, pw); err != nil {
		t.Errorf("correct password rejected: %v", err)
	}
	if err := CheckPassword(h, pw+"x"); err != ErrInvalidCredentials {
		t.Errorf("wrong password: want ErrInvalidCredentials, got %v", err)
	}
	if err := CheckPassword("not-a-bcrypt-hash", pw); err != ErrInvalidCredentials {
		t.Errorf("malformed hash: want ErrInvalidCredentials, got %v", err)
	}
}

func TestHashPasswordIsSalted(t *testing.T) {
	a, _ := HashPassword("ugyanaz a jelszó")
	b, _ := HashPassword("ugyanaz a jelszó")
	if a == b {
		t.Error("identical passwords produced identical hashes — salt is missing")
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name string
		pw   string
		ok   bool
	}{
		{"too short", "rövid", false},
		{"exactly minimum", strings.Repeat("a", MinPasswordLength), true},
		{"long enough", "elég hosszú jelszó", true},
		// Length is counted in runes: an accented password must not be penalised
		// for its UTF-8 byte count.
		{"accented at minimum", strings.Repeat("ő", MinPasswordLength), true},
		{"absurdly long", strings.Repeat("a", 2000), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.pw)
			if tt.ok && err != nil {
				t.Errorf("want accepted, got %v", err)
			}
			if !tt.ok && err == nil {
				t.Error("want rejected, got nil")
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"  Anna@Example.COM ", "anna@example.com", true},
		{"a@b.co", "a@b.co", true},
		{"", "", false},
		{"not-an-email", "", false},
		{"@example.com", "", false},
	}
	for _, tt := range tests {
		got, err := NormalizeEmail(tt.in)
		if tt.ok {
			if err != nil {
				t.Errorf("NormalizeEmail(%q): unexpected error %v", tt.in, err)
			} else if got != tt.want {
				t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.in, got, tt.want)
			}
		} else if err == nil {
			t.Errorf("NormalizeEmail(%q): want error, got %q", tt.in, got)
		}
	}
}

func TestSessionTokensAreUniqueAndHashed(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		tok, hash, err := NewSessionToken()
		if err != nil {
			t.Fatalf("NewSessionToken: %v", err)
		}
		if seen[tok] {
			t.Fatal("duplicate session token generated")
		}
		seen[tok] = true

		if tok == hash {
			t.Fatal("stored hash equals the plaintext token")
		}
		if HashToken(tok) != hash {
			t.Fatal("HashToken disagrees with NewSessionToken")
		}
		if len(hash) != 64 {
			t.Fatalf("want 64 hex chars of SHA-256, got %d", len(hash))
		}
	}
}

func TestSessionExpiryIsInTheFuture(t *testing.T) {
	exp := SessionExpiry()
	if !exp.After(time.Now().UTC()) {
		t.Fatal("session expiry is not in the future")
	}
	if exp.Sub(time.Now().UTC()) > SessionTTL+time.Minute {
		t.Fatal("session expiry exceeds SessionTTL")
	}
}
