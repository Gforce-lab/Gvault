package domain

import "fmt"

// ErrorCode is a stable machine-readable failure code.
type ErrorCode string

const (
	CodeValidation          ErrorCode = "VALIDATION_ERROR"
	CodeNotFound            ErrorCode = "NOT_FOUND"
	CodeConflict            ErrorCode = "CONFLICT"
	CodeUnauthorized        ErrorCode = "UNAUTHORIZED"
	CodeForbidden           ErrorCode = "FORBIDDEN"
	CodeProvider            ErrorCode = "PROVIDER_ERROR"
	CodeIncompatible        ErrorCode = "INCOMPATIBLE"
	CodeInternal            ErrorCode = "INTERNAL_ERROR"
	CodeGPUModelMismatch    ErrorCode = "GPU_MODEL_MISMATCH"
	CodeDriverMismatch      ErrorCode = "DRIVER_MISMATCH"
	CodeArchMismatch        ErrorCode = "ARCH_MISMATCH"
	CodeImageDigestMismatch ErrorCode = "IMAGE_DIGEST_MISMATCH"
	CodeSnapshotNotReady    ErrorCode = "SNAPSHOT_NOT_READY"
	CodeMissingMetadata     ErrorCode = "MISSING_METADATA"
	CodeRestoreTimeout      ErrorCode = "RESTORE_TIMEOUT"
	CodeProbeFailed         ErrorCode = "RESTORE_PROBE_FAILED"
)

// AppError is a typed application error safe to map to HTTP responses.
type AppError struct {
	Code    ErrorCode
	Message string
	Details map[string]any
	Cause   error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Cause }

func NewAppError(code ErrorCode, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func WrapAppError(code ErrorCode, message string, cause error) *AppError {
	return &AppError{Code: code, Message: message, Cause: cause}
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if ae, ok := err.(*AppError); ok {
		return ae.Code == CodeNotFound
	}
	return false
}
