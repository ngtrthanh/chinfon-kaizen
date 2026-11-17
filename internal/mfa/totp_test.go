package mfa

import (
	"strings"
	"testing"
	"time"
)

func TestTOTPRFC6238SHA1Vectors(t *testing.T) {
	// RFC 6238 test secret for SHA1: ASCII "12345678901234567890".
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, c := range cases {
		got, err := CodeAt(secret, c.unix/Period)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("t=%d: got %s want %s", c.unix, got, c.want)
		}
	}
}

func TestValidateAndReplayWindow(t *testing.T) {
	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	code, _ := CodeAt(secret, now.Unix()/Period)
	counter, ok := Validate(secret, code, now, 1)
	if !ok || counter != now.Unix()/Period {
		t.Fatalf("valid code rejected: counter=%d ok=%v", counter, ok)
	}
	if _, ok := Validate(secret, "000000", now, 1); ok {
		t.Fatal("wrong code accepted")
	}
	if _, ok := Validate(secret, code, now.Add(5*time.Minute), 1); ok {
		t.Fatal("old code accepted outside window")
	}
}

func TestProvisioningURI(t *testing.T) {
	uri := ProvisioningURI("Chinfon Kaizen 360", "P81-0001", "JBSWY3DPEHPK3PXP")
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Fatalf("bad URI: %s", uri)
	}
	if !strings.Contains(uri, "secret=JBSWY3DPEHPK3PXP") || !strings.Contains(uri, "issuer=Chinfon") {
		t.Fatalf("missing params: %s", uri)
	}
}

func TestSecretEncryption(t *testing.T) {
	key := make([]byte, KeyBytes)
	for i := range key {
		key[i] = byte(i + 1)
	}
	secret, _ := NewSecret()
	enc, err := Encrypt(key, []byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(enc, secret) {
		t.Fatal("ciphertext contains plaintext secret")
	}
	dec, err := Decrypt(key, enc)
	if err != nil {
		t.Fatal(err)
	}
	if string(dec) != secret {
		t.Fatalf("round trip mismatch: %q", dec)
	}
	key[0] ^= 0xff
	if _, err := Decrypt(key, enc); err == nil {
		t.Fatal("wrong key decrypted secret")
	}
}
