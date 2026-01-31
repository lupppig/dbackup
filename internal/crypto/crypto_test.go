package crypto

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCrypto_EndToEnd(t *testing.T) {
	passphrase := "super-secret-passphrase"
	data := []byte("this is some sensitive database dump data that needs to be secured.")

	km, err := NewKeyManager(passphrase, "")
	require.NoError(t, err)

	// Encryption
	var encrypted bytes.Buffer
	ew, err := NewEncryptWriter(&encrypted, km)
	require.NoError(t, err)

	_, err = ew.Write(data)
	require.NoError(t, err)
	err = ew.Close()
	require.NoError(t, err)

	assert.NotEqual(t, data, encrypted.Bytes(), "Encrypted data should not be plaintext")
	assert.Contains(t, encrypted.String(), MagicBytes, "Encrypted data should contain magic bytes")

	// Decryption
	dr := NewDecryptReader(&encrypted, km)
	decrypted, err := io.ReadAll(dr)
	require.NoError(t, err)

	assert.Equal(t, data, decrypted, "Decrypted data should match original data")
}

func TestCrypto_WrongPassphrase(t *testing.T) {
	data := []byte("secret data")
	km, _ := NewKeyManager("correct-pass", "")

	var encrypted bytes.Buffer
	ew, _ := NewEncryptWriter(&encrypted, km)
	ew.Write(data)
	ew.Close()

	// Decrypt with wrong passphrase
	kmWrong, _ := NewKeyManager("wrong-pass", "")
	dr := NewDecryptReader(&encrypted, kmWrong)
	_, err := io.ReadAll(dr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption failed")
}

func TestCrypto_KeyFile(t *testing.T) {
	keyFile := "test.key"
	keyData := []byte("01234567890123456789012345678901") // 32 bytes
	err := os.WriteFile(keyFile, keyData, 0600)
	require.NoError(t, err)
	defer os.Remove(keyFile)

	km, err := NewKeyManager("", keyFile)
	require.NoError(t, err)

	data := []byte("confidential info")
	var encrypted bytes.Buffer
	ew, _ := NewEncryptWriter(&encrypted, km)
	ew.Write(data)
	ew.Close()

	dr := NewDecryptReader(&encrypted, km)
	decrypted, _ := io.ReadAll(dr)
	assert.Equal(t, data, decrypted)
}

func TestCrypto_LargeData(t *testing.T) {
	// Generate data larger than ChunkSize (64KB)
	largeData := make([]byte, 200*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	km, _ := NewKeyManager("pass", "")
	var encrypted bytes.Buffer
	ew, _ := NewEncryptWriter(&encrypted, km)
	ew.Write(largeData)
	ew.Close()

	dr := NewDecryptReader(&encrypted, km)
	decrypted, _ := io.ReadAll(dr)
	assert.Equal(t, largeData, decrypted)
}
