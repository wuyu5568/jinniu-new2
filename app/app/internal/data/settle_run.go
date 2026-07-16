package data

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jinniu/app/app/app/internal/biz"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type settleRunModel struct {
	ID               uint64    `gorm:"primaryKey"`
	SettleDate       time.Time `gorm:"column:settle_date;type:date;uniqueIndex"`
	Forced           bool      `gorm:"column:forced"`
	SettledCount     int       `gorm:"column:settled_count"`
	ExitedCount      int       `gorm:"column:exited_count"`
	GenerationCount  int       `gorm:"column:generation_count"`
	CommunityCount   int       `gorm:"column:community_count"`
	PeerCount        int       `gorm:"column:peer_count"`
	Remark           string    `gorm:"column:remark"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (settleRunModel) TableName() string { return "settle_runs" }

type settleRunRepo struct{ data *Data }

func NewSettleRunRepo(d *Data) biz.SettleRunRepo { return &settleRunRepo{data: d} }

func toSettleRun(row settleRunModel) *biz.SettleRun {
	return &biz.SettleRun{
		ID:              row.ID,
		SettleDate:      row.SettleDate,
		Forced:          row.Forced,
		SettledCount:    row.SettledCount,
		ExitedCount:     row.ExitedCount,
		GenerationCount: row.GenerationCount,
		CommunityCount:  row.CommunityCount,
		PeerCount:       row.PeerCount,
		Remark:          row.Remark,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func (r *settleRunRepo) FindByDate(ctx context.Context, settleDate time.Time) (*biz.SettleRun, error) {
	day := time.Date(settleDate.Year(), settleDate.Month(), settleDate.Day(), 0, 0, 0, 0, time.UTC)
	var row settleRunModel
	err := r.data.db.WithContext(ctx).Where("settle_date = ?", day.Format("2006-01-02")).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toSettleRun(row), nil
}

func (r *settleRunRepo) FindLatest(ctx context.Context) (*biz.SettleRun, error) {
	var row settleRunModel
	err := r.data.db.WithContext(ctx).Order("settle_date DESC").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toSettleRun(row), nil
}

func (r *settleRunRepo) TryClaim(ctx context.Context, settleDate time.Time, remark string) (bool, error) {
	day := time.Date(settleDate.Year(), settleDate.Month(), settleDate.Day(), 0, 0, 0, 0, time.UTC)
	row := settleRunModel{
		SettleDate: day,
		Forced:     false,
		Remark:     remark,
	}
	err := r.data.db.WithContext(ctx).Create(&row).Error
	if err != nil {
		if isDuplicateKey(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return true
	}
	// Fallback for drivers that wrap the message only.
	msg := err.Error()
	return strings.Contains(msg, "Duplicate entry") || strings.Contains(msg, "1062")
}

func (r *settleRunRepo) Upsert(ctx context.Context, run *biz.SettleRun) error {
	day := time.Date(run.SettleDate.Year(), run.SettleDate.Month(), run.SettleDate.Day(), 0, 0, 0, 0, time.UTC)
	row := settleRunModel{
		SettleDate:      day,
		Forced:          run.Forced,
		SettledCount:    run.SettledCount,
		ExitedCount:     run.ExitedCount,
		GenerationCount: run.GenerationCount,
		CommunityCount:  run.CommunityCount,
		PeerCount:       run.PeerCount,
		Remark:          run.Remark,
	}
	return r.data.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "settle_date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"forced", "settled_count", "exited_count", "generation_count",
			"community_count", "peer_count", "remark", "updated_at",
		}),
	}).Create(&row).Error
}
