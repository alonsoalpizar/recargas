package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/alonsoalpizar/recargas/internal/auth"
	"github.com/alonsoalpizar/recargas/internal/config"
	"github.com/alonsoalpizar/recargas/internal/db"
	"github.com/alonsoalpizar/recargas/internal/httpx"
	"github.com/alonsoalpizar/recargas/internal/users"
	"github.com/alonsoalpizar/recargas/internal/wallet"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DBURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	usersSvc := users.NewService(pool, cfg.JWTSecret)
	walletSvc := wallet.NewService(pool)

	usersH := users.NewHandler(usersSvc)
	walletH := wallet.NewHandler(walletSvc)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Post("/register", usersH.Register)
		r.Post("/login", usersH.Login)

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireUser(cfg.JWTSecret))
			r.Get("/me", usersH.Me)
			r.Get("/wallet/balance", walletH.Balance)
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAdmin(cfg.AdminToken))
			r.Post("/admin/wallet/{userId}/adjust", walletH.Adjust)
		})
	})

	r.Handle("/*", http.FileServer(http.Dir("web")))

	srv := &http.Server{
		Addr:              "127.0.0.1:" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("base_url=%s", cfg.BaseURL)
	go func() {
		log.Printf("recargas listening on http://%s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")
	shutdownCtx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer scancel()
	_ = srv.Shutdown(shutdownCtx)
}
