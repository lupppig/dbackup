package apperrors

import (
	"fmt"
)

type ErrorType string

const (
	TypeDependency ErrorType = "Dependency" // Missing native tool (e.g. pg_dump)
	TypeConnection ErrorType = "Connection" // Network issue
	TypeAuth       ErrorType = "Auth"       // Basic auth, SSH keys, TLS certs
	TypeIntegrity  ErrorType = "Integrity"  // Checksum mismatch, corrupt header
	TypeSecurity   ErrorType = "Security"   // Encryption/decryption failure, missing key
	TypeConfig     ErrorType = "Config"     // Invalid flags, missing required params
	TypeResource   ErrorType = "Resource"   // Permission denied, out of space, file not found
	TypeInternal   ErrorType = "Internal"   // Unexpected internal failure
)

// AppError is a rich error type that provides categorize and hints for users.
type AppError struct {
	Type    ErrorType
	Message string
	Err     error
	Hint    string
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// New creates a new AppError
func New(t ErrorType, msg string, hint string) *AppError {
	return &AppError{
		Type:    t,
		Message: msg,
		Hint:    hint,
	}
}

// Wrap wraps an existing error into an AppError
func Wrap(err error, t ErrorType, msg string, hint string) *AppError {
	return &AppError{
		Type:    t,
		Message: msg,
		Err:     err,
		Hint:    hint,
	}
}

var (
	ErrIntegrityMismatch = New(TypeIntegrity, "Integrity failure", "The backup file may be corrupt or tampered with. Verify the source integrity.")
)
