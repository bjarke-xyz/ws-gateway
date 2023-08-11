package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	firebase "firebase.google.com/go/v4"
	serverPkg "github.com/bjarke-xyz/ws-gateway/internal/server"
	"github.com/bjarke-xyz/ws-gateway/internal/service"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/api/option"
)

func ServerCmd(ctx context.Context) error {
	godotenv.Load()
	port := 9090
	_port := os.Getenv("PORT")
	if _port != "" {
		port, _ = strconv.Atoi(_port)
	}
	logger := newLogger("api")
	credentialsJson := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS_CONTENT")
	credentialsJsonBytes := []byte(credentialsJson)
	opt := option.WithCredentialsJSON(credentialsJsonBytes)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return fmt.Errorf("error initializing app: %w", err)
	}
	authClient := service.NewFirebaseAuthRestClient(os.Getenv("FIREBASE_WEB_API_KEY"), os.Getenv("FIREBASE_PROJECT_ID"))

	pool, err := newDatabasePool(ctx, 16)
	if err != nil {
		return fmt.Errorf("error creating db pool: %w", err)
	}

	server, err := serverPkg.NewServer(ctx, logger, app, authClient, pool)
	if err != nil {
		return fmt.Errorf("error creating server")
	}

	srv := server.Server(port)

	// metrics
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":9091", mux)
	}()

	go func() {
		_ = srv.ListenAndServe()
	}()
	logger.Info("started server", slog.Int("port", port))
	<-ctx.Done()
	_ = srv.Shutdown(ctx)
	return nil
}
