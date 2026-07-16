package data

import (
	"context"
	"errors"
	"time"

	"github.com/jinniu/app/app/app/internal/biz"
	"gorm.io/gorm"
)

type recommendRepo struct{ data *Data }

func NewRecommendRepo(d *Data) biz.RecommendRepo { return &recommendRepo{data: d} }

func (r *recommendRepo) GetPath(ctx context.Context, userID uint64) (string, error) {
	var m UserRecommendModel
	err := r.data.db.WithContext(ctx).Where("user_id = ?", userID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return m.Path, nil
}

func (r *recommendRepo) SavePath(ctx context.Context, userID uint64, path string) error {
	var m UserRecommendModel
	err := r.data.db.WithContext(ctx).Where("user_id = ?", userID).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.data.db.WithContext(ctx).Create(&UserRecommendModel{UserID: userID, Path: path}).Error
	}
	if err != nil {
		return err
	}
	return r.data.db.WithContext(ctx).Model(&m).Update("path", path).Error
}

type loginChallengeRepo struct{ data *Data }

func NewLoginChallengeRepo(d *Data) biz.LoginChallengeRepo { return &loginChallengeRepo{data: d} }

func (r *loginChallengeRepo) Create(ctx context.Context, address, nonce string, expiresAt time.Time) error {
	return r.data.db.WithContext(ctx).Create(&LoginChallengeModel{
		Address:   address,
		Nonce:     nonce,
		ExpiresAt: expiresAt,
	}).Error
}

func (r *loginChallengeRepo) FindUsable(ctx context.Context, address, nonce string, now time.Time) (*biz.LoginChallenge, error) {
	var m LoginChallengeModel
	err := r.data.db.WithContext(ctx).
		Where("address = ? AND nonce = ? AND used_at IS NULL AND expires_at > ?", address, nonce, now).
		First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrInvalidNonce
	}
	if err != nil {
		return nil, err
	}
	return &biz.LoginChallenge{
		ID:        m.ID,
		Address:   m.Address,
		Nonce:     m.Nonce,
		ExpiresAt: m.ExpiresAt,
		UsedAt:    m.UsedAt,
		CreatedAt: m.CreatedAt,
	}, nil
}

func (r *loginChallengeRepo) MarkUsed(ctx context.Context, id uint64, usedAt time.Time) error {
	res := r.data.db.WithContext(ctx).Model(&LoginChallengeModel{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", usedAt)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrInvalidNonce
	}
	return nil
}
