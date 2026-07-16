package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/jinniu/app/app/app/internal/biz"
	"github.com/jinniu/app/app/app/internal/conf"
	"github.com/jinniu/app/app/app/internal/pkg/middleware/auth"
	"github.com/shopspring/decimal"
)

// AppService exposes HTTP handlers (manual routes, no generated pb).
type AppService struct {
	users  *biz.UserUseCase
	record *biz.RecordUseCase
	auth   *conf.Auth
	app    *conf.App
}

// NewAppService creates AppService.
func NewAppService(users *biz.UserUseCase, record *biz.RecordUseCase, auth *conf.Auth, app *conf.App) *AppService {
	return &AppService{users: users, record: record, auth: auth, app: app}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONArray(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	raw, _ := json.Marshal(v)
	w.Write(raw)
}

func writeBizError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	var ke *kerrors.Error
	if errors.As(err, &ke) {
		writeJSON(w, int(ke.Code), map[string]string{"message": ke.Message})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"message": err.Error()})
}

func (s *AppService) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.record.PingDB(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":  "degraded",
			"db":      "fail",
			"message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"db":     "ok",
	})
}

func (s *AppService) LoginChallenge(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid json"})
		return
	}
	msg, nonce, exp, err := s.users.CreateLoginChallenge(r.Context(), body.Address)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message":    msg,
		"nonce":      nonce,
		"expires_at": exp.Format(time.RFC3339),
	})
}

func (s *AppService) LoginVerify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Address    string `json:"address"`
		Signature  string `json:"signature"`
		Nonce      string `json:"nonce"`
		InviteCode string `json:"invite_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid json"})
		return
	}
	if body.Nonce == "" {
		writeBizError(w, biz.ErrInvalidNonce)
		return
	}
	res, err := s.users.Login(r.Context(), body.Address, body.Signature, body.Nonce, body.InviteCode)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": res.Token,
		"user":  userDTO(res.User),
	})
}

func (s *AppService) CheckAddress(w http.ResponseWriter, r *http.Request) {
	addr := r.URL.Query().Get("address")
	reg, isGenesis, err := s.users.CheckAddress(r.Context(), addr)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"registered":  reg,
		"is_genesis":  isGenesis,
	})
}

func (s *AppService) Me(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	user, err := s.users.GetProfile(r.Context(), uid)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userDTO(user))
}

func (s *AppService) CreateLocation(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	var body struct {
		Amount string `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid json"})
		return
	}
	amount, err := decimal.NewFromString(body.Amount)
	if err != nil {
		writeBizError(w, biz.ErrInvalidAmount)
		return
	}
	loc, err := s.record.CreateLocation(r.Context(), uid, amount)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, locationDTO(loc))
}

func (s *AppService) ListLocations(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	status := r.URL.Query().Get("status")
	locs, err := s.record.ListLocations(r.Context(), uid, status)
	if err != nil {
		writeBizError(w, err)
		return
	}
	items := make([]any, len(locs))
	for i, l := range locs {
		items[i] = locationDTO(l)
	}
	writeJSONArray(w, http.StatusOK, items)
}

func (s *AppService) CreateWithdraw(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	var body struct {
		Amount   string   `json:"amount"`
		OrderIDs []uint64 `json:"order_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid json"})
		return
	}
	amount, err := decimal.NewFromString(body.Amount)
	if err != nil {
		writeBizError(w, biz.ErrInvalidAmount)
		return
	}
	wd, err := s.record.CreateWithdraw(r.Context(), uid, amount, body.OrderIDs)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, withdrawDTO(wd))
}

func (s *AppService) ListWithdraws(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	status := r.URL.Query().Get("status")
	items, err := s.record.ListWithdraws(r.Context(), uid, status)
	if err != nil {
		writeBizError(w, err)
		return
	}
	out := make([]any, len(items))
	for i, w := range items {
		out[i] = withdrawDTO(w)
	}
	writeJSONArray(w, http.StatusOK, out)
}

func (s *AppService) CancelWithdraw(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	id, err := pathUint64(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid id"})
		return
	}
	wd, err := s.record.CancelWithdraw(r.Context(), uid, id)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, withdrawDTO(wd))
}

func (s *AppService) ListLedger(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeBizError(w, biz.ErrUnauthorized)
		return
	}
	from, to := ledgerRange(r)
	items, err := s.record.ListLedger(r.Context(), uid, from, to)
	if err != nil {
		writeBizError(w, err)
		return
	}
	out := make([]any, len(items))
	for i, e := range items {
		out[i] = ledgerDTO(e)
	}
	writeJSONArray(w, http.StatusOK, out)
}

func (s *AppService) AdminRecharge(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Address string `json:"address"`
		Amount  string `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid json"})
		return
	}
	amount, err := decimal.NewFromString(body.Amount)
	if err != nil {
		writeBizError(w, biz.ErrInvalidAmount)
		return
	}
	user, err := s.users.Deposit(r.Context(), body.Address, amount)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userDTO(user))
}

func (s *AppService) AdminSettleRun(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OrderIDs []uint64 `json:"order_ids"`
		Force    bool     `json:"force"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	res, err := s.record.SettleStatic(r.Context(), body.OrderIDs, body.Force)
	if err != nil {
		writeBizError(w, err)
		return
	}
	status := "ok"
	if res.Skipped {
		status = "already_settled"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           status,
		"skipped":          res.Skipped,
		"forced":           res.Forced,
		"settle_date":      res.SettleDate,
		"settled_count":    res.SettledCount,
		"exited_count":     res.ExitedCount,
		"generation_count": res.GenerationCount,
		"community_count":  res.CommunityCount,
		"peer_count":       res.PeerCount,
		"warning":          settleForceWarning(res),
	})
}

func (s *AppService) AdminListWithdraws(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	var userID uint64
	if v := r.URL.Query().Get("user_id"); v != "" {
		id, _ := strconv.ParseUint(v, 10, 64)
		userID = id
	}
	items, err := s.record.ListWithdrawsAdmin(r.Context(), status, userID)
	if err != nil {
		writeBizError(w, err)
		return
	}
	out := make([]any, len(items))
	for i, w := range items {
		out[i] = withdrawDTO(w)
	}
	writeJSONArray(w, http.StatusOK, out)
}

func (s *AppService) AdminApproveWithdraw(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint64(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid id"})
		return
	}
	wd, err := s.record.ApproveWithdraw(r.Context(), id)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, withdrawDTO(wd))
}

func (s *AppService) AdminRejectWithdraw(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint64(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid id"})
		return
	}
	var body struct {
		Remark string `json:"remark"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	wd, err := s.record.RejectWithdraw(r.Context(), id, body.Remark)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, withdrawDTO(wd))
}

func (s *AppService) AdminListConfigs(w http.ResponseWriter, r *http.Request) {
	items, err := s.record.ListConfigs(r.Context())
	if err != nil {
		writeBizError(w, err)
		return
	}
	out := make([]any, len(items))
	for i, c := range items {
		out[i] = map[string]any{"id": c.ID, "key": c.Key, "name": c.Name, "value": c.Value}
	}
	writeJSONArray(w, http.StatusOK, out)
}

func (s *AppService) AdminUpdateConfig(w http.ResponseWriter, r *http.Request) {
	id, err := pathUint64(r, "id")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid id"})
		return
	}
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid json"})
		return
	}
	cfg, err := s.record.UpdateConfig(r.Context(), id, body.Value)
	if err != nil {
		writeBizError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": cfg.ID, "key": cfg.Key, "name": cfg.Name, "value": cfg.Value})
}

// pathUint64 extracts a numeric path segment. Kratos hand-written routes do not
// populate gorilla/mux Vars, so we parse from the URL path (…/withdraws/{id}/…,
// …/business_configs/{id}).
func pathUint64(r *http.Request, key string) (uint64, error) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, p := range parts {
		if i+1 >= len(parts) {
			continue
		}
		next := parts[i+1]
		switch {
		case key == "id" && (p == "withdraws" || p == "business_configs" || p == "locations"):
			return strconv.ParseUint(next, 10, 64)
		}
	}
	return 0, fmt.Errorf("path param %s not found", key)
}

func ledgerRange(r *http.Request) (from, to time.Time) {
	now := time.Now()
	loc := now.Location()
	to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	from = to.AddDate(0, 0, -30)
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
			to = t.AddDate(0, 0, 1)
		}
	}
	return from, to
}

func userDTO(u *biz.User) map[string]any {
	if u == nil {
		return nil
	}
	return map[string]any{
		"id":                   u.ID,
		"address":              u.Address,
		"inviter_id":           u.InviterID,
		"account_balance":      u.AccountBalance.String(),
		"withdrawable_balance": u.WithdrawableBalance.String(),
		"community_level":      u.CommunityLevel,
		"community_volume":     u.CommunityVolume.String(),
		"reward_locked":        u.RewardLocked,
		"created_at":           u.CreatedAt.Format(time.RFC3339),
	}
}

func locationDTO(l *biz.Location) map[string]any {
	return map[string]any{
		"id":             l.ID,
		"user_id":        l.UserID,
		"amount":         l.Amount.String(),
		"multiplier":     l.Multiplier.String(),
		"exit_target":    l.ExitTarget.String(),
		"accumulated":    l.Accumulated.String(),
		"status":         l.Status,
		"rate_percent":   l.RatePercent.String(),
		"rate_direction": l.RateDirection,
		"created_at":     l.CreatedAt.Format(time.RFC3339),
	}
}

func withdrawDTO(w *biz.Withdraw) map[string]any {
	return map[string]any{
		"id":              w.ID,
		"user_id":         w.UserID,
		"amount":          w.Amount.String(),
		"fee_amount":      w.FeeAmount.String(),
		"credited_amount": w.CreditedAmount.String(),
		"order_ids":       w.OrderIDs,
		"status":          w.Status,
		"remark":          w.Remark,
		"tx_hash":         w.TxHash,
		"payout_error":    w.PayoutError,
		"reviewed_at":     formatTimePtr(w.ReviewedAt),
		"created_at":      w.CreatedAt.Format(time.RFC3339),
	}
}

func ledgerDTO(e *biz.LedgerEntry) map[string]any {
	return map[string]any{
		"id":           e.ID,
		"user_id":      e.UserID,
		"order_id":     e.OrderID,
		"entry_type":   e.EntryType,
		"amount":       e.Amount.String(),
		"balance_kind": e.BalanceKind,
		"remark":       e.Remark,
		"created_at":   e.CreatedAt.Format(time.RFC3339),
	}
}

func formatTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}
