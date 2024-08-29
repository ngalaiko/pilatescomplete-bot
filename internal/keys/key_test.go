package keys

import "testing"

func Test(t *testing.T) {
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}

	plaintext := "Hello, World!"

	encrypted, err := key.Encrypt([]byte(plaintext))
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := key.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}

	if plaintext != string(decrypted) {
		t.Fatal("encrypted != decrypted")
	}
}
