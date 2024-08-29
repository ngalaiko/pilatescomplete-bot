package keys

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

var keySize = 32 // 32 bytes for AES-256

type Key []byte

func NewKey() (*Key, error) {
	bytes := make([]byte, keySize)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	key := Key(bytes)
	return &key, nil
}

func ParseKey(bytes []byte) (*Key, error) {
	switch len(bytes) {
	case 16, 24, 32:
		key := Key(bytes)
		return &key, nil
	default:
		return nil, fmt.Errorf("invalid key size: got %d, need 16, 24 or 32", len(bytes))
	}
}

func (k Key) String() string {
	return base64.URLEncoding.EncodeToString(k)
}

func (k Key) Encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], data)

	return ciphertext, nil
}

func (k Key) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return ciphertext, nil
}
