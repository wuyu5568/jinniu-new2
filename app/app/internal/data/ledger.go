package data

import (
	"context"
	"time"

	"github.com/jinniu/app/app/app/internal/biz"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ledgerRepo struct{ data *Data }

func NewLedgerRepo(d *Data) biz.LedgerRepo { return &ledgerRepo{data: d} }

func (r *ledgerRepo) Create(ctx context.Context, e *biz.LedgerEntry) error {
	m := LedgerEntryModel{
		UserID:      e.UserID,
		OrderID:     e.OrderID,
		EntryType:   e.EntryType,
		Amount:      e.Amount,
		BalanceKind: e.BalanceKind,
		Remark:      e.Remark,
	}
	return r.data.db.WithContext(ctx).Create(&m).Error
}

func (r *ledgerRepo) ListByUser(ctx context.Context, userID uint64, from, to time.Time) ([]*biz.LedgerEntry, error) {
	q := r.data.db.WithContext(ctx).Where("created_at >= ? AND created_at < ?", from, to)
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	var rows []LedgerEntryModel
	if err := q.Order("created_at DESC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return ledgerModelsToBiz(rows), nil
}

func (r *ledgerRepo) ListPaged(ctx context.Context, address, entryType string, page, pageSize int) ([]*biz.LedgerEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	base := r.data.db.WithContext(ctx).Table("ledger_entries").
		Joins("LEFT JOIN users ON users.id = ledger_entries.user_id")
	if address != "" {
		base = base.Where("users.address LIKE ?", "%"+address+"%")
	}
	if entryType != "" {
		base = base.Where("ledger_entries.entry_type = ?", entryType)
	}
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []LedgerEntryModel
	offset := (page - 1) * pageSize
	if err := base.Session(&gorm.Session{}).
		Select("ledger_entries.id, ledger_entries.user_id, ledger_entries.order_id, ledger_entries.entry_type, ledger_entries.amount, ledger_entries.balance_kind, ledger_entries.remark, ledger_entries.created_at").
		Order("ledger_entries.created_at DESC, ledger_entries.id DESC").
		Offset(offset).Limit(pageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return ledgerModelsToBiz(rows), int(total), nil
}

func ledgerModelsToBiz(rows []LedgerEntryModel) []*biz.LedgerEntry {
	out := make([]*biz.LedgerEntry, len(rows))
	for i := range rows {
		out[i] = &biz.LedgerEntry{
			ID:          rows[i].ID,
			UserID:      rows[i].UserID,
			OrderID:     rows[i].OrderID,
			EntryType:   rows[i].EntryType,
			Amount:      rows[i].Amount,
			BalanceKind: rows[i].BalanceKind,
			Remark:      rows[i].Remark,
			CreatedAt:   rows[i].CreatedAt,
		}
	}
	return out
}

func (r *ledgerRepo) SumAmountByTypes(ctx context.Context, entryTypes []string) (decimal.Decimal, error) {
	if len(entryTypes) == 0 {
		return decimal.Zero, nil
	}
	var sum decimal.Decimal
	err := r.data.db.WithContext(ctx).Model(&LedgerEntryModel{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("entry_type IN ?", entryTypes).
		Scan(&sum).Error
	return sum, err
}

func (r *ledgerRepo) SumAmountByTypesBetween(ctx context.Context, entryTypes []string, from, to time.Time) (decimal.Decimal, error) {
	if len(entryTypes) == 0 {
		return decimal.Zero, nil
	}
	var sum decimal.Decimal
	err := r.data.db.WithContext(ctx).Model(&LedgerEntryModel{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("entry_type IN ? AND created_at >= ? AND created_at < ?", entryTypes, from, to).
		Scan(&sum).Error
	return sum, err
}
