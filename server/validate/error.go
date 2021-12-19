package validate

import "fmt"

func newValidateError(section, key string, value interface{}, message string, original error) *ValidateError {
	return &ValidateError{
		section:       section,
		key:           key,
		value:         value,
		message:       message,
		originalError: original,
	}
}

type ValidateError struct {
	error

	section string
	key     string
	value   interface{}

	message string

	originalError error
}

func (ve *ValidateError) Error() string {
	if ve.originalError != nil {
		return fmt.Sprintf("failed to validate %s.%s (%v), %s: %v", ve.section, ve.key, ve.value, ve.message, ve.originalError)
	} else {
		return fmt.Sprintf("failed to validate %s.%s (%v), %s", ve.section, ve.key, ve.value, ve.message)
	}
}
