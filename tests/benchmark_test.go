package tests

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"testing"
)

// BenchmarkDeduplication simulates the CAS chunking and hashing over a realistic 196MB dataset.
func BenchmarkDeduplication(b *testing.B) {
	data := make([]byte, 196*1024*1024)
	for i := 0; i < len(data); i += 4096 {
		data[i] = byte(i % 256)
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		chunkSize := 4 * 1024 * 1024
		for j := 0; j < len(data); j += chunkSize {
			end := j + chunkSize
			if end > len(data) {
				end = len(data)
			}
			hash := sha256.Sum256(data[j:end])
			_ = hash
		}
	}
}

// BenchmarkEncryption simulates the AES-256-GCM encryption step for a 196MB dataset.
func BenchmarkEncryption(b *testing.B) {
	data := make([]byte, 196*1024*1024)
	key := make([]byte, 32)
	rand.Read(key)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		chunkSize := 4 * 1024 * 1024
		for j := 0; j < len(data); j += chunkSize {
			end := j + chunkSize
			if end > len(data) {
				end = len(data)
			}
			_ = gcm.Seal(nil, nonce, data[j:end], nil)
		}
	}
}

// BenchmarkHashVerification simulates the final SHA-256 payload verification for 196MB.
func BenchmarkHashVerification(b *testing.B) {
	data := make([]byte, 196*1024*1024)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		h := sha256.New()
		h.Write(data)
		_ = h.Sum(nil)
	}
}
