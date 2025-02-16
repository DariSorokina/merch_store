package service

import (
	"merch_store/internal/app"
	"merch_store/internal/pkg/auth"
	"merch_store/internal/pkg/logger"

	"github.com/go-chi/chi/v5"
)

// Service encapsulates the HTTP server configuration, including the application's business logic,
// HTTP handlers, the server's run address, and a logger for event and error logging.
type Service struct {
	handlers   *handlers
	app        *app.App
	runAddress string
	log        *logger.Logger
}

// NewService creates and initializes a new Service instance.
// It sets up the handlers using the provided application and logger,
// and configures the server's run address.
func NewService(app *app.App, runAddress string, l *logger.Logger) *Service {
	handlers := newHandlers(app, l)
	return &Service{handlers: handlers, app: app, runAddress: runAddress, log: l}
}

// NewRouter sets up and returns a new chi.Router instance with the necessary middleware and routes.
// It applies logging middleware globally, and JWT authentication middleware for protected routes.
func (service *Service) NewRouter() chi.Router {
	router := chi.NewRouter()
	router.Use(service.log.WithLogging())
	router.Post("/api/auth", service.handlers.authHandler)
	router.Route("/", func(r chi.Router) {
		r.Use(auth.CheckJWTMiddleware())
		r.Get("/api/info", service.handlers.infoHandler)
		r.Post("/api/sendCoin", service.handlers.sendCoinHandler)
		r.Get("/api/buy/{item}", service.handlers.buyItemHandler)
	})
	return router
}
