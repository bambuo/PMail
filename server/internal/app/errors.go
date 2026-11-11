package app

// UseCaseError preserves the existing application error envelope at the API boundary.
type UseCaseError struct {
	Code    int
	Message string
	Data    any
}

func (e *UseCaseError) Error() string { return e.Message }
func NewUseCaseError(code int, message string, data any) error {
	return &UseCaseError{Code: code, Message: message, Data: data}
}
