package biz

import (
	"context"
	"errors"
	"testing"
)

func TestSettleForceDisabled(t *testing.T) {
	uc := &RecordUseCase{allowForceSettle: false}
	_, err := uc.SettleStatic(context.Background(), nil, true)
	if !errors.Is(err, ErrForceSettleDisabled) {
		t.Fatalf("want ErrForceSettleDisabled, got %v", err)
	}
}
