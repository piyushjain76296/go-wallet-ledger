package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/wallet-ledger/internal/auth"
	"github.com/wallet-ledger/internal/config"
	"github.com/wallet-ledger/internal/database"
	"github.com/wallet-ledger/internal/idempotency"
	"github.com/wallet-ledger/internal/kafka"
	"github.com/wallet-ledger/internal/ledger"
	customMiddleware "github.com/wallet-ledger/internal/middleware"
	"github.com/wallet-ledger/internal/outbox"
	"github.com/wallet-ledger/internal/payments"
	"github.com/wallet-ledger/internal/reconciliation"
	"github.com/wallet-ledger/internal/redis"
	"github.com/wallet-ledger/internal/transfers"
	"github.com/wallet-ledger/internal/wallets"
	"github.com/go-chi/cors"
)

type Server struct {
	router *chi.Mux
	config *config.Config
	db     *database.DB
	redis  *redis.Client
}

func NewServer(cfg *config.Config, db *database.DB, rdb *redis.Client) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(customMiddleware.MetricsMiddleware)
	r.Use(middleware.Logger) // In production, we'd use a custom slog middleware
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, 
	}))

	if rdb != nil {
		r.Use(customMiddleware.RateLimiter(rdb, 100)) // 100 req/min global limit
	}

	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		// TODO: check DB, Redis, etc.
		if err := db.Pool.Ping(r.Context()); err != nil {
			http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("READY"))
	})

	// Init Auth
	authRepo := auth.NewRepository(db.Pool)
	authService := auth.NewService(authRepo, cfg.JWTSecret)
	authHandler := auth.NewHandler(authService)

	// Init Wallets
	walletRepo := wallets.NewRepository(db.Pool)
	walletService := wallets.NewService(walletRepo)
	walletHandler := wallets.NewHandler(walletService)

	// Init Idempotency & Ledger
	idemRepo := idempotency.NewRepository(db.Pool)
	ledgerRepo := ledger.NewRepository()
	
	// Init Kafka & Outbox
	kafkaProducer, err := kafka.NewProducer(cfg.KafkaBroker)
	if err != nil {
		slog.Error("Failed to initialize Kafka producer", "error", err)
		// Usually we'd fail here, but we can allow it to start without Kafka for now
	}
	outboxRepo := outbox.NewRepository(db.Pool)
	if kafkaProducer != nil {
		outboxWorker := outbox.NewWorker(outboxRepo, kafkaProducer)
		go outboxWorker.Start(context.Background())
	}

	// Init Transfers
	transferService := transfers.NewService(db.Pool, ledgerRepo, walletRepo, outboxRepo)
	transferHandler := transfers.NewHandler(transferService, idemRepo, walletRepo)

	// Init Payments
	paymentProvider := payments.NewProviderSimulator(cfg.JWTSecret) // using JWT secret for webhook sig for simplicity
	paymentService := payments.NewService(db.Pool, ledgerRepo, paymentProvider, outboxRepo)
	paymentHandler := payments.NewHandler(paymentService, paymentProvider)

	// Init Reconciliation
	reconRepo := reconciliation.NewRepository(db.Pool)
	reconWorker := reconciliation.NewWorker(reconRepo)
	go reconWorker.Start(context.Background())
	reconHandler := reconciliation.NewHandler(db.Pool)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			authHandler.RegisterRoutes(r)
		})

		// Unauthenticated webhooks (they have their own signature verification)
		r.Post("/webhooks/payment-provider", paymentHandler.HandleWebhook)

		r.Group(func(r chi.Router) {
			r.Use(customMiddleware.AuthMiddleware(cfg.JWTSecret))
			
			r.Route("/wallets", func(r chi.Router) {
				walletHandler.RegisterRoutes(r)
			})

			r.Route("/transfers", func(r chi.Router) {
				transferHandler.RegisterRoutes(r)
			})

			r.Route("/payments", func(r chi.Router) {
				paymentHandler.RegisterRoutes(r)
			})

			r.Route("/reconciliation", func(r chi.Router) {
				reconHandler.RegisterRoutes(r)
			})
		})
	})

	return &Server{
		router: r,
		config: cfg,
		db:     db,
		redis:  rdb,
	}
}

func (s *Server) Start() {
	srv := &http.Server{
		Addr:    ":" + s.config.ServerPort,
		Handler: s.router,
	}

	go func() {
		slog.Info("Starting HTTP server", "port", s.config.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server exiting")
}
