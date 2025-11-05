package errors

import "fmt"

// ErrorType represents the type of error
type ErrorType string

const (
	// MySQL related errors
	ErrorTypeMySQLConnection ErrorType = "MySQLConnection"
	ErrorTypeMySQLParameter  ErrorType = "MySQLParameter"

	// Backup related errors
	ErrorTypeBackupExecution  ErrorType = "BackupExecution"
	ErrorTypeBackupValidation ErrorType = "BackupValidation"

	// Transfer related errors
	ErrorTypeOSSUpload   ErrorType = "OSSUpload"
	ErrorTypeStreamError ErrorType = "StreamError"
	ErrorTypeFileIO      ErrorType = "FileIO"

	// Configuration errors
	ErrorTypeConfig ErrorType = "Configuration"

	// Parsing errors
	ErrorTypeParsing ErrorType = "Parsing"
)

// AppError represents an application error with type information
type AppError struct {
	Type    ErrorType
	Message string
	Err     error
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewError creates a new AppError
func NewError(errType ErrorType, message string, err error) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}

// Convenience functions for creating specific error types

// NewMySQLConnectionError creates a MySQL connection error
func NewMySQLConnectionError(err error) *AppError {
	return NewError(ErrorTypeMySQLConnection, "MySQL connection failed", err)
}

// NewMySQLParameterError creates a MySQL parameter error
func NewMySQLParameterError(message string, err error) *AppError {
	return NewError(ErrorTypeMySQLParameter, message, err)
}

// NewBackupExecutionError creates a backup execution error
func NewBackupExecutionError(err error) *AppError {
	return NewError(ErrorTypeBackupExecution, "Backup execution failed", err)
}

// NewBackupValidationError creates a backup validation error
func NewBackupValidationError(message string) *AppError {
	return NewError(ErrorTypeBackupValidation, message, nil)
}

// NewOSSUploadError creates an OSS upload error
func NewOSSUploadError(err error) *AppError {
	return NewError(ErrorTypeOSSUpload, "OSS upload failed", err)
}

// NewStreamError creates a stream error
func NewStreamError(message string, err error) *AppError {
	return NewError(ErrorTypeStreamError, message, err)
}

// NewFileIOError creates a file I/O error
func NewFileIOError(message string, err error) *AppError {
	return NewError(ErrorTypeFileIO, message, err)
}

// NewConfigError creates a configuration error
func NewConfigError(message string, err error) *AppError {
	return NewError(ErrorTypeConfig, message, err)
}

// NewParsingError creates a parsing error
func NewParsingError(field string, value string, err error) *AppError {
	message := fmt.Sprintf("Failed to parse %s '%s'", field, value)
	return NewError(ErrorTypeParsing, message, err)
}
