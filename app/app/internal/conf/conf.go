package conf

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Bootstrap is the root config loaded from YAML (no protoc).
type Bootstrap struct {
	Server Server `yaml:"server"`
	Data   Data   `yaml:"data"`
	Auth   Auth   `yaml:"auth"`
	App    App    `yaml:"app"`
}

type Server struct {
	HTTP HTTP `yaml:"http"`
}

type HTTP struct {
	Addr    string        `yaml:"addr"`
	Timeout time.Duration `yaml:"timeout"`
}

type Data struct {
	Database Database `yaml:"database"`
}

type Database struct {
	Driver string `yaml:"driver"`
	Source string `yaml:"source"`
}

type Auth struct {
	JWTKey        string        `yaml:"jwt_key"`
	AdminUsername string        `yaml:"admin_username"`
	AdminPassword string        `yaml:"admin_password"`
	ChallengeTTL  time.Duration `yaml:"challenge_ttl"`
}

type App struct {
	GenesisAddress   string `yaml:"genesis_address"`
	SettleCron       string `yaml:"settle_cron"`
	SettleTimezone   string `yaml:"settle_timezone"`
	AllowForceSettle bool   `yaml:"allow_force_settle"` // 生产保持 false；本地测 force=1 时打开
	// 提取打款（ADR 0010）；生产开启前务必配置热钱包与 RPC
	PayoutEnabled bool   `yaml:"payout_enabled"`
	PayoutCron    string `yaml:"payout_cron"` // 空则不开定时打款
	BscRPC        string `yaml:"bsc_rpc"`
	UsdtAddress   string `yaml:"usdt_address"`
	HotWalletKey  string `yaml:"hot_wallet_key"` // hex，勿提交真实密钥；可用环境变量覆盖
	// PayoutMaxUSDT from env only (JINNIU_PAYOUT_MAX_USDT); 0 = no cap
	PayoutMaxUSDT float64 `yaml:"-"`
}

// Env overrides (K1 / E1): never log secret values.
const (
	EnvPayoutEnabled = "JINNIU_PAYOUT_ENABLED" // 1|true|yes|on
	EnvHotWalletKey  = "JINNIU_HOT_WALLET_KEY"  // hex private key, optional 0x prefix
	EnvBscRPC        = "JINNIU_BSC_RPC"         // optional RPC override
	EnvPayoutMaxUSDT = "JINNIU_PAYOUT_MAX_USDT" // e.g. 1 — reject credited_amount above this
)

// Load reads and parses a YAML config file, then applies env overrides.
func Load(path string) (*Bootstrap, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var bc Bootstrap
	if err := yaml.Unmarshal(raw, &bc); err != nil {
		return nil, err
	}
	applyEnvOverrides(&bc)
	if err := ValidateAppSafety(&bc.App); err != nil {
		return nil, err
	}
	return &bc, nil
}

// ValidateAppSafety enforces production payout defaults (P3).
// payout_enabled requires JINNIU_PAYOUT_MAX_USDT > 0.
func ValidateAppSafety(app *App) error {
	if app == nil {
		return nil
	}
	if app.PayoutEnabled && app.PayoutMaxUSDT <= 0 {
		return fmt.Errorf("payout_enabled requires %s > 0 (see docs/ops-payout.md)", EnvPayoutMaxUSDT)
	}
	return nil
}

func applyEnvOverrides(bc *Bootstrap) {
	if v := strings.TrimSpace(os.Getenv(EnvHotWalletKey)); v != "" {
		bc.App.HotWalletKey = v
	}
	if v := strings.TrimSpace(os.Getenv(EnvBscRPC)); v != "" {
		bc.App.BscRPC = v
	}
	if v := strings.TrimSpace(os.Getenv(EnvPayoutEnabled)); v != "" {
		bc.App.PayoutEnabled = envTruthy(v)
	}
	if v := strings.TrimSpace(os.Getenv(EnvPayoutMaxUSDT)); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			bc.App.PayoutMaxUSDT = f
		}
	}
}

func envTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
