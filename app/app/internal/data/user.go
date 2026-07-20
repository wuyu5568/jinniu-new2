package data

import (
	"context"
	"errors"
	"time"

	"github.com/jinniu/app/app/app/internal/biz"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type userRepo struct{ data *Data }

func NewUserRepo(d *Data) biz.UserRepo { return &userRepo{data: d} }

func toBizUser(m *UserModel) *biz.User {
	return &biz.User{
		ID:                   m.ID,
		Address:              m.Address,
		InviterID:            m.InviterID,
		AccountBalance:       m.AccountBalance,
		WithdrawableBalance:  m.WithdrawableBalance,
		CommunityLevel:       m.CommunityLevel,
		CommunityVolume:      m.CommunityVolume,
		CommunityLevelLocked: m.CommunityLevelLocked,
		DisabledAt:           m.DisabledAt,
		RewardLocked:         m.RewardLocked,
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
	}
}

func (r *userRepo) FindByID(ctx context.Context, id uint64) (*biz.User, error) {
	var m UserModel
	if err := r.data.db.WithContext(ctx).First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, biz.ErrUserNotFound
		}
		return nil, err
	}
	return toBizUser(&m), nil
}

func (r *userRepo) FindByAddress(ctx context.Context, address string) (*biz.User, error) {
	var m UserModel
	if err := r.data.db.WithContext(ctx).Where("address = ?", address).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, biz.ErrUserNotFound
		}
		return nil, err
	}
	return toBizUser(&m), nil
}

func (r *userRepo) Create(ctx context.Context, u *biz.User) (*biz.User, error) {
	m := UserModel{
		Address:             u.Address,
		InviterID:           u.InviterID,
		AccountBalance:      u.AccountBalance,
		WithdrawableBalance: u.WithdrawableBalance,
		CommunityLevel:      u.CommunityLevel,
		CommunityVolume:     u.CommunityVolume,
		RewardLocked:        u.RewardLocked,
	}
	if err := r.data.db.WithContext(ctx).Create(&m).Error; err != nil {
		return nil, err
	}
	return r.FindByID(ctx, m.ID)
}

func (r *userRepo) ListAll(ctx context.Context) ([]*biz.User, error) {
	var rows []UserModel
	if err := r.data.db.WithContext(ctx).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.User, len(rows))
	for i := range rows {
		out[i] = toBizUser(&rows[i])
	}
	return out, nil
}

func (r *userRepo) CountDirectReferrals(ctx context.Context, userID uint64) (int, error) {
	var n int64
	err := r.data.db.WithContext(ctx).Model(&UserModel{}).Where("inviter_id = ?", userID).Count(&n).Error
	return int(n), err
}

func (r *userRepo) CountEffectiveDirectReferrals(ctx context.Context, userID uint64) (int, error) {
	var n int64
	err := r.data.db.WithContext(ctx).Model(&UserModel{}).
		Where("inviter_id = ?", userID).
		Where("EXISTS (SELECT 1 FROM locations WHERE locations.user_id = users.id)").
		Count(&n).Error
	return int(n), err
}

func (r *userRepo) UpdateCommunity(ctx context.Context, userID uint64, level uint8, volume decimal.Decimal) error {
	return r.data.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ?", userID).
		Updates(map[string]any{"community_level": level, "community_volume": volume}).Error
}

func (r *userRepo) UpdateCommunityVolume(ctx context.Context, userID uint64, volume decimal.Decimal) error {
	return r.data.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ?", userID).
		Update("community_volume", volume).Error
}

func (r *userRepo) ExistsByAddress(ctx context.Context, address string) (bool, error) {
	var n int64
	err := r.data.db.WithContext(ctx).Model(&UserModel{}).Where("address = ?", address).Count(&n).Error
	return n > 0, err
}

func (r *userRepo) ListPaged(ctx context.Context, address string, page, pageSize int) ([]*biz.User, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	q := r.data.db.WithContext(ctx).Model(&UserModel{})
	if address != "" {
		q = q.Where("address LIKE ?", "%"+address+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []UserModel
	offset := (page - 1) * pageSize
	if err := q.Order("id DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*biz.User, len(rows))
	for i := range rows {
		out[i] = toBizUser(&rows[i])
	}
	return out, int(total), nil
}

func (r *userRepo) ListByInviter(ctx context.Context, inviterID uint64) ([]*biz.User, error) {
	var rows []UserModel
	if err := r.data.db.WithContext(ctx).Where("inviter_id = ?", inviterID).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.User, len(rows))
	for i := range rows {
		out[i] = toBizUser(&rows[i])
	}
	return out, nil
}

func (r *userRepo) SoftDelete(ctx context.Context, userID uint64, at time.Time) error {
	res := r.data.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ?", userID).
		Update("disabled_at", at)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrUserNotFound
	}
	return nil
}

func (r *userRepo) Restore(ctx context.Context, userID uint64) error {
	res := r.data.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ?", userID).
		Update("disabled_at", nil)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrUserNotFound
	}
	return nil
}

func (r *userRepo) SetRewardLocked(ctx context.Context, userID uint64, locked bool) error {
	res := r.data.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ?", userID).
		Update("reward_locked", locked)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrUserNotFound
	}
	return nil
}

func (r *userRepo) SetCommunityLevel(ctx context.Context, userID uint64, level uint8, lock bool) error {
	res := r.data.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"community_level":        level,
			"community_level_locked": lock,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrUserNotFound
	}
	return nil
}

func (r *userRepo) SetCommunityLevelLocked(ctx context.Context, userID uint64, locked bool) error {
	res := r.data.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ?", userID).
		Update("community_level_locked", locked)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrUserNotFound
	}
	return nil
}

func (r *userRepo) ListByPathPrefix(ctx context.Context, pathPrefix string) ([]*biz.User, error) {
	var rows []UserModel
	err := r.data.db.WithContext(ctx).
		Joins("INNER JOIN user_recommends ur ON ur.user_id = users.id").
		Where("ur.path = ? OR ur.path LIKE ?", pathPrefix, pathPrefix+",%").
		Order("users.id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*biz.User, len(rows))
	for i := range rows {
		out[i] = toBizUser(&rows[i])
	}
	return out, nil
}

func (r *userRepo) CountAll(ctx context.Context) (int, error) {
	var n int64
	err := r.data.db.WithContext(ctx).Model(&UserModel{}).Count(&n).Error
	return int(n), err
}

func (r *userRepo) CountNonDisabled(ctx context.Context) (int, error) {
	var n int64
	err := r.data.db.WithContext(ctx).Model(&UserModel{}).Where("disabled_at IS NULL").Count(&n).Error
	return int(n), err
}

func (r *userRepo) CountCreatedBetween(ctx context.Context, from, to time.Time) (int, error) {
	var n int64
	err := r.data.db.WithContext(ctx).Model(&UserModel{}).
		Where("created_at >= ? AND created_at < ?", from, to).
		Count(&n).Error
	return int(n), err
}

func (r *userRepo) SumWithdrawableBalance(ctx context.Context) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := r.data.db.WithContext(ctx).Model(&UserModel{}).
		Select("COALESCE(SUM(withdrawable_balance), 0)").Scan(&sum).Error
	return sum, err
}

func (r *userRepo) SumAccountBalance(ctx context.Context) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := r.data.db.WithContext(ctx).Model(&UserModel{}).
		Select("COALESCE(SUM(account_balance), 0)").Scan(&sum).Error
	return sum, err
}

type userBalanceRepo struct{ data *Data }

func NewUserBalanceRepo(d *Data) biz.UserBalanceRepo { return &userBalanceRepo{data: d} }

func (r *userBalanceRepo) AddAccountBalance(ctx context.Context, userID uint64, delta decimal.Decimal) error {
	return r.addBalance(ctx, "account_balance", userID, delta)
}

func (r *userBalanceRepo) AddWithdrawableBalance(ctx context.Context, userID uint64, delta decimal.Decimal) error {
	return r.addBalance(ctx, "withdrawable_balance", userID, delta)
}

func (r *userBalanceRepo) SubAccountBalance(ctx context.Context, userID uint64, delta decimal.Decimal) error {
	return r.subBalance(ctx, "account_balance", userID, delta)
}

func (r *userBalanceRepo) SubWithdrawableBalance(ctx context.Context, userID uint64, delta decimal.Decimal) error {
	return r.subBalance(ctx, "withdrawable_balance", userID, delta)
}

func (r *userBalanceRepo) SetAccountBalance(ctx context.Context, userID uint64, amount decimal.Decimal) error {
	return r.setBalance(ctx, "account_balance", userID, amount)
}

func (r *userBalanceRepo) SetWithdrawableBalance(ctx context.Context, userID uint64, amount decimal.Decimal) error {
	return r.setBalance(ctx, "withdrawable_balance", userID, amount)
}

func (r *userBalanceRepo) addBalance(ctx context.Context, column string, userID uint64, delta decimal.Decimal) error {
	res := r.data.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ?", userID).
		Update(column, gorm.Expr(column+" + ?", delta))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrUserNotFound
	}
	return nil
}

func (r *userBalanceRepo) subBalance(ctx context.Context, column string, userID uint64, delta decimal.Decimal) error {
	res := r.data.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ? AND "+column+" >= ?", userID, delta).
		Update(column, gorm.Expr(column+" - ?", delta))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrInsufficientBalance
	}
	return nil
}

func (r *userBalanceRepo) setBalance(ctx context.Context, column string, userID uint64, amount decimal.Decimal) error {
	res := r.data.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ?", userID).
		Update(column, amount)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrUserNotFound
	}
	return nil
}
