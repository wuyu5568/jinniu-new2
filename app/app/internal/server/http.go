package server

import (
	stdhttp "net/http"

	"github.com/go-kratos/kratos/v2/middleware/recovery"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/jinniu/app/app/app/internal/conf"
	"github.com/jinniu/app/app/app/internal/pkg/middleware/auth"
	"github.com/jinniu/app/app/app/internal/service"
)

// NewHTTPServer creates the Kratos HTTP server with hand-written routes.
func NewHTTPServer(cfg *conf.Bootstrap, svc *service.AppService) *khttp.Server {
	var opts []khttp.ServerOption
	opts = append(opts, khttp.Middleware(recovery.Recovery()))
	opts = append(opts, khttp.Filter(corsFilter()))
	if cfg.Server.HTTP.Addr != "" {
		opts = append(opts, khttp.Address(cfg.Server.HTTP.Addr))
	}
	if cfg.Server.HTTP.Timeout > 0 {
		opts = append(opts, khttp.Timeout(cfg.Server.HTTP.Timeout))
	}
	srv := khttp.NewServer(opts...)
	registerHTTPRoutes(srv, cfg, svc)
	return srv
}

func corsFilter() func(stdhttp.Handler) stdhttp.Handler {
	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				origin = "*"
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			if r.Method == stdhttp.MethodOptions {
				w.WriteHeader(stdhttp.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func registerHTTPRoutes(srv *khttp.Server, cfg *conf.Bootstrap, svc *service.AppService) {
	srv.Handle("/health", stdhttp.HandlerFunc(svc.Health))

	jwt := cfg.Auth.JWTKey

	// User compat routes (taurus style, no /v1/)
	app := srv.Route("/api/app_server")
	app.POST("/eth_authorize", wrapStd(svc.CompatEthAuthorize))
	app.GET("/subscribe_tiers", wrapStd(svc.CompatSubscribeTiers))
	app.GET("/user_info", wrapStd(auth.RequireJWT(jwt, svc.CompatUserInfo)))
	app.POST("/buy", wrapStd(auth.RequireJWT(jwt, svc.CompatBuy)))
	app.GET("/order_list", wrapStd(auth.RequireJWT(jwt, svc.CompatOrderList)))
	app.GET("/location_list", wrapStd(auth.RequireJWT(jwt, svc.CompatLocationList)))
	app.POST("/withdraw", wrapStd(auth.RequireJWT(jwt, svc.CompatWithdraw)))
	app.GET("/withdraw_list", wrapStd(auth.RequireJWT(jwt, svc.CompatWithdrawList)))
	app.POST("/withdraw_cancel", wrapStd(auth.RequireJWT(jwt, svc.CompatWithdrawCancel)))
	app.GET("/recommend_list", wrapStd(auth.RequireJWT(jwt, svc.CompatRecommendList)))
	app.GET("/reward_list", wrapStd(auth.RequireJWT(jwt, svc.CompatRewardList)))
	app.GET("/deposit_list", wrapStd(auth.RequireJWT(jwt, svc.CompatDepositList)))

	// User v1 routes (nonce login + REST)
	app.POST("/v1/login/challenge", wrapStd(svc.LoginChallenge))
	app.POST("/v1/login/verify", wrapStd(svc.LoginVerify))
	app.GET("/v1/auth/check", wrapStd(svc.CheckAddress))
	app.GET("/v1/me", wrapStd(auth.RequireJWT(jwt, svc.Me)))
	app.POST("/v1/locations", wrapStd(auth.RequireJWT(jwt, svc.CreateLocation)))
	app.GET("/v1/locations", wrapStd(auth.RequireJWT(jwt, svc.ListLocations)))
	app.POST("/v1/withdraws", wrapStd(auth.RequireJWT(jwt, svc.CreateWithdraw)))
	app.GET("/v1/withdraws", wrapStd(auth.RequireJWT(jwt, svc.ListWithdraws)))
	app.POST("/v1/withdraws/{id}/cancel", wrapStd(auth.RequireJWT(jwt, svc.CancelWithdraw)))
	app.GET("/v1/ledger", wrapStd(auth.RequireJWT(jwt, svc.ListLedger)))

	// Admin compat routes (dapp-admin style)
	adm := srv.Route("/api/admin_jinniu")
	adm.POST("/login", wrapStd(svc.CompatAdminLogin))
	adm.GET("/my_auth_list", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatMyAuthList)))
	adm.GET("/all", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminAll)))
	adm.GET("/user_list", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminUserList)))
	adm.GET("/location_list", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminLocationList)))
	adm.GET("/buy_list", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminBuyList)))
	adm.GET("/withdraw_list", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminWithdrawList)))
	adm.POST("/withdraw_pass", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminWithdrawPass)))
	adm.POST("/withdraw_reject", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminWithdrawReject)))
	adm.POST("/withdraw_cancel_pending", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminWithdrawCancelPending)))
	adm.POST("/withdraw_payout", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminWithdrawPayout)))
	adm.POST("/withdraw_payout_confirm", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminWithdrawPayoutConfirm)))
	adm.POST("/withdraw_payout_run", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminWithdrawPayoutRun)))
	adm.POST("/settle", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminSettle)))
	adm.GET("/settle_status", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminSettleStatus)))
	adm.GET("/payout_status", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminPayoutStatus)))
	adm.GET("/config", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminConfig)))
	adm.POST("/config_update", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminConfigUpdate)))
	adm.GET("/reward_list", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminRewardList)))
	adm.GET("/record_list", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminRecordList)))
	adm.GET("/deposit", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminDepositChain)))
	adm.POST("/deposit", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminDeposit)))
	adm.POST("/deposit_replay", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminDepositReplay)))
	adm.POST("/add_money_two", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminAddMoneyTwo)))
	adm.POST("/add_money_three", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminAddMoneyThree)))
	adm.POST("/vip_update", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminVIPUpdate)))
	adm.POST("/vip_unlock", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminVIPUnlock)))
	adm.POST("/lock_user", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminLockUser)))
	adm.POST("/lock_user_reward", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminLockUserReward)))
	adm.GET("/user_recommend", wrapStd(auth.RequireAdminJWT(jwt, svc.CompatAdminUserRecommend)))

	// Admin v1 routes (JWT)
	adm.POST("/v1/users/recharge", wrapStd(auth.RequireAdminJWT(jwt, svc.AdminRecharge)))
	adm.POST("/v1/settle/run", wrapStd(auth.RequireAdminJWT(jwt, svc.AdminSettleRun)))
	adm.GET("/v1/withdraws", wrapStd(auth.RequireAdminJWT(jwt, svc.AdminListWithdraws)))
	adm.POST("/v1/withdraws/{id}/approve", wrapStd(auth.RequireAdminJWT(jwt, svc.AdminApproveWithdraw)))
	adm.POST("/v1/withdraws/{id}/reject", wrapStd(auth.RequireAdminJWT(jwt, svc.AdminRejectWithdraw)))
	adm.GET("/v1/business_configs", wrapStd(auth.RequireAdminJWT(jwt, svc.AdminListConfigs)))
	adm.PUT("/v1/business_configs/{id}", wrapStd(auth.RequireAdminJWT(jwt, svc.AdminUpdateConfig)))
}

func wrapStd(h stdhttp.HandlerFunc) khttp.HandlerFunc {
	return func(ctx khttp.Context) error {
		h(ctx.Response(), ctx.Request())
		return nil
	}
}
