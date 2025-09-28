package rsa

import (
	"crypto"
	"crypto/sha256"
	"os"
	"testing"
)

func TestRSA(t *testing.T) {

	r := New(crypto.SHA256, sha256.New())

	pubKeyPem, err := os.ReadFile("public.pem")
	if err != nil {
		t.Fatal(err)
	}

	pubKey, err := Pem2PublicKey(pubKeyPem)
	if err != nil {
		t.Fatal(err)
	}

	sign, err := os.ReadFile("test.txt.sign.bin")
	if err != nil {
		t.Fatal(err)
	}

	err = r.VerifySign(pubKey, sign, []byte("2a004004-9238-4e42-a370-59fa4a24fe35"))
	if err != nil {
		t.Fatal(err)
	}

	t.Log("verify success")

}
