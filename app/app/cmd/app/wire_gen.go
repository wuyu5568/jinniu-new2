// Code generated manually for P0. Run `wire` in cmd/app to regenerate.

package main

import (
	"context"
	"log"

	"github.com/go-kratos/kratos/v2"
	"github.com/jinniu/app/app/app/internal/biz"
	"github.com/jinniu/app/app/app/internal/conf"
	"github.com/jinniu/app/app/app/internal/data"
	"github.com/jinniu/app/app/app/internal/pkg/middleware/auth"
	"github.com/jinniu/app/app/app/internal/server"
	"github.com/jinniu/app/app/app/internal/service"
	"github.com/shopspring/decimal"
)

func newApp(cfg *conf.Bootstrap, d *data.Data) (*kratos.App, func(), error) {
	repos := provideRepos(d)

	userUC := biz.NewUserUseCase(
		repos.users,
		repos.balances,
		repos.recommends,
		repos.challenges,
		repos.ledger,
		auth.NewSignatureVerifier(),
		auth.NewTokenIssuer(&cfg.Auth),
		&cfg.Auth,
		cfg.App.GenesisAddress,
	)
	recordUC := biz.NewRecordUseCase(
		repos.locations,
		repos.withdraws,
		repos.ledger,
		repos.balances,
		repos.users,
		repos.recommends,
		repos.params,
		repos.ethUserRecord,
		repos.settleRuns,
		cfg.App.AllowForceSettle,
	)
	recordUC.SetDBPinger(d)
	recordUC.SetPayoutConfig(biz.PayoutConfig{
		Enabled:      cfg.App.PayoutEnabled,
		RPC:          cfg.App.BscRPC,
		USDT:         cfg.App.UsdtAddress,
		HotWalletKey: cfg.App.HotWalletKey,
		MaxUSDT:      decimal.NewFromFloat(cfg.App.PayoutMaxUSDT),
		CronExpr:     cfg.App.PayoutCron,
	})

	svc := service.NewAppService(userUC, recordUC, &cfg.Auth, &cfg.App)
	hs := server.NewHTTPServer(cfg, svc)
	settleCron := server.NewSettleCron(&cfg.App, recordUC)
	payoutCron := server.NewPayoutCron(&cfg.App, recordUC)
	return server.NewApp(cfg, hs, settleCron, payoutCron)
}

type repoBundle struct {
	users         biz.UserRepo
	balances      biz.UserBalanceRepo
	recommends    biz.RecommendRepo
	challenges    biz.LoginChallengeRepo
	locations     biz.LocationRepo
	withdraws     biz.WithdrawRepo
	ledger        biz.LedgerRepo
	params        biz.ParamsRepo
	ethUserRecord biz.EthUserRecordRepo
	settleRuns    biz.SettleRunRepo
}

func provideRepos(d *data.Data) repoBundle {
	params := data.NewParamsRepo(d)
	if p, err := params.Get(context.Background()); err == nil {
		biz.SetActiveParams(p)
	} else {
		biz.SetActiveParams(nil)
		log.Printf("load params: %v", err)
	}
	return repoBundle{
		users:         data.NewUserRepo(d),
		balances:      data.NewUserBalanceRepo(d),
		recommends:    data.NewRecommendRepo(d),
		challenges:    data.NewLoginChallengeRepo(d),
		locations:     data.NewLocationRepo(d),
		withdraws:     data.NewWithdrawRepo(d),
		ledger:        data.NewLedgerRepo(d),
		params:        params,
		ethUserRecord: data.NewEthUserRecordRepo(d),
		settleRuns:    data.NewSettleRunRepo(d),
	}
}
