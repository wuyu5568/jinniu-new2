package data

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type UserModel struct {
	ID                  uint64 `gorm:"primaryKey"`
	Address             string `gorm:"size:64;uniqueIndex"`
	InviterID           *uint64
	AccountBalance      decimal.Decimal `gorm:"type:decimal(36,8)"`
	WithdrawableBalance decimal.Decimal `gorm:"type:decimal(36,8)"`
	CommunityLevel      uint8
	CommunityVolume     decimal.Decimal `gorm:"type:decimal(36,8)"`
	DisabledAt          *time.Time
	RewardLocked        bool
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (UserModel) TableName() string { return "users" }

type LoginChallengeModel struct {
	ID        uint64 `gorm:"primaryKey"`
	Address   string `gorm:"size:64;index"`
	Nonce     string `gorm:"size:64;uniqueIndex"`
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (LoginChallengeModel) TableName() string { return "login_challenges" }

type LocationModel struct {
	ID            uint64 `gorm:"primaryKey"`
	UserID        uint64 `gorm:"index"`
	Amount        decimal.Decimal `gorm:"type:decimal(36,8)"`
	Multiplier    decimal.Decimal `gorm:"type:decimal(3,1)"`
	ExitTarget    decimal.Decimal `gorm:"type:decimal(36,8)"`
	Accumulated   decimal.Decimal `gorm:"type:decimal(36,8)"`
	Status        string          `gorm:"size:16;index"`
	RatePercent   decimal.Decimal `gorm:"type:decimal(5,2)"`
	RateDirection string          `gorm:"size:8"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (LocationModel) TableName() string { return "locations" }

type UserRecommendModel struct {
	ID        uint64 `gorm:"primaryKey"`
	UserID    uint64 `gorm:"uniqueIndex"`
	Path      string `gorm:"size:2048"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (UserRecommendModel) TableName() string { return "user_recommends" }

type LedgerEntryModel struct {
	ID          uint64 `gorm:"primaryKey"`
	UserID      uint64 `gorm:"index"`
	OrderID     *uint64
	EntryType   string          `gorm:"size:32;index"`
	Amount      decimal.Decimal `gorm:"type:decimal(36,8)"`
	BalanceKind string          `gorm:"size:16"`
	Remark      string          `gorm:"size:255"`
	CreatedAt   time.Time       `gorm:"index"`
}

func (LedgerEntryModel) TableName() string { return "ledger_entries" }

type WithdrawModel struct {
	ID             uint64          `gorm:"primaryKey"`
	UserID         uint64          `gorm:"index"`
	Amount         decimal.Decimal `gorm:"type:decimal(36,8)"`
	FeeAmount      decimal.Decimal `gorm:"type:decimal(36,8)"`
	CreditedAmount decimal.Decimal `gorm:"type:decimal(36,8)"`
	OrderIDsJSON   []byte          `gorm:"column:order_ids_json;type:json"`
	Status         string          `gorm:"size:16;index"`
	Remark         string          `gorm:"size:255"`
	TxHash         string          `gorm:"column:tx_hash;size:80"`
	PayoutError    string          `gorm:"column:payout_error;size:255"`
	ReviewedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (WithdrawModel) TableName() string { return "withdraws" }

type BusinessConfigModel struct {
	ID        uint64 `gorm:"primaryKey"`
	ConfigKey string `gorm:"column:config_key;size:64;uniqueIndex"`
	Name      string `gorm:"size:128"`
	Value     string `gorm:"type:text"`
	SortOrder int    `gorm:"column:sort_order"`
	UpdatedAt time.Time
}

func (BusinessConfigModel) TableName() string { return "business_configs" }

// ensure no accidental AutoMigrate in NewData — models are mapping only.
var _ = gorm.ErrRecordNotFound
