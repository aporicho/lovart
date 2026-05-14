package generation

import (
	"fmt"
	"testing"
)

func TestIsConcurrencyLimitError(t *testing.T) {
	err := fmt.Errorf("generation: submit: take slot: %w", &SlotError{Code: 123, Message: "已达当前并发上限"})
	if !IsConcurrencyLimitError(err) {
		t.Fatalf("IsConcurrencyLimitError returned false")
	}
	if IsConcurrencyLimitError(fmt.Errorf("some other submit failure")) {
		t.Fatalf("IsConcurrencyLimitError returned true for unrelated error")
	}
}
