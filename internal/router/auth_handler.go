package router

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/grzegab/wok/internal/config"
	"golang.org/x/crypto/bcrypt"
)

// loginFailureDelay is applied to every failed login attempt (missing
// fields, unknown username, or wrong password) so response timing can't be
// used to enumerate the valid username or tell failure reasons apart.
var loginFailureDelay = time.Second

type authSessionStore interface {
	Create(ctx context.Context, token string) error
	Revoke(ctx context.Context, token string) error
}

type AuthHandler struct {
	sessionRepo authSessionStore
	conf        *config.Config
	tmpl        *template.Template
}

func NewAuthHandler(sessionRepo authSessionStore, conf *config.Config, tmpl *template.Template) *AuthHandler {
	return &AuthHandler{sessionRepo: sessionRepo, conf: conf, tmpl: tmpl}
}

type loginPageData struct {
	Username string
	Error    string
	LoggedIn bool
}

func (h *AuthHandler) login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.loginPost(w, r)
		return
	}

	h.renderLogin(w, loginPageData{})
}

func (h *AuthHandler) loginPost(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		h.renderLoginUnauthorized(w, username)
		return
	}

	if username != h.conf.LoginUser {
		h.renderLoginUnauthorized(w, username)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(h.conf.LoginPassHash), []byte(password)); err != nil {
		h.renderLoginUnauthorized(w, username)
		return
	}

	token, err := generateSessionToken()
	if err != nil {
		log.Printf("could not generate session token: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.sessionRepo.Create(r.Context(), token); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.conf.SessionTTL.Seconds()),
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) renderLoginUnauthorized(w http.ResponseWriter, username string) {
	time.Sleep(loginFailureDelay)
	w.WriteHeader(http.StatusUnauthorized)
	h.renderLogin(w, loginPageData{Username: username, Error: "Invalid credentials"})
}

func (h *AuthHandler) renderLogin(w http.ResponseWriter, data loginPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.tmpl.ExecuteTemplate(w, "login", data); err != nil {
		log.Printf("could not execute login template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *AuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if err := h.sessionRepo.Revoke(r.Context(), cookie.Value); err != nil {
			log.Printf("could not revoke session: %v", err)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/l0g1n", http.StatusSeeOther)
}

func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
