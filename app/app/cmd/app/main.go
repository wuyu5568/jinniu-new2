package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jinniu/app/app/app/internal/conf"
	"github.com/jinniu/app/app/app/internal/data"
)

var confPath string

func init() {
	flag.StringVar(&confPath, "conf", "configs/config.yaml", "config path, e.g. app/app/configs/config.yaml")
}

func main() {
	flag.Parse()

	cfg, err := conf.Load(confPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if cfg.App.AllowForceSettle {
		log.Printf("WARN: allow_force_settle=true (disable in production)")
	}
	if cfg.App.PayoutEnabled {
		log.Printf("payout enabled; max_usdt=%v hot_key_set=%v", cfg.App.PayoutMaxUSDT, cfg.App.HotWalletKey != "")
	}

	d, cleanupData, err := data.NewData(&cfg.Data)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer cleanupData()

	kapp, cleanupApp, err := newApp(cfg, d)
	if err != nil {
		log.Fatalf("wire app: %v", err)
	}
	defer cleanupApp()

	go func() {
		if err := kapp.Run(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	if err := kapp.Stop(); err != nil {
		log.Printf("stop: %v", err)
	}
}