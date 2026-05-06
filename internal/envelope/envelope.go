// Package envelope defines the JSON envelope used by CLI stdout and MCP tool results.
// Every command returns {"ok":true,"data":{}} or {"ok":false,"error":{}}.
package envelope

import (
	"encoding/json"
	"fmt"
)

// Envelope is the top-level JSON contract.
type Envelope struct {
	OK              bool       `json:"ok"`
	Data            any        `json:"data,omitempty"`
	ExecutionClass  string     `json:"execution_class,omitempty"`
	NetworkRequired *bool      `json:"network_required,omitempty"`
	RemoteWrite     *bool      `json:"remote_write,omitempty"`
	Submitted       *bool      `json:"submitted,omitempty"`
	CacheUsed       *bool      `json:"cache_used,omitempty"`
	Warnings        []string   `json:"warnings,omitempty"`
	Error           *ErrorBody `json:"error,omitempty"`
}

// ErrorBody carries a machine-readable error code and human message.
type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// ExecutionMetadata describes whether a command uses local state, remote
// preflight/read checks, or remote submit/write behavior.
type ExecutionMetadata struct {
	ExecutionClass  string
	NetworkRequired bool
	RemoteWrite     bool
	Submitted       *bool
	CacheUsed       *bool
}

// OK wraps data into a success envelope.
func OK(data any, warnings ...string) Envelope {
	return Envelope{OK: true, Data: data, Warnings: warnings}
}

// OKWithMetadata wraps data and adds stable execution semantics for agents.
func OKWithMetadata(data any, metadata ExecutionMetadata, warnings ...string) Envelope {
	return Envelope{
		OK:              true,
		Data:            data,
		ExecutionClass:  metadata.ExecutionClass,
		NetworkRequired: boolPtr(metadata.NetworkRequired),
		RemoteWrite:     boolPtr(metadata.RemoteWrite),
		Submitted:       metadata.Submitted,
		CacheUsed:       metadata.CacheUsed,
		Warnings:        warnings,
	}
}

// Err wraps an error into a failure envelope.
func Err(code string, msg string, details map[string]any) Envelope {
	return Envelope{OK: false, Error: &ErrorBody{Code: code, Message: msg, Details: details}}
}

func boolPtr(value bool) *bool {
	return &value
}

// PrintJSON serializes e to compact JSON and writes it to stdout.
// This is the stable machine contract — agents parse it.
func PrintJSON(e Envelope) {
	b, _ := json.Marshal(e)
	fmt.Println(string(b))
}
