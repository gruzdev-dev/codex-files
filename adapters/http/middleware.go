package http

import (
	"net/http"
	"strings"

	"codex-files/core/domain"
	"codex-files/pkg/identity"
	"github.com/golang-jwt/jwt/v5"
)

type AuthMiddleware struct {
	secret []byte
}

func NewAuthMiddleware(secret string) *AuthMiddleware {
	return &AuthMiddleware{secret: []byte(secret)}
}

func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return m.secret, nil
		})

		if err != nil || !token.Valid {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		id := domain.Identity{
			UserID: getClaim(claims, "sub"),
			Scopes: parseScopes(claims["scope"]),
		}

		ctx := identity.WithCtx(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getClaim(claims jwt.MapClaims, key string) string {
	val, _ := claims[key].(string)
	return val
}

func parseScopes(raw interface{}) []string {
	if s, ok := raw.(string); ok {
		return strings.Split(s, " ")
	}
	if slice, ok := raw.([]interface{}); ok {
		res := make([]string, len(slice))
		for i, v := range slice {
			res[i], _ = v.(string)
		}
		return res
	}
	return nil
}
