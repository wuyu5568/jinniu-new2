package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jinniu/app/app/app/internal/biz"
)

type ctxKey struct{}

// WithUserID stores authenticated user id in context.
func WithUserID(ctx context.Context, userID uint64) context.Context {
	return context.WithValue(ctx, ctxKey{}, userID)
}

// UserIDFromContext reads user id set by JWT middleware.
func UserIDFromContext(ctx context.Context) (uint64, bool) {
	v, ok := ctx.Value(ctxKey{}).(uint64)
	return v, ok
}

func parseBearerUID(tokenStr string, key []byte) (uint64, bool) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return 0, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, false
	}
	if role, _ := claims["role"].(string); role == "admin" {
		return 0, false
	}
	raw, ok := claims["uid"].(float64)
	if !ok {
		return 0, false
	}
	return uint64(raw), true
}

func parseBearerAdmin(tokenStr string, key []byte) bool {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		return key, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false
	}
	role, _ := claims["role"].(string)
	return role == "admin"
}

// RequireAdminJWT wraps std http handlers with Bearer admin JWT validation.
func RequireAdminJWT(jwtKey string, next http.HandlerFunc) http.HandlerFunc {
	key := []byte(jwtKey)
	return func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		if authz == "" || !strings.HasPrefix(authz, "Bearer ") {
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(authz, "Bearer ")
		if !parseBearerAdmin(tokenStr, key) {
			http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// RequireJWT wraps std http handlers with Bearer JWT validation.
func RequireJWT(jwtKey string, next http.HandlerFunc) http.HandlerFunc {
	key := []byte(jwtKey)
	return func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		if authz == "" || !strings.HasPrefix(authz, "Bearer ") {
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(authz, "Bearer ")
		uid, ok := parseBearerUID(tokenStr, key)
		if !ok {
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r.WithContext(WithUserID(r.Context(), uid)))
	}
}

// JWT validates Bearer tokens for Kratos middleware (optional).
func JWT(jwtKey string) middleware.Middleware {
	key := []byte(jwtKey)
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tr, ok := transport.FromServerContext(ctx); ok {
				if ht, ok := tr.(*khttp.Transport); ok {
					authz := ht.Request().Header.Get("Authorization")
					if authz == "" || !strings.HasPrefix(authz, "Bearer ") {
						return nil, biz.ErrUnauthorized
					}
					tokenStr := strings.TrimPrefix(authz, "Bearer ")
					uid, ok := parseBearerUID(tokenStr, key)
					if !ok {
						return nil, biz.ErrUnauthorized
					}
					ctx = WithUserID(ctx, uid)
				}
			}
			return handler(ctx, req)
		}
	}
}
