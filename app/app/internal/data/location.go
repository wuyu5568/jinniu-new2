package data

import (
	"context"
	"errors"
	"time"

	"github.com/jinniu/app/app/app/internal/biz"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type locationRepo struct{ data *Data }

func NewLocationRepo(d *Data) biz.LocationRepo { return &locationRepo{data: d} }

func toBizLocation(m *LocationModel) *biz.Location {
	return &biz.Location{
		ID:              m.ID,
		UserID:          m.UserID,
		Amount:          m.Amount,
		Multiplier:      m.Multiplier,
		ExitTarget:      m.ExitTarget,
		Accumulated:     m.Accumulated,
		Status:          m.Status,
		RatePercent:     m.RatePercent,
		RateDirection:   m.RateDirection,
		RateTurnPending: m.RateTurnPending,
		LastSettledRate: m.LastSettledRate,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}
}

func (r *locationRepo) Create(ctx context.Context, loc *biz.Location) (*biz.Location, error) {
	m := LocationModel{
		UserID:          loc.UserID,
		Amount:          loc.Amount,
		Multiplier:      loc.Multiplier,
		ExitTarget:      loc.ExitTarget,
		Accumulated:     loc.Accumulated,
		Status:          loc.Status,
		RatePercent:     loc.RatePercent,
		RateDirection:   loc.RateDirection,
		RateTurnPending: loc.RateTurnPending,
		LastSettledRate: loc.LastSettledRate,
	}
	if err := r.data.db.WithContext(ctx).Create(&m).Error; err != nil {
		return nil, err
	}
	return r.FindByID(ctx, m.ID)
}

func (r *locationRepo) FindByID(ctx context.Context, id uint64) (*biz.Location, error) {
	var m LocationModel
	if err := r.data.db.WithContext(ctx).First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, biz.ErrLocationNotFound
		}
		return nil, err
	}
	return toBizLocation(&m), nil
}

func (r *locationRepo) Update(ctx context.Context, loc *biz.Location) (*biz.Location, error) {
	updates := map[string]any{
		"accumulated":       loc.Accumulated,
		"status":            loc.Status,
		"rate_percent":      loc.RatePercent,
		"rate_direction":    loc.RateDirection,
		"rate_turn_pending": loc.RateTurnPending,
	}
	if loc.LastSettledRate != nil {
		updates["last_settled_rate"] = *loc.LastSettledRate
	}
	res := r.data.db.WithContext(ctx).Model(&LocationModel{}).Where("id = ?", loc.ID).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, biz.ErrLocationNotFound
	}
	return r.FindByID(ctx, loc.ID)
}

func (r *locationRepo) ListByUser(ctx context.Context, userID uint64, status string) ([]*biz.Location, error) {
	q := r.data.db.WithContext(ctx).Where("user_id = ?", userID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []LocationModel
	if err := q.Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Location, len(rows))
	for i := range rows {
		out[i] = toBizLocation(&rows[i])
	}
	return out, nil
}

func (r *locationRepo) ListActive(ctx context.Context, orderIDs []uint64) ([]*biz.Location, error) {
	q := r.data.db.WithContext(ctx).Where("status = ?", biz.OrderStatusActive)
	if len(orderIDs) > 0 {
		q = q.Where("id IN ?", orderIDs)
	}
	var rows []LocationModel
	if err := q.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Location, len(rows))
	for i := range rows {
		out[i] = toBizLocation(&rows[i])
	}
	return out, nil
}

func (r *locationRepo) FindEarliestActive(ctx context.Context, userID uint64) (*biz.Location, error) {
	var m LocationModel
	err := r.data.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, biz.OrderStatusActive).
		Order("id ASC").First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrLocationNotFound
	}
	if err != nil {
		return nil, err
	}
	return toBizLocation(&m), nil
}

func (r *locationRepo) SumAmountsByUser(ctx context.Context) (map[uint64]decimal.Decimal, error) {
	type row struct {
		UserID uint64
		Sum    decimal.Decimal
	}
	var rows []row
	if err := r.data.db.WithContext(ctx).Model(&LocationModel{}).
		Select("user_id, COALESCE(SUM(amount), 0) as sum").
		Group("user_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint64]decimal.Decimal, len(rows))
	for _, r := range rows {
		out[r.UserID] = r.Sum
	}
	return out, nil
}

func (r *locationRepo) ListAllPaged(ctx context.Context, address string, page, pageSize int) ([]*biz.Location, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	base := r.data.db.WithContext(ctx).Table("locations").
		Joins("LEFT JOIN users ON users.id = locations.user_id")
	if address != "" {
		base = base.Where("users.address LIKE ?", "%"+address+"%")
	}
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []LocationModel
	offset := (page - 1) * pageSize
	if err := base.Session(&gorm.Session{}).
		Select("locations.id, locations.user_id, locations.amount, locations.multiplier, locations.exit_target, locations.accumulated, locations.status, locations.rate_percent, locations.rate_direction, locations.rate_turn_pending, locations.last_settled_rate, locations.created_at, locations.updated_at").
		Order("locations.id DESC").Offset(offset).Limit(pageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*biz.Location, len(rows))
	for i := range rows {
		out[i] = toBizLocation(&rows[i])
	}
	return out, int(total), nil
}

func (r *locationRepo) SumActiveByUser(ctx context.Context, userID uint64) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := r.data.db.WithContext(ctx).Model(&LocationModel{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("user_id = ? AND status = ?", userID, biz.OrderStatusActive).
		Scan(&sum).Error
	return sum, err
}

func (r *locationRepo) SumAmountByUser(ctx context.Context, userID uint64) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := r.data.db.WithContext(ctx).Model(&LocationModel{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("user_id = ?", userID).
		Scan(&sum).Error
	return sum, err
}

func (r *locationRepo) ExistsByUser(ctx context.Context, userID uint64) (bool, error) {
	var n int64
	err := r.data.db.WithContext(ctx).Model(&LocationModel{}).
		Where("user_id = ?", userID).Limit(1).Count(&n).Error
	return n > 0, err
}

func (r *locationRepo) SumAllAmount(ctx context.Context) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := r.data.db.WithContext(ctx).Model(&LocationModel{}).
		Select("COALESCE(SUM(amount), 0)").Scan(&sum).Error
	return sum, err
}

func (r *locationRepo) SumAmountCreatedBetween(ctx context.Context, from, to time.Time) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := r.data.db.WithContext(ctx).Model(&LocationModel{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("created_at >= ? AND created_at < ?", from, to).
		Scan(&sum).Error
	return sum, err
}

func (r *locationRepo) SumAmountByPathPrefix(ctx context.Context, pathPrefix string) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := r.data.db.WithContext(ctx).Table("locations").
		Select("COALESCE(SUM(locations.amount), 0)").
		Joins("INNER JOIN user_recommends ur ON ur.user_id = locations.user_id").
		Where("ur.path = ? OR ur.path LIKE ?", pathPrefix, pathPrefix+",%").
		Scan(&sum).Error
	return sum, err
}

func (r *locationRepo) CountDistinctUsers(ctx context.Context) (int, error) {
	var n int64
	err := r.data.db.WithContext(ctx).Model(&LocationModel{}).
		Select("COUNT(DISTINCT user_id)").Scan(&n).Error
	return int(n), err
}

func (r *locationRepo) CountUsersFirstCreatedBetween(ctx context.Context, from, to time.Time) (int, error) {
	var n int64
	err := r.data.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM (
			SELECT user_id FROM locations
			GROUP BY user_id
			HAVING MIN(created_at) >= ? AND MIN(created_at) < ?
		) AS first_loc
	`, from, to).Scan(&n).Error
	return int(n), err
}
