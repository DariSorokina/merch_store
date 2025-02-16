// Package storage provides primitives for connecting to and interacting with data storage systems.
// It defines the Storage interface along with a PostgreSQL implementation that manages user authentication,
// item purchases, coin transfers, and retrieval of user-related information from the database.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"merch_store/internal/models"
	"merch_store/internal/pkg/logger"
	"merch_store/internal/pkg/security"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	createUserQuery        = `INSERT INTO content.users (username, password_hash, coins) VALUES ($1, $2, $3) RETURNING id;`
	checkUserQuery         = `SELECT id, password_hash FROM content.users WHERE username = $1;`
	buyItemQuery           = `INSERT INTO content.merch_purchases (user_id, merch_id, quantity) VALUES ($1, $2, $3);`
	getItemPriceQuery      = `SELECT id, price FROM content.merch WHERE merch_name = $1;`
	getUserInfoQuery       = `SELECT username, coins FROM content.users WHERE id = $1;`
	updateUserCoinsQuery   = `UPDATE content.users SET coins = coins + $1, updated_at = NOW() WHERE id = $2;`
	getUserIDQuery         = `SELECT id FROM content.users WHERE username = $1;`
	transferCoinsQuery     = `INSERT INTO content.coin_transfers (from_user_id, to_user_id, amount) VALUES ($1, $2, $3);`
	getMerchPurchasesQuery = `SELECT m.merch_name, SUM(mp.quantity) AS total_quantity FROM content.merch_purchases mp JOIN content.merch m ON mp.merch_id = m.id WHERE mp.user_id = $1 GROUP BY m.merch_name;`
	getSendCoinsQuery      = `SELECT u.username AS recipient_username, ct.amount FROM content.coin_transfers ct JOIN content.users u ON ct.to_user_id = u.id WHERE ct.from_user_id = $1 ORDER BY ct.created_at DESC;`
	getReceivedCoinsQuery  = `SELECT u.username AS sender_username, ct.amount FROM content.coin_transfers ct JOIN content.users u ON ct.from_user_id = u.id WHERE ct.to_user_id = $1 ORDER BY ct.created_at DESC;`
)

// Storage defines the methods required for data storage operations.
type Storage interface {
	// Close closes the database connection.
	Close()

	// Authentication methods.
	CheckUser(ctx context.Context, user *models.User) (*models.User, error)
	CreateUser(ctx context.Context, user *models.User) (*models.User, error)

	// Item-related method.
	GetItemPrice(ctx context.Context, tx *sql.Tx, itemName string) (*models.Item, error)

	// User information methods.
	GetUserInfo(ctx context.Context, tx *sql.Tx, userID int32) (*models.User, error)
	GetUserID(ctx context.Context, tx *sql.Tx, username string) (*models.User, error)
	UpdateUserCoins(ctx context.Context, tx *sql.Tx, userID int32, coins int) error

	// Transactional operations.
	BuyItem(ctx context.Context, userID int32, itemName string) error
	TransferCoins(ctx context.Context, userID int32, req models.SendCoinRequest) error

	// Methods to retrieve purchase and transaction details.
	GetMerchPurchasesInfo(ctx context.Context, tx *sql.Tx, userID int32) ([]models.InventoryItem, error)
	GetCoinsTransactionInfo(ctx context.Context, tx *sql.Tx, userID int32, username string, query string) ([]models.TransactionDetail, error)
	GetInfo(ctx context.Context, userID int32) (*models.InfoResponse, error)
}

// PostgreSQL implements the Storage interface using a PostgreSQL database.
type PostgreSQL struct {
	db  *sql.DB        // Connection to the database.
	log *logger.Logger // Logger for recording events and errors.
}

// NewPostgreSQL creates a new PostgreSQL instance with the provided connection string and logger.
// It opens the connection and pings the database to ensure connectivity.
func NewPostgreSQL(cofigDBString string, l *logger.Logger) (*PostgreSQL, error) {
	db, err := sql.Open("pgx", cofigDBString)
	if err != nil {
		l.Sugar().Errorf("Failed to open a database: %s", err)
		return &PostgreSQL{db: db, log: l}, err
	}

	const defaultTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		l.Sugar().Errorf("Database ping failed: %s", err)
		return &PostgreSQL{db: db, log: l}, err
	}

	return &PostgreSQL{db: db, log: l}, nil
}

// Close closes the database connection if it is open.
func (postgresql *PostgreSQL) Close() {
	if postgresql.db != nil {
		postgresql.db.Close()
	}
}

// CheckUser verifies the user's credentials by retrieving the user's ID and encrypted password,
// then checking the provided password against the stored hash.
func (postgresql *PostgreSQL) CheckUser(ctx context.Context, user *models.User) (*models.User, error) {
	var encryptedPassword string

	err := postgresql.db.QueryRowContext(ctx, checkUserQuery, user.Username).Scan(&user.ID, &encryptedPassword)
	if errors.Is(err, sql.ErrNoRows) {
		return user, nil
	}
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute a query checkUserQuery: %s", err)
		return user, err
	}

	err = security.CheckPassword(encryptedPassword, user.Password)
	if err != nil {
		postgresql.log.Sugar().Errorf(err.Error())
		return user, err
	}

	return user, nil
}

// CreateUser registers a new user by hashing the password and inserting the user into the database.
func (postgresql *PostgreSQL) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	encryptedPassword := security.HashPassword(user.Password)

	err := postgresql.db.QueryRowContext(ctx, createUserQuery, user.Username, encryptedPassword, user.Coins).Scan(&user.ID)
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute a query createUserQuery: %s", err)
		return user, err
	}
	return user, err
}

// GetItemPrice retrieves the ID and price of an item given its name, using a transaction.
func (postgresql *PostgreSQL) GetItemPrice(ctx context.Context, tx *sql.Tx, itemName string) (*models.Item, error) {
	item := &models.Item{
		Name: itemName,
	}

	err := tx.QueryRowContext(ctx, getItemPriceQuery, itemName).Scan(&item.ID, &item.Price)
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute a query getItemPriceQuery: %s", err)
		return item, err
	}

	return item, nil
}

// GetUserInfo retrieves the username and coin balance for a given user ID using a transaction.
func (postgresql *PostgreSQL) GetUserInfo(ctx context.Context, tx *sql.Tx, userID int32) (*models.User, error) {
	user := &models.User{
		ID: userID,
	}

	err := tx.QueryRowContext(ctx, getUserInfoQuery, user.ID).Scan(&user.Username, &user.Coins)
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute a query getUserInfoQuery: %s", err)
		return user, err
	}

	return user, nil
}

// UpdateUserCoins updates the user's coin balance by adding the specified number of coins.
func (postgresql *PostgreSQL) UpdateUserCoins(ctx context.Context, tx *sql.Tx, userID int32, coins int) error {
	result, err := tx.ExecContext(ctx, updateUserCoinsQuery, coins, userID)
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute a query updateUserCoinsQuery: %s", err)
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute RowsAffected in updateUserCoinsQuery: %s", err)
		postgresql.log.Sugar().Infof("Affected rows: %d", rows)
		return err
	}

	return nil
}

// GetUserID retrieves a user's ID given their username using a transaction.
func (postgresql *PostgreSQL) GetUserID(ctx context.Context, tx *sql.Tx, username string) (*models.User, error) {
	user := &models.User{
		Username: username,
	}

	err := tx.QueryRowContext(ctx, getUserIDQuery, user.Username).Scan(&user.ID)
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute a query getUserIDQuery: %s", err)
		return user, err
	}

	return user, nil
}

// BuyItem processes the purchase of an item by a user.
// It uses a transaction to update the user's coin balance and record the purchase.
func (postgresql *PostgreSQL) BuyItem(ctx context.Context, userID int32, itemName string) error {
	tx, err := postgresql.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	item, err := postgresql.GetItemPrice(ctx, tx, itemName)
	if err != nil {
		return err
	}

	err = postgresql.UpdateUserCoins(ctx, tx, userID, -item.Price)
	if err != nil {
		return err
	}

	quantity := 1

	result, err := tx.ExecContext(ctx, buyItemQuery, userID, item.ID, quantity)
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute a query buyItemQuery: %s", err)
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute RowsAffected in buyItemQuery: %s", err)
		postgresql.log.Sugar().Infof("Affected rows: %d", rows)
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

// TransferCoins processes the transfer of coins from one user to another.
// It updates both users' coin balances and records the transfer in the database within a transaction.
func (postgresql *PostgreSQL) TransferCoins(ctx context.Context, userID int32, req models.SendCoinRequest) error {
	tx, err := postgresql.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = postgresql.UpdateUserCoins(ctx, tx, userID, -req.Amount)
	if err != nil {
		return err
	}

	toUser, err := postgresql.GetUserID(ctx, tx, req.ToUser)
	if err != nil {
		return err
	}

	err = postgresql.UpdateUserCoins(ctx, tx, toUser.ID, req.Amount)
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, transferCoinsQuery, userID, toUser.ID, req.Amount)
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute a query transferCoinsQuery: %s", err)
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute RowsAffected in transferCoinsQuery: %s", err)
		postgresql.log.Sugar().Infof("Affected rows: %d", rows)
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

// GetMerchPurchasesInfo retrieves a list of merchandise purchase records for a user.
// It returns a slice of InventoryItem representing the purchased items and their quantities.
func (postgresql *PostgreSQL) GetMerchPurchasesInfo(ctx context.Context, tx *sql.Tx, userID int32) ([]models.InventoryItem, error) {
	rows, err := tx.QueryContext(ctx, getMerchPurchasesQuery, userID)
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute a query getMerchPurchasesQuery: %s", err)
		return nil, err
	}
	defer rows.Close()

	const initialInventoryCapacity = 10
	inventory := make([]models.InventoryItem, 0, initialInventoryCapacity)

	for rows.Next() {
		inventoryItem := models.InventoryItem{}
		if err := rows.Scan(&inventoryItem.Type, &inventoryItem.Quantity); err != nil {
			postgresql.log.Sugar().Errorf("Failed to scan order information in GetMerchPurchasesInfo method: %s", err)
			return nil, err
		}

		inventory = append(inventory, inventoryItem)
	}

	if err := rows.Err(); err != nil {
		postgresql.log.Sugar().Errorf("The last error encountered by Rows.Scan in GetMerchPurchasesInfo method: %s", err)
		return inventory, err
	}

	return inventory, err
}

// GetCoinsTransactionInfo retrieves coin transaction details for a user.
// The 'query' parameter determines whether to fetch sent or received transactions.
// It returns a slice of TransactionDetail containing the transaction data.
func (postgresql *PostgreSQL) GetCoinsTransactionInfo(ctx context.Context, tx *sql.Tx, userID int32, username string, query string) ([]models.TransactionDetail, error) {
	rows, err := tx.QueryContext(ctx, query, userID)
	if err != nil {
		postgresql.log.Sugar().Errorf("Failed to execute a query getCoinsTransactionQuery: %s", err)
		return nil, err
	}
	defer rows.Close()

	const transactionDetailCapacity = 10
	transactionDetailInfo := make([]models.TransactionDetail, 0, transactionDetailCapacity)
	for rows.Next() {
		transactionDetail := models.TransactionDetail{}
		if query == getSendCoinsQuery {
			transactionDetail.FromUser = username
			if err := rows.Scan(&transactionDetail.ToUser, &transactionDetail.Amount); err != nil {
				postgresql.log.Sugar().Errorf("Failed to scan order information in GetCoinsTransactionInfo method: %s", err)
				return nil, err
			}
		} else {
			transactionDetail.ToUser = username
			if err := rows.Scan(&transactionDetail.FromUser, &transactionDetail.Amount); err != nil {
				postgresql.log.Sugar().Errorf("Failed to scan order information in GetCoinsTransactionInfo method: %s", err)
				return nil, err
			}
		}
		transactionDetailInfo = append(transactionDetailInfo, transactionDetail)
	}

	if err := rows.Err(); err != nil {
		postgresql.log.Sugar().Errorf("The last error encountered by Rows.Scan in GetCoinsTransactionInfo method: %s", err)
		return transactionDetailInfo, err
	}

	return transactionDetailInfo, err
}

// GetInfo aggregates complete information about a user, including coin balance, inventory, and transaction history.
// It uses a transaction to combine data from multiple queries and returns an InfoResponse.
func (postgresql *PostgreSQL) GetInfo(ctx context.Context, userID int32) (*models.InfoResponse, error) {
	infoResponse := &models.InfoResponse{}

	tx, err := postgresql.db.BeginTx(ctx, nil)
	if err != nil {
		return infoResponse, err
	}
	defer tx.Rollback()

	user, err := postgresql.GetUserInfo(ctx, tx, userID)
	if err != nil {
		return infoResponse, err
	}

	inventory, err := postgresql.GetMerchPurchasesInfo(ctx, tx, userID)
	if err != nil {
		return infoResponse, err
	}

	transactionDetailSent, err := postgresql.GetCoinsTransactionInfo(ctx, tx, userID, user.Username, getSendCoinsQuery)
	if err != nil {
		return infoResponse, err
	}

	transactionDetailReceived, err := postgresql.GetCoinsTransactionInfo(ctx, tx, userID, user.Username, getReceivedCoinsQuery)
	if err != nil {
		return infoResponse, err
	}

	coinHistory := &models.CoinHistory{Received: transactionDetailReceived, Sent: transactionDetailSent}
	infoResponse.Coins = user.Coins
	infoResponse.Inventory = inventory
	infoResponse.CoinHistory = coinHistory

	if err = tx.Commit(); err != nil {
		return infoResponse, err
	}

	return infoResponse, nil
}
