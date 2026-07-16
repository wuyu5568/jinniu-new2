package data

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jinniu/app/app/app/internal/biz"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type withdrawRepo struct{ data *Data }

func NewWithdrawRepo(d *Data) biz.WithdrawRepo { return &withdrawRepo{data: d} }

func toBizWithdraw(m *WithdrawModel) (*biz.Withdraw, error) {
	var ids []uint64
	if len(m.OrderIDsJSON) > 0 {
		if err := json.Unmarshal(m.OrderIDsJSON, &ids); err != nil {
			return nil, err
		}
	}
	return &biz.Withdraw{
		ID:             m.ID,
		UserID:         m.UserID,
		Amount:         m.Amount,
		FeeAmount:      m.FeeAmount,
		CreditedAmount: m.CreditedAmount,
		OrderIDs:       ids,
		Status:         m.Status,
		Remark:         m.Remark,
		TxHash:         m.TxHash,
		PayoutError:    m.PayoutError,
		ReviewedAt:     m.ReviewedAt,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}, nil
}

func (r *withdrawRepo) Create(ctx context.Context, w *biz.Withdraw) (*biz.Withdraw, error) {
	raw, err := json.Marshal(w.OrderIDs)
	if err != nil {
		return nil, err
	}
	m := WithdrawModel{
		UserID:         w.UserID,
		Amount:         w.Amount,
		FeeAmount:      w.FeeAmount,
		CreditedAmount: w.CreditedAmount,
		OrderIDsJSON:   raw,
		Status:         w.Status,
		Remark:         w.Remark,
	}
	if err := r.data.db.WithContext(ctx).Create(&m).Error; err != nil {
		return nil, err
	}
	return r.FindByID(ctx, m.ID)
}

func (r *withdrawRepo) FindByID(ctx context.Context, id uint64) (*biz.Withdraw, error) {
	var m WithdrawModel
	if err := r.data.db.WithContext(ctx).First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, biz.ErrWithdrawNotFound
		}
		return nil, err
	}
	return toBizWithdraw(&m)
}

func (r *withdrawRepo) ListByUser(ctx context.Context, userID uint64, status string) ([]*biz.Withdraw, error) {
	return r.ListFiltered(ctx, status, userID)
}

func (r *withdrawRepo) ListFiltered(ctx context.Context, status string, userID uint64) ([]*biz.Withdraw, error) {
	q := r.data.db.WithContext(ctx).Model(&WithdrawModel{})
	if status != "" && status != "all" {
		q = q.Where("status = ?", status)
	}
	if userID > 0 {
		q = q.Where("user_id = ?", userID)
	}
	var rows []WithdrawModel
	if err := q.Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Withdraw, 0, len(rows))
	for i := range rows {
		w, err := toBizWithdraw(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

func (r *withdrawRepo) ListPayoutQueue(ctx context.Context, limit int) ([]*biz.Withdraw, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []WithdrawModel
	err := r.data.db.WithContext(ctx).Model(&WithdrawModel{}).
		Where("status IN ?", []string{biz.WithdrawRewarded, biz.WithdrawApproved, biz.WithdrawDoing}).
		Order("id ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*biz.Withdraw, 0, len(rows))
	for i := range rows {
		w, err := toBizWithdraw(&rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

func (r *withdrawRepo) CountByStatuses(ctx context.Context, statuses ...string) (int64, error) {
	if len(statuses) == 0 {
		return 0, nil
	}
	var n int64
	err := r.data.db.WithContext(ctx).Model(&WithdrawModel{}).
		Where("status IN ?", statuses).
		Count(&n).Error
	return n, err
}

func (r *withdrawRepo) CasUpdateStatus(ctx context.Context, id uint64, fromStatus, toStatus, remark string) (bool, error) {
	updates := map[string]any{
		"status": toStatus,
	}
	if fromStatus == biz.WithdrawPending && (toStatus == biz.WithdrawRewarded || toStatus == biz.WithdrawRejected || toStatus == biz.WithdrawApproved) {
		updates["reviewed_at"] = gorm.Expr("CURRENT_TIMESTAMP(3)")
	}
	if remark != "" {
		updates["remark"] = remark
	}
	res := r.data.db.WithContext(ctx).Model(&WithdrawModel{}).
		Where("id = ? AND status = ?", id, fromStatus).
		Updates(updates)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// CasClaimPayout moves rewarded|approved → doing (atomic claim for payout worker).
func (r *withdrawRepo) CasClaimPayout(ctx context.Context, id uint64) (bool, error) {
	res := r.data.db.WithContext(ctx).Model(&WithdrawModel{}).
		Where("id = ? AND status IN ?", id, []string{biz.WithdrawRewarded, biz.WithdrawApproved}).
		Updates(map[string]any{
			"status":       biz.WithdrawDoing,
			"payout_error": "",
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *withdrawRepo) SetTxHash(ctx context.Context, id uint64, txHash string) error {
	return r.data.db.WithContext(ctx).Model(&WithdrawModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"tx_hash":      txHash,
			"payout_error": "",
		}).Error
}

func (r *withdrawRepo) MarkPass(ctx context.Context, id uint64) (bool, error) {
	res := r.data.db.WithContext(ctx).Model(&WithdrawModel{}).
		Where("id = ? AND status = ?", id, biz.WithdrawDoing).
		Where("tx_hash <> ?", "").
		Update("status", biz.WithdrawPass)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *withdrawRepo) SetPayoutError(ctx context.Context, id uint64, msg string) error {
	if len(msg) > 250 {
		msg = msg[:250]
	}
	return r.data.db.WithContext(ctx).Model(&WithdrawModel{}).
		Where("id = ?", id).
		Update("payout_error", msg).Error
}

func (r *withdrawRepo) SumApprovedAmount(ctx context.Context) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := r.data.db.WithContext(ctx).Model(&WithdrawModel{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("status IN ?", []string{biz.WithdrawRewarded, biz.WithdrawDoing, biz.WithdrawPass, biz.WithdrawApproved}).
		Scan(&sum).Error
	return sum, err
}

func (r *withdrawRepo) SumApprovedAmountBetween(ctx context.Context, from, to time.Time) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := r.data.db.WithContext(ctx).Model(&WithdrawModel{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("status IN ?", []string{biz.WithdrawRewarded, biz.WithdrawDoing, biz.WithdrawPass, biz.WithdrawApproved}).
		Where("COALESCE(reviewed_at, created_at) >= ? AND COALESCE(reviewed_at, created_at) < ?", from, to).
		Scan(&sum).Error
	return sum, err
}
