// Package app provides the core application logic for managing URLs and user interactions// Package app provides the core business logic for the merch store application.
// It handles user authentication, item purchasing, coin transfers, and user information retrieval.
// The package integrates with the storage layer for data persistence and uses the auth package for token generation.
// Logging functionality is provided via the logger package.
package app

import (
	"context"
	"errors"
	"merch_store/internal/models"
	"merch_store/internal/pkg/auth"
	"merch_store/internal/pkg/logger"
	"merch_store/internal/storage"
)

// Predefined errors for missing required parameters in requests.
var (
	// ErrMissingUsernameOrPassword indicates that either the username or password is not provided.
	ErrMissingUsernameOrPassword = errors.New("app: missing username or password")
	// ErrMissingUsernameOrAmount indicates that either the recipient username or amount is not provided.
	ErrMissingUsernameOrAmount = errors.New("app: missing user or amount")
)

// App encapsulates the application logic and dependencies required to process requests.
// It interacts with the storage layer and uses a logger for error and activity logging.
type App struct {
	db  storage.Storage // Database storage layer for persistent data operations.
	log *logger.Logger  // Logger for logging application events and errors.
}

// NewApp creates and returns a new instance of App with the provided storage and logger dependencies.
func NewApp(db storage.Storage, log *logger.Logger) *App {
	return &App{db: db, log: log}
}

// ProcessAuth handles user authentication by verifying credentials and generating a token.
// If the user does not exist, it creates a new user with a default coin balance.
func (app *App) ProcessAuth(ctx context.Context, req models.AuthRequest) (string, error) {
	if req.Username == "" || req.Password == "" {
		return "", ErrMissingUsernameOrPassword
	}

	user := &models.User{
		Username: req.Username,
		Password: req.Password,
	}

	user, err := app.db.CheckUser(ctx, user)
	if err != nil {
		return "", err
	}

	if user.ID == 0 {
		user.Coins = 1000
		user, err = app.db.CreateUser(ctx, user)
		if err != nil {
			return "", err
		}
	}

	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		return "", err
	}

	return token, nil
}

// ProcessBuy processes the purchase of an item for a given user by delegating to the storage layer.
func (app *App) ProcessBuy(ctx context.Context, userID int32, itemName string) error {
	err := app.db.BuyItem(ctx, userID, itemName)
	if err != nil {
		return err
	}

	return nil
}

// ProcessSendCoin handles the coin transfer from one user to another.
// It validates the request and then processes the coin transfer via the storage layer.
func (app *App) ProcessSendCoin(ctx context.Context, userID int32, req models.SendCoinRequest) error {
	if req.ToUser == "" || req.Amount == 0 {
		return ErrMissingUsernameOrAmount
	}

	err := app.db.TransferCoins(ctx, userID, req)
	if err != nil {
		return err
	}

	return nil
}

// ProcessInfo retrieves detailed information about a user's account.
// It queries the storage layer for information such as coin balance and other user-specific details.
func (app *App) ProcessInfo(ctx context.Context, userID int32) (*models.InfoResponse, error) {
	infoResponse, err := app.db.GetInfo(ctx, userID)
	if err != nil {
		return nil, err
	}

	return infoResponse, nil
}
