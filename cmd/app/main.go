package main

import (
	"context"
	_ "database/sql"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"git.codenrock.com/avito-testirovanie-na-backend-1270/cnrprod1725728996-team-79175/zadanie-6105/api"
	"git.codenrock.com/avito-testirovanie-na-backend-1270/cnrprod1725728996-team-79175/zadanie-6105/internal/db"
	"git.codenrock.com/avito-testirovanie-na-backend-1270/cnrprod1725728996-team-79175/zadanie-6105/internal/handlers"
)

type Config struct {
	Port string
	DSN  string
}

var _ api.ServerInterface = (*handlers.MyServer)(nil)

func setupLogging() *slog.Logger {
	log := slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
	)

	return log
}

func main() {
	log := setupLogging()
	slog.SetDefault(log)

	log.Info("Starting server", slog.String("Port", "8080"))
	log.Info("Starting db", slog.String("DSN", os.Getenv("POSTGRES_CONN")))

	cfg := Config{
		Port: os.Getenv("SERVER_ADDRESS"), // 8080
		DSN:  os.Getenv("POSTGRES_CONN"),
	}

	dbConn, err := db.NewDB(context.Background(), cfg.DSN)
	if err != nil {
		log.Error("Failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer dbConn.Close()

	log.Info("Starting server", slog.String("Port", cfg.Port))
	log.Debug("Debugging info enabled")

	r := chi.NewRouter()

	myServer := handlers.NewServer(dbConn)

	r.Route("/api", func(apiRouter chi.Router) {
		apiHandler := api.HandlerFromMux(myServer, apiRouter)
		apiRouter.Mount("/", apiHandler)
	})

	log.Info("Starting server", slog.String("port", cfg.Port))
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("Failed to start server", slog.String("error", err.Error()))
		os.Exit(1)
	}

}
