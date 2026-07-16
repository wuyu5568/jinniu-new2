package wallet

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// NormalizeAddress lowercases and validates an EVM address.
func NormalizeAddress(addr string) (string, bool) {
	addr = strings.TrimSpace(addr)
	if !common.IsHexAddress(addr) {
		return "", false
	}
	return strings.ToLower(common.HexToAddress(addr).Hex()), true
}

// NormalizeOrEmpty lowercases a valid address or returns empty string.
func NormalizeOrEmpty(addr string) string {
	a, ok := NormalizeAddress(addr)
	if !ok {
		return ""
	}
	return a
}
