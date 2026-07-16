package auth

import (
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jinniu/app/app/app/internal/biz"
	"github.com/jinniu/app/app/app/internal/conf"
	"strconv"
)

// EthVerifier verifies Ethereum personal_sign signatures.
type EthVerifier struct{}

func NewSignatureVerifier() biz.SignatureVerifier { return &EthVerifier{} }

func (v *EthVerifier) Verify(address, message, signature string) error {
	address = strings.TrimSpace(address)
	signature = strings.TrimSpace(signature)
	if address == "" || signature == "" || message == "" {
		return biz.ErrInvalidSignature
	}
	sig, err := hexutil.Decode(signature)
	if err != nil {
		return biz.ErrInvalidSignature
	}
	if len(sig) != 65 {
		return biz.ErrInvalidSignature
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	// Try exact message, then lowercase / EIP-55 checksum (wallet clients differ).
	candidates := []string{message}
	lower := strings.ToLower(message)
	if lower != message {
		candidates = append(candidates, lower)
	}
	if common.IsHexAddress(message) {
		checksum := common.HexToAddress(message).Hex()
		if checksum != message && checksum != lower {
			candidates = append(candidates, checksum)
		}
	}
	want := common.HexToAddress(address)
	for _, msg := range candidates {
		hash := accounts.TextHash([]byte(msg))
		pub, err := crypto.SigToPub(hash, sig)
		if err != nil {
			continue
		}
		recovered := crypto.PubkeyToAddress(*pub)
		if strings.EqualFold(recovered.Hex(), want.Hex()) {
			return nil
		}
	}
	return biz.ErrInvalidSignature
}

// JWTIssuer issues HS256 JWTs.
type JWTIssuer struct {
	key []byte
	ttl time.Duration
}

func NewTokenIssuer(authConf *conf.Auth) biz.TokenIssuer {
	key := []byte("change-me")
	ttl := 72 * time.Hour
	if authConf != nil {
		if authConf.JWTKey != "" {
			key = []byte(authConf.JWTKey)
		}
	}
	return &JWTIssuer{key: key, ttl: ttl}
}

func (i *JWTIssuer) Issue(userID uint64, address string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"uid":  userID,
		"addr": address,
		"exp":  now.Add(i.ttl).Unix(),
		"iat":  now.Unix(),
		"sub":  strconv.FormatUint(userID, 10),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(i.key)
}

func (i *JWTIssuer) IssueAdmin() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"role": "admin",
		"exp":  now.Add(i.ttl).Unix(),
		"iat":  now.Unix(),
		"sub":  "admin",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(i.key)
}

var _ biz.SignatureVerifier = (*EthVerifier)(nil)
var _ biz.TokenIssuer = (*JWTIssuer)(nil)
