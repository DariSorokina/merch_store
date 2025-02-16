package main

import (
	"context"
	"errors"
	"log"
	"merch_store/internal/app"
	"merch_store/internal/config"
	"merch_store/internal/pkg/logger"
	"merch_store/internal/service"
	"merch_store/internal/storage"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var l *logger.Logger
	var err error
	if l, err = logger.CreateLogger(config.LogLevel); err != nil {
		log.Fatal("Failed to create logger:", err)
	}

	storage, err := storage.NewPostgreSQL(config.DatabaseURI, l)
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	app := app.NewApp(storage, l)
	service := service.NewService(app, config.ServerRunAddress, l)

	const readHeaderTimeout = 5 * time.Second
	server := &http.Server{Addr: config.ServerRunAddress, Handler: service.NewRouter(), ReadHeaderTimeout: readHeaderTimeout}

	serverCtx, serverStopCtx := context.WithCancel(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sig

		const shutdownTimeout = 30 * time.Second
		shutdownCtx, cancel := context.WithTimeout(serverCtx, shutdownTimeout)
		defer cancel()

		go func() {
			<-shutdownCtx.Done()
			if errors.Is(shutdownCtx.Err(), context.DeadlineExceeded) {
				log.Fatal("graceful shutdown timed out.. forcing exit.")
			}
		}()

		err := server.Shutdown(shutdownCtx)
		if err != nil {
			log.Fatal(err)
		}
		serverStopCtx()
	}()

	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		defer storage.Close()
		log.Fatal(err)
	}

	<-serverCtx.Done()
}
