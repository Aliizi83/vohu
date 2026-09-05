package crypto_test

import (
	"testing"

	"github.com/Aliizi83/vohu/pkg/crypto"
)

const testKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" // 32 zero bytes, base64

func TestEncryptDecrypt_RoundTrips(t *testing.T) {
	box, err := crypto.NewBox(testKey)
	if err != nil {
		t.Fatalf("NewBox failed: %v", err)
	}

	ciphertext, err := box.Encrypt("super-secret-password")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if ciphertext == "super-secret-password" {
		t.Fatal("expected ciphertext to differ from plaintext")
	}

	plaintext, err := box.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if plaintext != "super-secret-password" {
		t.Fatalf("expected round-tripped plaintext %q, got %q", "super-secret-password", plaintext)
	}
}

func TestEncrypt_ProducesDifferentCiphertextEachTime(t *testing.T) {
	box, err := crypto.NewBox(testKey)
	if err != nil {
		t.Fatalf("NewBox failed: %v", err)
	}

	first, err := box.Encrypt("same-input")
	if err != nil {
		t.Fatalf("first Encrypt failed: %v", err)
	}
	second, err := box.Encrypt("same-input")
	if err != nil {
		t.Fatalf("second Encrypt failed: %v", err)
	}

	if first == second {
		t.Fatal("expected distinct nonces to produce distinct ciphertexts for identical plaintext")
	}
}

func TestNewBox_RejectsWrongKeySize(t *testing.T) {
	_, err := crypto.NewBox("dG9vc2hvcnQ=") // "tooshort" base64-encoded, not 32 bytes
	if err != crypto.ErrInvalidKeySize {
		t.Fatalf("expected ErrInvalidKeySize, got %v", err)
	}
}

func TestDecrypt_FailsOnTamperedCiphertext(t *testing.T) {
	box, err := crypto.NewBox(testKey)
	if err != nil {
		t.Fatalf("NewBox failed: %v", err)
	}

	ciphertext, err := box.Encrypt("data")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	tampered := "A" + ciphertext[1:]
	if _, err := box.Decrypt(tampered); err == nil {
		t.Fatal("expected Decrypt to fail on tampered ciphertext")
	}
}
