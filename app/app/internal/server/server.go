package server

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/wire"
	"github.com/jinniu/app/app/app/internal/conf"
)

// ProviderSet is the server layer wire set.
var ProviderSet = wire.NewSet(NewHTTPServer, NewSettleCron, NewPayoutCron, NewApp)

// NewApp wires Kratos application (HTTP only for P0 skeleton).
func NewApp(cfg *conf.Bootstrap, hs *http.Server, cron *SettleCron, payout *PayoutCron) (*kratos.App, func(), error) {
	cron.Start()
	payout.Start()
	app := kratos.New(
		kratos.Name("jinniu-app"),
		kratos.Server(hs),
	)
	cleanup := func() {
		cron.Stop()
		payout.Stop()
	}
	return app, cleanup, nil
}