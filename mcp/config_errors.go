package mcp

import (
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
)

type configConflictError struct {
	Message string
	Details map[string]any
}

func (e configConflictError) Error() string {
	return e.Message
}

type configCommandError struct {
	Message string
	Details map[string]any
}

func (e configCommandError) Error() string {
	return e.Message
}

type configInputError struct {
	Message string
	Details map[string]any
}

func (e configInputError) Error() string {
	return e.Message
}

func configErrorEnvelope(err error) envelope.Envelope {
	switch e := err.(type) {
	case configConflictError:
		return envelope.Err("config_conflict", e.Message, e.Details)
	case configCommandError:
		return envelope.Err("mcp_config_failed", e.Message, e.Details)
	case configInputError:
		return envelope.Err(errors.CodeInputError, e.Message, e.Details)
	default:
		return envelope.Err(errors.CodeInternal, "mcp config failed", map[string]any{"error": err.Error()})
	}
}
