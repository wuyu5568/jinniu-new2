package server

import (
	"context"
	"log"
	"sync/atomic"

	"github.com/jinniu/app/app/app/internal/conf"
	"github.com/jinniu/app/app/app/internal/service"
	"github.com/robfig/cron/v3"
)

// DepositCron periodically syncs on-chain deposits into account balances.
type DepositCron struct {
	cronExpr string
	c        *cron.Cron
	svc      *service.AppService
	busy     atomic.Bool
}

// NewDepositCron builds deposit scheduler; no-op when expr empty.
func NewDepositCron(app *conf.App, svc *service.AppService) *DepositCron {
	return &DepositCron{
		cronExpr: app.DepositCron,
		svc:      svc,
	}
}

// Start begins the scheduler.
func (d *DepositCron) Start() {
	if d.cronExpr == "" {
		return
	}
	d.c = cron.New()
	_, err := d.c.AddFunc(d.cronExpr, func() {
		if !d.busy.CompareAndSwap(false, true) {
			log.Printf("deposit cron: skip, previous run still in progress")
			return
		}
		defer d.busy.Store(false)

		res, err := d.svc.RunChainDepositSync(context.Background())
		if err != nil {
			log.Printf("deposit cron: %v", err)
			return
		}
		if res == nil {
			return
		}
		if res.Pulled > 0 || res.Credited > 0 || res.Skipped > 0 || res.Errors > 0 {
			log.Printf("deposit cron: pulled=%d credited=%d skipped=%d errors=%d cursor=%d->%d caught_up=%v",
				res.Pulled, res.Credited, res.Skipped, res.Errors, res.CursorBefore, res.CursorAfter, res.CaughtUp)
		}
	})
	if err != nil {
		log.Printf("deposit cron schedule: %v", err)
		return
	}
	d.c.Start()
	log.Printf("deposit cron started: %s", d.cronExpr)
}

// Stop stops the scheduler.
func (d *DepositCron) Stop() {
	if d.c != nil {
		ctx := d.c.Stop()
		<-ctx.Done()
	}
}
