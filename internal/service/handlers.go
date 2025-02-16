// Package service contains HTTP handler implementations for the merch store API endpoints.
// It orchestrates request parsing, calls the underlying business logic in the app package,
// handles errors (including database-specific errors), and writes appropriate HTTP responses.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"merch_store/internal/app"
	"merch_store/internal/models"
	"merch_store/internal/pkg/auth"
	"merch_store/internal/pkg/logger"

	"github.com/go-chi/chi/v5"
	pgconn "github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	pgx_pgconn "github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const requestTimeout = 10 * time.Second

// handlers aggregates dependencies needed by HTTP handlers,
// including the application business logic and logger.
type handlers struct {
	app *app.App
	log *logger.Logger
}

// newHandlers initializes a new handlers instance with the provided app and logger dependencies.
func newHandlers(app *app.App, l *logger.Logger) *handlers {
	return &handlers{app: app, log: l}
}

// authHandler handles user authentication requests.
// It reads the request body, unmarshals it into an AuthRequest,
// invokes the authentication process, and returns a JSON response with a token.
func (handlers *handlers) authHandler(res http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), requestTimeout)
	defer cancel()

	var authRequest models.AuthRequest
	var authResponse models.AuthResponse

	requestBody, err := io.ReadAll(req.Body)
	if err != nil {
		writeErrorResponse(res, err.Error(), http.StatusBadRequest)
		return
	}

	if err = json.Unmarshal(requestBody, &authRequest); err != nil {
		writeErrorResponse(res, err.Error(), http.StatusBadRequest)
		return
	}

	var pgError *pgconn.PgError
	authResponse.Token, err = handlers.app.ProcessAuth(ctx, authRequest)
	if err != nil {
		if ok := errors.As(err, &pgError); ok && pgError.Code == pgerrcode.UniqueViolation {
			writeErrorResponse(res, "user with provided name already exists", http.StatusUnauthorized)
			return
		}

		if errors.Is(err, app.ErrMissingUsernameOrPassword) {
			writeErrorResponse(res, "missing username or password", http.StatusBadRequest)
			return
		}

		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			writeErrorResponse(res, "incorrect password", http.StatusUnauthorized)
			return
		}
		writeErrorResponse(res, err.Error(), http.StatusInternalServerError)
		return
	}

	result, err := json.Marshal(authResponse)
	if err != nil {
		writeErrorResponse(res, err.Error(), http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write(result)
}

// buyItemHandler processes requests to purchase an item.
// It extracts the authenticated user's ID from the context, retrieves the item name from the URL,
// and calls the business logic to process the purchase.
func (handlers *handlers) buyItemHandler(res http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), requestTimeout)
	defer cancel()

	userID, ok := req.Context().Value(auth.ContextUserID).(int32)
	if !ok || userID == 0 {
		writeErrorResponse(res, "unauthorized", http.StatusUnauthorized)
		return
	}

	var pgError *pgx_pgconn.PgError
	itemName := chi.URLParam(req, "item")
	err := handlers.app.ProcessBuy(ctx, userID, itemName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeErrorResponse(res, "invalid item name provided", http.StatusBadRequest)
			return
		}

		if ok := errors.As(err, &pgError); ok && pgError.Code == pgerrcode.CheckViolation {
			writeErrorResponse(res, "insufficient funds to purchase the item", http.StatusBadRequest)
			return
		}

		writeErrorResponse(res, err.Error(), http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
}

// sendCoinHandler processes coin transfer requests between users.
// It validates the request body, checks for the required fields,
// and calls the application logic to perform the coin transfer.
func (handlers *handlers) sendCoinHandler(res http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), requestTimeout)
	defer cancel()

	userID, ok := req.Context().Value(auth.ContextUserID).(int32)
	if !ok || userID == 0 {
		writeErrorResponse(res, "unauthorized", http.StatusUnauthorized)
		return
	}

	var sendCoinRequest models.SendCoinRequest

	requestBody, err := io.ReadAll(req.Body)
	if err != nil {
		writeErrorResponse(res, err.Error(), http.StatusBadRequest)
		return
	}

	if err = json.Unmarshal(requestBody, &sendCoinRequest); err != nil {
		writeErrorResponse(res, err.Error(), http.StatusBadRequest)
		return
	}

	var pgError *pgx_pgconn.PgError
	err = handlers.app.ProcessSendCoin(ctx, userID, sendCoinRequest)
	if err != nil {
		if errors.Is(err, app.ErrMissingUsernameOrAmount) {
			writeErrorResponse(res, "missing username or amount", http.StatusBadRequest)
			return
		}

		if ok := errors.As(err, &pgError); ok && pgError.Code == pgerrcode.CheckViolation {
			switch err.(*pgx_pgconn.PgError).ConstraintName {
			case "users_coins_check":
				writeErrorResponse(res, "insufficient funds to perform the transfer", http.StatusBadRequest)
				return
			case "chk_different_users":
				writeErrorResponse(res, "self-transfer of money is not allowed; please choose a different user.", http.StatusBadRequest)
				return
			default:
				writeErrorResponse(res, "transfer cannot be performed", http.StatusInternalServerError)
			}
		}

		writeErrorResponse(res, err.Error(), http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
}

// infoHandler retrieves user account information.
// It extracts the user ID from the context, calls the business logic to obtain user info,
// and returns the information in JSON format.
func (handlers *handlers) infoHandler(res http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), requestTimeout)
	defer cancel()

	userID, ok := req.Context().Value(auth.ContextUserID).(int32)
	if !ok || userID == 0 {
		writeErrorResponse(res, "unauthorized", http.StatusUnauthorized)
		return
	}

	info, err := handlers.app.ProcessInfo(ctx, userID)
	if err != nil {
		writeErrorResponse(res, err.Error(), http.StatusInternalServerError)
		return
	}

	result, err := json.Marshal(info)
	if err != nil {
		writeErrorResponse(res, err.Error(), http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write(result)
}

func writeErrorResponse(res http.ResponseWriter, errorInfo string, statusCode int) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(statusCode)
	json.NewEncoder(res).Encode(models.ErrorResponse{Errors: errorInfo})
}
