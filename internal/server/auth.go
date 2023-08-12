package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/samber/lo"
)

var idTokenCookieKey = "ID_TOKEN"
var refreshTokenCookieKey = "REFRESH_TOKEN"

var (
	IdTokenCtxKey      = &contextKey{"IdToken"}
	RefreshTokenCtxKey = &contextKey{"RefreshToken"}
	ErrorCtxKey        = &contextKey{"Error"}
)

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   idTokenCookieKey,
		Value:  "",
		MaxAge: -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:   refreshTokenCookieKey,
		Value:  "",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")
	if email == "" || password == "" {
		http.Redirect(w, r, "/login?error=bad request", http.StatusSeeOther)
		return
	}

	resp, err := s.authClient.SignInWithEmailAndPassword(r.Context(), email, password)
	if err != nil {
		s.logger.Error("failed to login", "error", err)
		http.Redirect(w, r, "/login?error=internal error", http.StatusSeeOther)
		return
	}
	if resp.Error != nil {
		http.Redirect(w, r, fmt.Sprintf("/login?error=%v", resp.Error.Error()), http.StatusSeeOther)
		return
	}

	// TODO: check claims to see if user is allowed to access this product

	// 5 days
	cookieExpires := time.Now().Add(5 * 24 * time.Hour)

	http.SetCookie(w, &http.Cookie{
		Name:     idTokenCookieKey,
		Value:    resp.IdToken,
		Expires:  cookieExpires,
		HttpOnly: true,
		Secure:   true,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieKey,
		Value:    resp.RefreshToken,
		Expires:  cookieExpires,
		HttpOnly: true,
		Secure:   true,
	})
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (s *server) firebaseJwtVerifier(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idTokenCookie, ok := lo.Find(r.Cookies(), func(c *http.Cookie) bool { return c.Name == idTokenCookieKey })
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		refreshTokenCookie, ok := lo.Find(r.Cookies(), func(c *http.Cookie) bool { return c.Name == refreshTokenCookieKey })
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if len(idTokenCookie.Value) == 0 || len(refreshTokenCookie.Value) == 0 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		auth, err := s.app.Auth(ctx)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.logger.Error("failed to get firebase auth", "error", err)
			return
		}

		token, err := auth.VerifyIDToken(ctx, idTokenCookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login?"+errorQuery(err.Error()), http.StatusSeeOther)
			return
		}
		// TODO: check claims to see if user is allowed to access this product
		ctx = NewContext(ctx, token, refreshTokenCookie.Value, err)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type contextKey struct {
	name string
}

func NewContext(ctx context.Context, t *auth.Token, refreshToken string, err error) context.Context {
	ctx = context.WithValue(ctx, IdTokenCtxKey, t)
	ctx = context.WithValue(ctx, RefreshTokenCtxKey, refreshToken)
	ctx = context.WithValue(ctx, ErrorCtxKey, err)
	return ctx
}

func TokenFromContext(ctx context.Context) (*auth.Token, string, error) {
	idToken, _ := ctx.Value(IdTokenCtxKey).(*auth.Token)
	refreshToken, _ := ctx.Value(RefreshTokenCtxKey).(string)
	var err error
	err, _ = ctx.Value(ErrorCtxKey).(error)
	return idToken, refreshToken, err
}
