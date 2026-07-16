package payout

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/shopspring/decimal"
)

const erc20TransferABI = `[{"constant":false,"inputs":[{"name":"_to","type":"address"},{"name":"_value","type":"uint256"}],"name":"transfer","outputs":[{"name":"","type":"bool"}],"type":"function"}]`

// TransferUSDT sends `amount` human USDT as 18-decimal token units on BSC; returns tx hash.
func TransferUSDT(ctx context.Context, rpcURL, tokenAddr, privHex, toAddr string, amount decimal.Decimal) (txHash string, err error) {
	if rpcURL == "" || tokenAddr == "" || privHex == "" || toAddr == "" {
		return "", fmt.Errorf("missing payout params")
	}
	if !amount.IsPositive() {
		return "", fmt.Errorf("non-positive amount")
	}
	privHex = strings.TrimPrefix(strings.TrimSpace(privHex), "0x")
	key, err := crypto.HexToECDSA(privHex)
	if err != nil {
		return "", err
	}
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return "", err
	}
	defer client.Close()

	parsed, err := abi.JSON(strings.NewReader(erc20TransferABI))
	if err != nil {
		return "", err
	}
	contract := bind.NewBoundContract(common.HexToAddress(tokenAddr), parsed, client, client, client)

	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(56))
	if err != nil {
		return "", err
	}
	auth.Context = ctx

	units := amount.Shift(18).Truncate(0)
	wei, ok := new(big.Int).SetString(units.String(), 10)
	if !ok || wei.Sign() <= 0 {
		return "", fmt.Errorf("invalid token amount")
	}

	tx, err := contract.Transact(auth, "transfer", common.HexToAddress(toAddr), wei)
	if err != nil {
		return "", err
	}
	return tx.Hash().Hex(), nil
}

// ReceiptOK returns whether a mined tx succeeded. mined=false if not yet mined.
func ReceiptOK(ctx context.Context, rpcURL, txHash string) (mined bool, success bool, err error) {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return false, false, err
	}
	defer client.Close()
	r, err := client.TransactionReceipt(ctx, common.HexToHash(txHash))
	if err != nil {
		return false, false, nil
	}
	return true, r.Status == types.ReceiptStatusSuccessful, nil
}

// HotAddress derives address from private key hex.
func HotAddress(privHex string) (string, error) {
	privHex = strings.TrimPrefix(strings.TrimSpace(privHex), "0x")
	key, err := crypto.HexToECDSA(privHex)
	if err != nil {
		return "", err
	}
	pub := key.Public().(*ecdsa.PublicKey)
	return crypto.PubkeyToAddress(*pub).Hex(), nil
}
