package errors

import "fmt"

// 模块错误码前缀，便于定位
const (
	ErrStore    = "M1"
	ErrRouter   = "M2"
	ErrBilling  = "M3"
	ErrProxy    = "M4"
	ErrTUI      = "M5"
	ErrProcess  = "M6"
	ErrCLI      = "M7"
	ErrSecurity = "M8"
	ErrToken    = "M0"
)

type Error struct {
	Module  string
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s-%s] %s: %v", e.Module, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s-%s] %s", e.Module, e.Code, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// New 创建一个新的标准错误
func New(module, code, message string, cause error) error {
	return &Error{
		Module:  module,
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
