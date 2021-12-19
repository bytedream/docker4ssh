package validate

import (
	"fmt"
	"github.com/docker/docker/client"
	"strings"
)

type Validator struct {
	Cli    *client.Client
	Strict bool
}

type ValidatorResult struct {
	Strict bool

	Errors []*ValidateError
}

func (vr *ValidatorResult) Ok() bool {
	return len(vr.Errors) == 0
}

func (vr *ValidatorResult) String() string {
	builder := strings.Builder{}

	if len(vr.Errors) == 0 {
		if vr.Strict {
			builder.WriteString("Validated all files, no errors were found. You're good to go (strict mode on)")
		} else {
			builder.WriteString("Validated all files, no errors were found. You're good to go")
		}
	} else {
		if vr.Strict {
			builder.WriteString(fmt.Sprintf("Found %d errors (strict mode on)\n\n", len(vr.Errors)))
		} else {
			builder.WriteString(fmt.Sprintf("Found %d errors\n\n", len(vr.Errors)))
		}
		for _, err := range vr.Errors {
			builder.WriteString(fmt.Sprintf("%v\n", err))
		}
	}

	return builder.String()
}
