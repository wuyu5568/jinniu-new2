package data

import (
	"context"
	"errors"

	"github.com/go-sql-driver/mysql"
	"github.com/jinniu/app/app/app/internal/biz"
	"gorm.io/gorm"
)

// EthUserRecord maps eth_user_record（对齐 new18new）.
type EthUserRecord struct {
	ID        int64  `gorm:"primaryKey"`
	Hash      string `gorm:"column:hash;type:varchar(100);not null"`
	UserId    int64  `gorm:"column:user_id;not null"`
	Status    string `gorm:"column:status;type:varchar(45);not null"`
	Type      string `gorm:"column:type;type:varchar(45);not null"`
	Amount    string `gorm:"column:amount;type:varchar(64);not null"`
	AmountTwo uint64 `gorm:"column:amount_two;not null"`
	CoinType  string `gorm:"column:coin_type;type:varchar(45);not null"`
	Last      int64  `gorm:"column:last;not null;uniqueIndex:uk_eth_user_record_last"`
}

func (EthUserRecord) TableName() string { return "eth_user_record" }

type ethUserRecordRepo struct{ data *Data }

func NewEthUserRecordRepo(d *Data) biz.EthUserRecordRepo {
	return &ethUserRecordRepo{data: d}
}

// GetEthUserRecordLast returns MAX(last). Empty table → -1 (next index = 0).
func (e *ethUserRecordRepo) GetEthUserRecordLast(ctx context.Context) (int64, error) {
	var ethUserRecord EthUserRecord
	err := e.data.db.WithContext(ctx).Table("eth_user_record").Order("last desc").First(&ethUserRecord).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return -1, nil
		}
		return -1, err
	}
	return ethUserRecord.Last, nil
}

func (e *ethUserRecordRepo) GetByLast(ctx context.Context, last int64) (*biz.EthUserRecord, error) {
	var row EthUserRecord
	err := e.data.db.WithContext(ctx).Table("eth_user_record").Where("last = ?", last).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &biz.EthUserRecord{
		ID:        row.ID,
		UserId:    row.UserId,
		Hash:      row.Hash,
		Status:    row.Status,
		Type:      row.Type,
		Amount:    row.Amount,
		AmountTwo: row.AmountTwo,
		CoinType:  row.CoinType,
		Last:      row.Last,
	}, nil
}

func (e *ethUserRecordRepo) CreateEthUserRecordListByHash(ctx context.Context, r *biz.EthUserRecord) (*biz.EthUserRecord, error) {
	row := EthUserRecord{
		UserId:    r.UserId,
		Hash:      r.Hash,
		Type:      r.Type,
		Status:    r.Status,
		Amount:    r.Amount,
		AmountTwo: r.AmountTwo,
		CoinType:  r.CoinType,
		Last:      r.Last,
	}
	if err := e.data.db.WithContext(ctx).Table("eth_user_record").Create(&row).Error; err != nil {
		if isMySQLDuplicate(err) {
			return nil, biz.ErrChainDepositExists
		}
		return nil, err
	}
	return &biz.EthUserRecord{
		ID:        row.ID,
		UserId:    row.UserId,
		Hash:      row.Hash,
		Status:    row.Status,
		Type:      row.Type,
		Amount:    row.Amount,
		AmountTwo: row.AmountTwo,
		CoinType:  row.CoinType,
		Last:      row.Last,
	}, nil
}

func (e *ethUserRecordRepo) DeleteByLast(ctx context.Context, last int64) error {
	return e.data.db.WithContext(ctx).Table("eth_user_record").Where("last = ?", last).Delete(&EthUserRecord{}).Error
}

func isMySQLDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
