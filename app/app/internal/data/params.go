package data

import (
	"context"
	"errors"

	"github.com/jinniu/app/app/app/internal/biz"
	"gorm.io/gorm"
)

type paramsRepo struct{ data *Data }

func NewParamsRepo(d *Data) biz.ParamsRepo { return &paramsRepo{data: d} }

func (r *paramsRepo) loadMap(ctx context.Context) (map[string]string, error) {
	var rows []BusinessConfigModel
	if err := r.data.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, row := range rows {
		m[row.ConfigKey] = row.Value
	}
	return m, nil
}

func (r *paramsRepo) seedIfEmpty(ctx context.Context) error {
	var n int64
	if err := r.data.db.WithContext(ctx).Model(&BusinessConfigModel{}).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	def := biz.DefaultBusinessParams()
	return r.Save(ctx, &def)
}

func (r *paramsRepo) Get(ctx context.Context) (*biz.BusinessParams, error) {
	if err := r.seedIfEmpty(ctx); err != nil {
		def := biz.DefaultBusinessParams()
		return &def, nil
	}
	m, err := r.loadMap(ctx)
	if err != nil {
		def := biz.DefaultBusinessParams()
		return &def, nil
	}
	p, err := biz.AggregateBusinessParams(m)
	if err != nil {
		def := biz.DefaultBusinessParams()
		return &def, nil
	}
	return p, nil
}

func (r *paramsRepo) Save(ctx context.Context, p *biz.BusinessParams) error {
	seeds := biz.FlattenBusinessParams(p)
	return r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, s := range seeds {
			row := BusinessConfigModel{
				ConfigKey: s.Key,
				Name:      s.Name,
				Value:     s.Value,
				SortOrder: s.Sort,
			}
			if err := tx.Where("config_key = ?", s.Key).
				Assign(map[string]any{"name": s.Name, "value": s.Value, "sort_order": s.Sort}).
				FirstOrCreate(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *paramsRepo) ListConfigs(ctx context.Context) ([]*biz.BusinessConfig, error) {
	if err := r.seedIfEmpty(ctx); err != nil {
		return nil, err
	}
	// Repair display names (DB may have mojibake from non-UTF8 inserts).
	labels := biz.ConfigLabels()
	for key, name := range labels {
		_ = r.data.db.WithContext(ctx).Model(&BusinessConfigModel{}).
			Where("config_key = ? AND name <> ?", key, name).
			Update("name", name).Error
	}
	var rows []BusinessConfigModel
	if err := r.data.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.BusinessConfig, len(rows))
	for i := range rows {
		name := rows[i].Name
		if n, ok := labels[rows[i].ConfigKey]; ok {
			name = n
		}
		out[i] = &biz.BusinessConfig{
			ID:    rows[i].ID,
			Key:   rows[i].ConfigKey,
			Name:  name,
			Value: rows[i].Value,
		}
	}
	return out, nil
}

func (r *paramsRepo) UpdateConfigValue(ctx context.Context, id uint64, value string) (*biz.BusinessConfig, error) {
	configs, err := r.ListConfigs(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(configs))
	var target *biz.BusinessConfig
	for _, c := range configs {
		m[c.Key] = c.Value
		if c.ID == id {
			cp := *c
			cp.Value = value
			target = &cp
			m[c.Key] = value
		}
	}
	if target == nil {
		return nil, gorm.ErrRecordNotFound
	}
	p, err := biz.AggregateBusinessParams(m)
	if err != nil {
		return nil, err
	}
	if err := biz.ValidateBusinessParams(p); err != nil {
		return nil, err
	}
	res := r.data.db.WithContext(ctx).Model(&BusinessConfigModel{}).Where("id = ?", id).Update("value", value)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, errors.New("config not found")
	}
	return target, nil
}
