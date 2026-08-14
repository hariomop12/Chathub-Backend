package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/jwt"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/httpapi"
)

type contextKey string

const userKey contextKey = "userId"

func ClerkAuth(clerkSecretKey string) func(http.Handler) http.Handler {
	clerk.SetKey(clerkSecretKey)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				claims, err := jwt.Verify(r.Context(), &jwt.VerifyParams{
					Token: token,
				})
				if err == nil && claims != nil {
					ctx := context.WithValue(r.Context(), userKey, claims.Subject)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func UserIDFromCtx(ctx context.Context) string {
	id, _ := ctx.Value(userKey).(string)
	return id
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserIDFromCtx(r.Context()) == "" {
			httpapi.WriteErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}
