// Package errs classifies errors by transport-agnostic kind and stable code; ports map kinds to statuses.
package errs

import (
	"errors"
	"fmt"
	"slices"
	"sync"
)

// Kind is the class of failure; it decides status, logging and retryability.
type Kind int

// Kinds, from the client's fault to ours.
const (
	Internal Kind = iota
	Invalid
	NotFound
	Conflict
	Unprocessable
	Unauthorized
	Forbidden
	Unavailable
	RateLimited
)

// Code identifies one failure for machines; it is the stable part of the contract.
type Code string

// FieldError points at one input that failed; Field is a JSON pointer into the body or query.x / header.x / path.x.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error is a classified error. Msg is written for end users; wrap context for logs with fmt.Errorf.
type Error struct {
	Kind   Kind
	Code   Code
	Msg    string
	Fields []FieldError
}

func (e *Error) Error() string { return e.Msg }

// New returns a sentinel-style error; compare with errors.Is. Every code passes through here, which is what lets
// the contract test compare the emitted set against the OpenAPI enum.
func New(kind Kind, code Code, msg string) *Error {
	registry.Lock()
	registry.codes[code] = append(registry.codes[code], kind)
	registry.Unlock()
	return &Error{Kind: kind, Code: code, Msg: msg}
}

var registry = struct {
	sync.Mutex
	codes map[Code][]Kind
}{codes: map[Code][]Kind{internalCode: {Internal}}}

const internalCode Code = "internal"

// Codes lists every code constructed so far, sorted, with the kinds it was constructed with.
func Codes() map[Code][]Kind {
	registry.Lock()
	defer registry.Unlock()
	out := make(map[Code][]Kind, len(registry.codes))
	for c, kinds := range registry.codes {
		out[c] = slices.Clone(kinds)
	}
	return out
}

// Wrap adds context for logs while keeping kind, code and the user-facing Msg reachable.
func Wrap(err error, format string, args ...any) error {
	return fmt.Errorf(format+": %w", append(args, err)...)
}

// KindOf returns the outermost classified kind, or Internal.
func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return Internal
}

// CodeOf returns the outermost classified code, or "internal".
func CodeOf(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return internalCode
}

// WithFields copies e with field-level detail attached.
func (e *Error) WithFields(fields ...FieldError) *Error {
	return &Error{Kind: e.Kind, Code: e.Code, Msg: e.Msg, Fields: fields}
}

// FieldsOf returns the classified error's field errors, if any.
func FieldsOf(err error) []FieldError {
	var e *Error
	if errors.As(err, &e) {
		return e.Fields
	}
	return nil
}

// UserMessage returns the classified error's Msg, or "" for unclassified errors, which must never reach users.
func UserMessage(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Msg
	}
	return ""
}

// Retryable reports whether the same or a re-evaluated request can succeed later.
func Retryable(err error) bool {
	switch KindOf(err) {
	case Unavailable, RateLimited:
		return true
	case Conflict:
		return CodeOf(err) == "concurrent-update"
	default:
		return false
	}
}
