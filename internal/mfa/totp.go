package mfa

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	Period      = 30
	Digits      = 6
	SecretBytes = 20
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func NewSecret() (string, error) {
	b := make([]byte, SecretBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b32.EncodeToString(b), nil
}

func normalizeSecret(secret string) string {
	s := strings.ToUpper(strings.TrimSpace(secret))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimRight(s, "=")
	return s
}

func CodeAt(secret string, counter int64) (string, error) {
	secret = normalizeSecret(secret)
	key, err := b32.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("invalid TOTP secret: %w", err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(counter))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", Digits, bin%1_000_000), nil
}

func Validate(secret, code string, now time.Time, window int) (int64, bool) {
	code = strings.TrimSpace(code)
	if len(code) != Digits {
		return 0, false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	if window < 0 {
		window = 0
	}
	current := now.Unix() / Period
	for i := -window; i <= window; i++ {
		counter := current + int64(i)
		want, err := CodeAt(secret, counter)
		if err != nil {
			return 0, false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return counter, true
		}
	}
	return 0, false
}

func ProvisioningURI(issuer, account, secret string) string {
	label := url.QueryEscape(issuer + ":" + account)
	label = strings.ReplaceAll(label, "+", "%20")
	v := url.Values{}
	v.Set("secret", normalizeSecret(secret))
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", fmt.Sprintf("%d", Digits))
	v.Set("period", fmt.Sprintf("%d", Period))
	return "otpauth://totp/" + label + "?" + v.Encode()
}

func RemainingSeconds(now time.Time) int {
	return Period - int(now.Unix()%Period)
}
