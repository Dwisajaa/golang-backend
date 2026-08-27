package auth

import (
	"crypto/rand"
	"math/big"
)

// OtpGenerator produces the numeric code exactly like Laravel's
// `random_int(100000, 999999)`: 6 digits in 100000–999999, via crypto/rand.
type OtpGenerator struct{}

func NewOtpGenerator() *OtpGenerator { return &OtpGenerator{} }

// otpRange is 100000..999999 inclusive → 900000 possible values.
const (
	otpMin = 100000
	otpMax = 999999
)

func (g *OtpGenerator) Generate() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(otpMax-otpMin+1)))
	if err != nil {
		return "", err
	}
	return itoaPad6(int(otpMin + n.Int64())), nil
}

// itoaPad6 formats without leading-zero concerns (range guarantees 6 digits),
// mirroring random_int's fixed-width result.
func itoaPad6(n int) string {
	if n == 0 {
		return "000000"
	}
	var buf [6]byte
	for i := 5; i >= 0; i-- {
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[:])
}
