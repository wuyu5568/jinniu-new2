package server

import (
	"context"
	"log"

	"github.com/jinniu/app/app/app/internal/biz"
	"github.com/jinniu/app/app/app/internal/conf"
	"github.com/robfig/cron/v3"
)

// PayoutCron optionally drains withdraw payout queue (ADR 0010).
type PayoutCron struct {
	cronExpr string
	c        *cron.Cron
	record   *biz.RecordUseCase
}

// NewPayoutCron builds payout scheduler; no-op when expr empty.
func NewPayoutCron(app *conf.App, record *biz.RecordUseCase) *PayoutCron {
	return &PayoutCron{
		cronExpr: app.PayoutCron,
		record:   record,
	}
}

// Start begins the scheduler.
func (p *PayoutCron) Start() {
	if p.cronExpr == "" || !p.recordHasPayout() {
		return
	}
	p.c = cron.New()
	_, err := p.c.AddFunc(p.cronExpr, func() {
		n, err := p.record.RunPayoutQueue(context.Background(), 10)
		if err != nil {
			log.Printf("payout cron: %v", err)
			return
		}
		if n > 0 {
			log.Printf("payout cron processed=%d", n)
		}
	})
	if err != nil {
		log.Printf("payout cron schedule: %v", err)
		return
	}
	p.c.Start()
}

func (p *PayoutCron) recordHasPayout() bool {
	// Enabled is checked inside RunPayoutQueue; still start cron if expr set so ops can toggle via restart+config.
	return true
}

// Stop stops the scheduler.
func (p *PayoutCron) Stop() {
	if p.c != nil {
		ctx := p.c.Stop()
		<-ctx.Done()
	}
}
