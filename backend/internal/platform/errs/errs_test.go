package errs

import (
	"errors"
	"testing"
)

func TestClassificationSurvivesWrapping(t *testing.T) {
	base := New(Conflict, "invalid-transition", "not allowed from here")
	wrapped := Wrap(Wrap(base, "apply %s", "X"), "service")
	if !errors.Is(wrapped, base) {
		t.Fatal("errors.Is lost the sentinel")
	}
	if KindOf(wrapped) != Conflict || CodeOf(wrapped) != "invalid-transition" || UserMessage(wrapped) != "not allowed from here" {
		t.Errorf("kind/code/msg = %v/%s/%q", KindOf(wrapped), CodeOf(wrapped), UserMessage(wrapped))
	}
	if wrapped.Error() != "service: apply X: not allowed from here" {
		t.Errorf("log message = %q", wrapped.Error())
	}
}

func TestUnclassifiedIsInternalWithNoUserMessage(t *testing.T) {
	err := errors.New("pq: connection reset")
	if KindOf(err) != Internal || CodeOf(err) != "internal" || UserMessage(err) != "" {
		t.Errorf("kind/code/msg = %v/%s/%q", KindOf(err), CodeOf(err), UserMessage(err))
	}
}

func TestRetryable(t *testing.T) {
	cases := map[error]bool{
		New(Unavailable, "unavailable", ""):     true,
		New(Conflict, "concurrent-update", ""):  true,
		New(Conflict, "invalid-transition", ""): false,
		New(NotFound, "dispute-not-found", ""):  false,
		errors.New("boom"):                      false,
	}
	for err, want := range cases {
		if got := Retryable(err); got != want {
			t.Errorf("Retryable(%v) = %v, want %v", err, got, want)
		}
	}
}
