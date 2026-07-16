package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/jinniu/app/app/app/internal/conf"
	"github.com/jinniu/app/app/app/internal/pkg/wallet"
	"github.com/shopspring/decimal"
)

// User is the domain user (balances on users table).
type User struct {
	ID                  uint64
	Address             string
	InviterID           *uint64
	AccountBalance      decimal.Decimal
	WithdrawableBalance decimal.Decimal
	CommunityLevel      uint8
	CommunityVolume     decimal.Decimal
	DisabledAt          *time.Time
	RewardLocked        bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (u *User) IsDisabled() bool {
	return u != nil && u.DisabledAt != nil
}

// LoginChallenge is a one-time login nonce.
type LoginChallenge struct {
	ID        uint64
	Address   string
	Nonce     string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// UserRepo loads users by id or address.
type UserRepo interface {
	FindByAddress(ctx context.Context, address string) (*User, error)
	FindByID(ctx context.Context, id uint64) (*User, error)
	Create(ctx context.Context, u *User) (*User, error)
	ListAll(ctx context.Context) ([]*User, error)
	CountDirectReferrals(ctx context.Context, userID uint64) (int, error)
	UpdateCommunity(ctx context.Context, userID uint64, level uint8, volume decimal.Decimal) error
	ExistsByAddress(ctx context.Context, address string) (bool, error)
	ListPaged(ctx context.Context, address string, page, pageSize int) ([]*User, int, error)
	ListByInviter(ctx context.Context, inviterID uint64) ([]*User, error)
	SoftDelete(ctx context.Context, userID uint64, at time.Time) error
	Restore(ctx context.Context, userID uint64) error
	SetRewardLocked(ctx context.Context, userID uint64, locked bool) error
	SetCommunityLevel(ctx context.Context, userID uint64, level uint8) error
	ListByPathPrefix(ctx context.Context, pathPrefix string) ([]*User, error)
	CountAll(ctx context.Context) (int, error)
	CountNonDisabled(ctx context.Context) (int, error)
	CountCreatedBetween(ctx context.Context, from, to time.Time) (int, error)
	SumWithdrawableBalance(ctx context.Context) (decimal.Decimal, error)
	SumAccountBalance(ctx context.Context) (decimal.Decimal, error)
}

// UserBalanceRepo mutates account_balance / withdrawable_balance on users.
type UserBalanceRepo interface {
	AddAccountBalance(ctx context.Context, userID uint64, delta decimal.Decimal) error
	AddWithdrawableBalance(ctx context.Context, userID uint64, delta decimal.Decimal) error
	SubAccountBalance(ctx context.Context, userID uint64, delta decimal.Decimal) error
	SubWithdrawableBalance(ctx context.Context, userID uint64, delta decimal.Decimal) error
	SetAccountBalance(ctx context.Context, userID uint64, amount decimal.Decimal) error
	SetWithdrawableBalance(ctx context.Context, userID uint64, amount decimal.Decimal) error
}

// RecommendRepo maintains user_recommends.path.
type RecommendRepo interface {
	GetPath(ctx context.Context, userID uint64) (string, error)
	SavePath(ctx context.Context, userID uint64, path string) error
}

// LoginChallengeRepo nonce challenges for wallet login.
type LoginChallengeRepo interface {
	Create(ctx context.Context, address, nonce string, expiresAt time.Time) error
	FindUsable(ctx context.Context, address, nonce string, now time.Time) (*LoginChallenge, error)
	MarkUsed(ctx context.Context, id uint64, usedAt time.Time) error
}

// SignatureVerifier verifies wallet personal_sign signatures.
type SignatureVerifier interface {
	Verify(address, message, signature string) error
}

// TokenIssuer issues JWT tokens after successful login.
type TokenIssuer interface {
	Issue(userID uint64, address string) (string, error)
	IssueAdmin() (string, error)
}

// LoginResult is returned after a successful login.
type LoginResult struct {
	Token string
	User  *User
}

// UserUseCase wallet login, invite bind, profile, admin deposit.
type UserUseCase struct {
	users       UserRepo
	balances    UserBalanceRepo
	recommends  RecommendRepo
	challenges  LoginChallengeRepo
	ledger      LedgerRepo
	verifier    SignatureVerifier
	tokens      TokenIssuer
	auth        *conf.Auth
	genesisAddr string
}

// NewUserUseCase constructs UserUseCase.
func NewUserUseCase(
	users UserRepo,
	balances UserBalanceRepo,
	recommends RecommendRepo,
	challenges LoginChallengeRepo,
	ledger LedgerRepo,
	verifier SignatureVerifier,
	tokens TokenIssuer,
	auth *conf.Auth,
	genesisAddr string,
) *UserUseCase {
	return &UserUseCase{
		users:       users,
		balances:    balances,
		recommends:  recommends,
		challenges:  challenges,
		ledger:      ledger,
		verifier:    verifier,
		tokens:      tokens,
		auth:        auth,
		genesisAddr: wallet.NormalizeOrEmpty(genesisAddr),
	}
}

// CreateLoginChallenge issues a one-time nonce and message to sign.
func (uc *UserUseCase) CreateLoginChallenge(ctx context.Context, address string) (message, nonce string, expiresAt time.Time, err error) {
	address, ok := wallet.NormalizeAddress(address)
	if !ok {
		return "", "", time.Time{}, ErrInvalidAmount
	}
	ttl := 5 * time.Minute
	if uc.auth != nil && uc.auth.ChallengeTTL > 0 {
		ttl = uc.auth.ChallengeTTL
	}
	nonce, err = randomNonce(16)
	if err != nil {
		return "", "", time.Time{}, err
	}
	expiresAt = time.Now().Add(ttl)
	message = buildLoginMessage(nonce)
	if err := uc.challenges.Create(ctx, address, nonce, expiresAt); err != nil {
		return "", "", time.Time{}, err
	}
	return message, nonce, expiresAt, nil
}

// Login verifies signature, registers new users with invite, and issues a token.
func (uc *UserUseCase) Login(ctx context.Context, address, signature, nonce, inviteCode string) (*LoginResult, error) {
	address, ok := wallet.NormalizeAddress(address)
	if !ok || signature == "" || nonce == "" {
		return nil, ErrInvalidNonce
	}
	inviteCode = wallet.NormalizeOrEmpty(inviteCode)

	challenge, err := uc.challenges.FindUsable(ctx, address, nonce, time.Now())
	if err != nil {
		return nil, ErrInvalidNonce
	}
	message := buildLoginMessage(nonce)
	if err := uc.verifier.Verify(address, message, signature); err != nil {
		return nil, ErrInvalidSignature
	}
	if err := uc.challenges.MarkUsed(ctx, challenge.ID, time.Now()); err != nil {
		return nil, err
	}

	user, err := uc.users.FindByAddress(ctx, address)
	if err == nil && user != nil {
		if user.IsDisabled() {
			return nil, ErrUserDisabled
		}
		token, err := uc.tokens.Issue(user.ID, user.Address)
		if err != nil {
			return nil, err
		}
		return &LoginResult{Token: token, User: user}, nil
	}
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	genesis := uc.genesisAddr
	var inviterID *uint64
	if address == genesis {
		// genesis may register without invite
	} else {
		if inviteCode == "" {
			return nil, ErrInviteRequired
		}
		if inviteCode == address {
			return nil, ErrInviteInvalid
		}
		inviter, err := uc.users.FindByAddress(ctx, inviteCode)
		if err != nil || inviter == nil {
			return nil, ErrInviteInvalid
		}
		id := inviter.ID
		inviterID = &id
	}

	user, err = uc.users.Create(ctx, &User{
		Address:             address,
		InviterID:           inviterID,
		AccountBalance:      decimal.Zero,
		WithdrawableBalance: decimal.Zero,
	})
	if err != nil {
		return nil, err
	}

	path := strconv.FormatUint(user.ID, 10)
	if inviterID != nil {
		parentPath, _ := uc.recommends.GetPath(ctx, *inviterID)
		if parentPath != "" {
			path = parentPath + "," + path
		}
	}
	_ = uc.recommends.SavePath(ctx, user.ID, path)

	token, err := uc.tokens.Issue(user.ID, user.Address)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, User: user}, nil
}

// EthAuthorize verifies personal_sign of the wallet address (no nonce) and issues a token.
func (uc *UserUseCase) EthAuthorize(ctx context.Context, address, signature, inviteCode string) (*LoginResult, error) {
	address, ok := wallet.NormalizeAddress(address)
	if !ok || signature == "" {
		return nil, ErrInvalidSignature
	}
	inviteCode = wallet.NormalizeOrEmpty(inviteCode)

	if err := uc.verifier.Verify(address, address, signature); err != nil {
		return nil, ErrInvalidSignature
	}

	user, err := uc.users.FindByAddress(ctx, address)
	if err == nil && user != nil {
		if user.IsDisabled() {
			return nil, ErrUserDisabled
		}
		token, err := uc.tokens.Issue(user.ID, user.Address)
		if err != nil {
			return nil, err
		}
		return &LoginResult{Token: token, User: user}, nil
	}
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	genesis := uc.genesisAddr
	var inviterID *uint64
	if address == genesis {
		// genesis may register without invite
	} else {
		if inviteCode == "" {
			return nil, ErrInviteRequired
		}
		if inviteCode == address {
			return nil, ErrInviteInvalid
		}
		inviter, err := uc.users.FindByAddress(ctx, inviteCode)
		if err != nil || inviter == nil {
			return nil, ErrInviteInvalid
		}
		id := inviter.ID
		inviterID = &id
	}

	user, err = uc.users.Create(ctx, &User{
		Address:             address,
		InviterID:           inviterID,
		AccountBalance:      decimal.Zero,
		WithdrawableBalance: decimal.Zero,
	})
	if err != nil {
		return nil, err
	}

	path := strconv.FormatUint(user.ID, 10)
	if inviterID != nil {
		parentPath, _ := uc.recommends.GetPath(ctx, *inviterID)
		if parentPath != "" {
			path = parentPath + "," + path
		}
	}
	_ = uc.recommends.SavePath(ctx, user.ID, path)

	token, err := uc.tokens.Issue(user.ID, user.Address)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Token: token, User: user}, nil
}

// AdminLogin validates configured admin credentials and returns an admin JWT.
func (uc *UserUseCase) AdminLogin(_ context.Context, username, password string) (string, error) {
	if uc.auth == nil || username != uc.auth.AdminUsername || password != uc.auth.AdminPassword {
		return "", ErrUnauthorized
	}
	return uc.tokens.IssueAdmin()
}

// SetWithdrawable sets withdrawable balance to an absolute target (>= 0) and ledgers the delta as admin_adjust.
func (uc *UserUseCase) SetWithdrawable(ctx context.Context, address string, amount decimal.Decimal) (*User, error) {
	address, ok := wallet.NormalizeAddress(address)
	if !ok || amount.IsNegative() {
		return nil, ErrInvalidAmount
	}
	user, err := uc.users.FindByAddress(ctx, address)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}
	before := user.WithdrawableBalance
	if before.Equal(amount) {
		return user, nil
	}
	if err := uc.balances.SetWithdrawableBalance(ctx, user.ID, amount); err != nil {
		return nil, err
	}
	if uc.ledger != nil {
		delta := amount.Sub(before)
		_ = uc.ledger.Create(ctx, &LedgerEntry{
			UserID:      user.ID,
			EntryType:   LedgerAdminAdjust,
			Amount:      delta,
			BalanceKind: BalanceWithdrawable,
			Remark:      "admin set withdrawable " + before.String() + "->" + amount.String(),
		})
	}
	return uc.users.FindByID(ctx, user.ID)
}

// SetAccountBalance sets account balance to an absolute target (>= 0) and ledgers the delta as deposit.
func (uc *UserUseCase) SetAccountBalance(ctx context.Context, address string, amount decimal.Decimal) (*User, error) {
	address, ok := wallet.NormalizeAddress(address)
	if !ok || amount.IsNegative() {
		return nil, ErrInvalidAmount
	}
	user, err := uc.users.FindByAddress(ctx, address)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}
	before := user.AccountBalance
	if before.Equal(amount) {
		return user, nil
	}
	if err := uc.balances.SetAccountBalance(ctx, user.ID, amount); err != nil {
		return nil, err
	}
	if uc.ledger != nil {
		delta := amount.Sub(before)
		_ = uc.ledger.Create(ctx, &LedgerEntry{
			UserID:      user.ID,
			EntryType:   LedgerDeposit,
			Amount:      delta,
			BalanceKind: BalanceAccount,
			Remark:      "admin set account " + before.String() + "->" + amount.String(),
		})
	}
	return uc.users.FindByID(ctx, user.ID)
}

// GetProfile returns a user by id.
func (uc *UserUseCase) GetProfile(ctx context.Context, userID uint64) (*User, error) {
	user, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// CheckAddress reports whether the wallet address is already registered.
func (uc *UserUseCase) CheckAddress(ctx context.Context, address string) (registered bool, isGenesis bool, err error) {
	address, ok := wallet.NormalizeAddress(address)
	if !ok {
		return false, false, ErrInvalidAmount
	}
	ok, err = uc.users.ExistsByAddress(ctx, address)
	if err != nil {
		return false, false, err
	}
	return ok, address == uc.genesisAddr, nil
}

// Deposit credits account balance for admin recharge.
func (uc *UserUseCase) Deposit(ctx context.Context, address string, amount decimal.Decimal) (*User, error) {
	address, ok := wallet.NormalizeAddress(address)
	if !ok || !amount.IsPositive() {
		return nil, ErrInvalidAmount
	}
	user, err := uc.users.FindByAddress(ctx, address)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}
	if err := uc.balances.AddAccountBalance(ctx, user.ID, amount); err != nil {
		return nil, err
	}
	if uc.ledger != nil {
		_ = uc.ledger.Create(ctx, &LedgerEntry{
			UserID:      user.ID,
			EntryType:   LedgerDeposit,
			Amount:      amount,
			BalanceKind: BalanceAccount,
			Remark:      "admin deposit",
		})
	}
	return uc.users.FindByID(ctx, user.ID)
}

func buildLoginMessage(nonce string) string {
	return "Jinniu login\nnonce: " + nonce
}

func randomNonce(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
