package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

const (
	KeySize    = 32 // AES-256
	SaltSize   = 32
	NonceSize  = 12
	TagSize    = 16
	ChunkSize  = 64 * 1024 // 64KB chunks for GCM streaming
	MagicBytes = "DBKP"
	Version    = 1
)

// KeyManager handles key derivation and loading
type KeyManager struct {
	key []byte
}

func NewKeyManager(passphrase, keyFile string) (*KeyManager, error) {
	if passphrase == "" && keyFile == "" {
		return nil, fmt.Errorf("either passphrase or key-file must be provided for encryption")
	}

	var key []byte
	if keyFile != "" {
		var err error
		key, err = os.ReadFile(keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read key file: %w", err)
		}
		if len(key) != KeySize {
			// If not 32 bytes, hashing is used to fit
			h := sha256.Sum256(key)
			key = h[:]
		}
	} else {
		// Passphrase is used with a salt from the file header during decryption.
		// For encryption, a fresh salt is generated.
		// Placeholder key is replaced or used to derive the real key per-session.
		key = []byte(passphrase)
	}

	return &KeyManager{key: key}, nil
}

// DeriveKey derives a fixed-size key from a passphrase and salt
func DeriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, 4096, KeySize, sha256.New)
}

// EncryptWriter wraps a writer with AES-256-GCM encryption
type EncryptWriter struct {
	w    io.Writer
	gcm  cipher.AEAD
	key  []byte
	salt []byte
	buf  []byte
	err  error
}

func NewEncryptWriter(w io.Writer, km *KeyManager) (*EncryptWriter, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	// Use raw key if available (from file); otherwise derive from passphrase.
	key := km.key
	if len(key) != KeySize {
		key = DeriveKey(string(key), salt)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Write Header: Magic (4) + Version (1) + Salt (32)
	header := append([]byte(MagicBytes), Version)
	header = append(header, salt...)
	if _, err := w.Write(header); err != nil {
		return nil, err
	}

	return &EncryptWriter{
		w:    w,
		gcm:  gcm,
		key:  key,
		salt: salt,
		buf:  make([]byte, 0, ChunkSize),
	}, nil
}

func (ew *EncryptWriter) Write(p []byte) (n int, err error) {
	if ew.err != nil {
		return 0, ew.err
	}

	n = len(p)
	for len(p) > 0 {
		space := ChunkSize - len(ew.buf)
		if space > len(p) {
			ew.buf = append(ew.buf, p...)
			p = nil
		} else {
			ew.buf = append(ew.buf, p[:space]...)
			p = p[space:]
			if err := ew.flush(); err != nil {
				ew.err = err
				return 0, err
			}
		}
	}
	return n, nil
}

func (ew *EncryptWriter) flush() error {
	if len(ew.buf) == 0 {
		return nil
	}

	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}

	ciphertext := ew.gcm.Seal(nil, nonce, ew.buf, nil)

	// Chunk format: [Nonce (12)] + [Len (4)] + [Ciphertext (len + 16 tag)]
	chunkHeader := make([]byte, NonceSize+4)
	copy(chunkHeader, nonce)
	binary.BigEndian.PutUint32(chunkHeader[NonceSize:], uint32(len(ciphertext)))

	if _, err := ew.w.Write(chunkHeader); err != nil {
		return err
	}
	if _, err := ew.w.Write(ciphertext); err != nil {
		return err
	}

	ew.buf = ew.buf[:0]
	return nil
}

func (ew *EncryptWriter) Close() error {
	if ew.err != nil {
		return ew.err
	}
	if err := ew.flush(); err != nil {
		return err
	}
	if cl, ok := ew.w.(io.Closer); ok {
		return cl.Close()
	}
	return nil
}

// DecryptReader wraps a reader with AES-256-GCM decryption
type DecryptReader struct {
	r      io.Reader
	gcm    cipher.AEAD
	km     *KeyManager
	buf    []byte
	pos    int
	header bool
	err    error
}

func NewDecryptReader(r io.Reader, km *KeyManager) *DecryptReader {
	return &DecryptReader{
		r:  r,
		km: km,
	}
}

func (dr *DecryptReader) Read(p []byte) (int, error) {
	if dr.err != nil {
		return 0, dr.err
	}

	if !dr.header {
		if err := dr.readHeader(); err != nil {
			dr.err = err
			return 0, err
		}
		dr.header = true
	}

	if dr.pos >= len(dr.buf) {
		if err := dr.nextChunk(); err != nil {
			dr.err = err
			return 0, err
		}
	}

	n := copy(p, dr.buf[dr.pos:])
	dr.pos += n
	return n, nil
}

func (dr *DecryptReader) readHeader() error {
	// Magic (4) + Version (1) + Salt (32)
	head := make([]byte, 4+1+SaltSize)
	if _, err := io.ReadFull(dr.r, head); err != nil {
		return fmt.Errorf("failed to read encryption header: %w", err)
	}

	if string(head[:4]) != MagicBytes {
		return fmt.Errorf("corrupt backup: missing security magic")
	}

	salt := head[5:]
	key := dr.km.key
	if len(key) != KeySize {
		key = DeriveKey(string(key), salt)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	dr.gcm = gcm
	return nil
}

func (dr *DecryptReader) nextChunk() error {
	// [Nonce (12)] + [Len (4)]
	head := make([]byte, NonceSize+4)
	if _, err := io.ReadFull(dr.r, head); err != nil {
		return err // Might be EOF
	}

	nonce := head[:NonceSize]
	length := binary.BigEndian.Uint32(head[NonceSize:])

	ciphertext := make([]byte, length)
	if _, err := io.ReadFull(dr.r, ciphertext); err != nil {
		return fmt.Errorf("failed to read chunk: %w", err)
	}

	plaintext, err := dr.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decryption failed: invalid key or tampered data")
	}

	dr.buf = plaintext
	dr.pos = 0
	return nil
}
