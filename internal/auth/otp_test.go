package auth

import (
	"testing"
)

func TestOtpGenerateIsNumeric6Digits(t *testing.T) {
	g := NewOtpGenerator()
	for i := 0; i < 50; i++ {
		otp, err := g.Generate()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if len(otp) != 6 {
			t.Fatalf("expected 6 digits, got %q", otp)
		}
		for _, r := range otp {
			if r < '0' || r > '9' {
				t.Fatalf("non-numeric OTP %q", otp)
			}
		}
	}
}

func TestOtpGenerateWithinLaravelRange(t *testing.T) {
	// Laravel uses random_int(100000, 999999): no leading zeros, >= 100000.
	g := NewOtpGenerator()
	for i := 0; i < 200; i++ {
		otp, _ := g.Generate()
		if otp < "100000" || otp > "999999" {
			t.Fatalf("OTP %q outside Laravel 100000-999999 range", otp)
		}
		if otp[0] == '0' {
			t.Fatalf("unexpected leading zero %q", otp)
		}
	}
}

func TestOtpGenerateDiversity(t *testing.T) {
	// OTP space is 900000 values; a strict uniqueness test over 200 draws can
	// legitimately collide (~2%). Assert we see a spread of outputs instead.
	g := NewOtpGenerator()
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		otp, _ := g.Generate()
		seen[otp] = true
	}
	if len(seen) < 2 {
		t.Fatal("expected a spread of OTP outputs")
	}
}
