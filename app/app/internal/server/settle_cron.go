package server

import (
	"context"
	"log"
	"time"

	"github.com/jinniu/app/app/app/internal/biz"
	"github.com/jinniu/app/app/app/internal/conf"
	"github.com/robfig/cron/v3"
)

// SettleCron schedules daily settlement.
type SettleCron struct {
	cronExpr string
	loc      *time.Location
	c        *cron.Cron
	record   *biz.RecordUseCase
}

// NewSettleCron builds cron from app config.
func NewSettleCron(app *conf.App, record *biz.RecordUseCase) *SettleCron {
	loc, err := time.LoadLocation(app.SettleTimezone)
	if err != nil {
		loc = time.UTC
	}
	return &SettleCron{
		cronExpr: app.SettleCron,
		loc:      loc,
		record:   record,
	}
}

// Start begins the scheduler; no-op if cron expression is empty.
func (s *SettleCron) Start() {
	if s.cronExpr == "" {
		return
	}
	s.c = cron.New(cron.WithLocation(s.loc))
	_, err := s.c.AddFunc(s.cronExpr, func() {
		ctx := context.Background()
		res, err := s.record.SettleStatic(ctx, nil, false)
		if err != nil {
			log.Printf("settle cron error: %v", err)
			return
		}
		if res.Skipped {
			log.Printf("settle cron skipped: already settled date=%s", res.SettleDate)
			return
		}
		log.Printf("settle cron done: settled=%d exited=%d gen=%d community=%d peer=%d",
			res.SettledCount, res.ExitedCount, res.GenerationCount, res.CommunityCount, res.PeerCount)
	})
	if err != nil {
		log.Printf("settle cron: %v", err)
		return
	}
	s.c.Start()
}

// Stop stops the scheduler.
func (s *SettleCron) Stop() {
	if s.c != nil {
		ctx := s.c.Stop()
		<-ctx.Done()
	}
}
