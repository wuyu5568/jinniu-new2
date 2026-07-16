package biz

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestChunkAccelerationIntoOrder(t *testing.T) {
	acc := decimal.RequireFromString("50")
	exit := decimal.RequireFromString("200")
	amount := decimal.RequireFromString("1000")

	chunk, room := chunkAcceleration(acc, exit, amount)
	if !room.Equal(decimal.RequireFromString("150")) {
		t.Fatalf("room=%s", room)
	}
	if !chunk.Equal(decimal.RequireFromString("150")) {
		t.Fatalf("chunk=%s want 150", chunk)
	}

	chunk2, _ := chunkAcceleration(acc, exit, decimal.RequireFromString("10"))
	if !chunk2.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("chunk2=%s", chunk2)
	}
}

func TestChunkAccelerationNoRoom(t *testing.T) {
	chunk, room := chunkAcceleration(
		decimal.RequireFromString("200"),
		decimal.RequireFromString("200"),
		decimal.RequireFromString("50"),
	)
	if !room.IsZero() || !chunk.IsZero() {
		t.Fatalf("chunk=%s room=%s", chunk, room)
	}
}
