package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/grzegab/wok/internal/config"
	"golang.org/x/crypto/bcrypt"
)

const SessionCookieName = "wok_session"

// loginFailureDelay is applied to every failed login attempt (missing
// fields, unknown username, or wrong password) so response timing can't be
// used to enumerate the valid username or tell failure reasons apart.
var loginFailureDelay = time.Second

type AuthSessionStore interface {
	Create(ctx context.Context, token string) error
	Revoke(ctx context.Context, token string) error
}

type LoginLimiter interface {
	Allow(ip string) bool
	Reset(ip string)
}

type AuthHandler struct {
	sessionRepo AuthSessionStore
	limiter     LoginLimiter
	conf        *config.Config
	tmpl        *template.Template
}

func NewAuthHandler(sessionRepo AuthSessionStore, limiter LoginLimiter, conf *config.Config, tmpl *template.Template) *AuthHandler {
	return &AuthHandler{sessionRepo: sessionRepo, limiter: limiter, conf: conf, tmpl: tmpl}
}

type loginPageData struct {
	Username string
	Error    string
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.LoginPost(w, r)
		return
	}

	h.renderLogin(w, loginPageData{})
}

func (h *AuthHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	ip := middleware.GetClientIP(r.Context())
	if !h.limiter.Allow(ip) {
		log.Printf("too many login attempts from %s", ip)
		w.WriteHeader(http.StatusTooManyRequests)
		h.renderLogin(w, loginPageData{Username: username, Error: "Too many attempts, try again later"})
		return
	}

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
	h.limiter.Reset(ip)

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.conf.SessionTTL.Seconds()),
	})

	http.Redirect(w, r, "/l1st", http.StatusSeeOther)
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

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		if err := h.sessionRepo.Revoke(r.Context(), cookie.Value); err != nil {
			log.Printf("could not revoke session: %v", err)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	http.Redirect(w, r, "/", http.StatusNotFound)
}

func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
