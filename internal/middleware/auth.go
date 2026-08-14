package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/hariomop12/real-time-chat-app/backend-go/internal/auth"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/httpapi"
	"github.com/hariomop12/real-time-chat-app/backend-go/internal/repository"
)

type contextKey string

const userKey contextKey = "userId"

// GoogleAuth verifies a Google Sign-In ID token from the Authorization header.
// On success it upserts the Google user and injects their user id into the
// request context. On failure the request passes through without a user, so
// RequireAuth can respond 401.
func GoogleAuth(verifier auth.Verifier, userRepo *repository.UserRepo) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				claims, err := verifier.Verify(r.Context(), token)
				if err == nil && claims != nil {
					user, err := userRepo.UpsertByGoogle(claims.Sub, claims.Name, claims.Email, claims.Picture)
					if err == nil && user != nil {
						ctx := context.WithValue(r.Context(), userKey, user.ID)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
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
