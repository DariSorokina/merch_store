// Package models defines the data structures used throughout the application.
// It includes request and response payloads for authentication, coin transfers,
// user information, inventory items, and transaction details.
package models

// AuthRequest represents the authentication request payload.
// It contains the username and password provided by the user.
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AuthResponse represents the authentication response payload.
// It contains the generated token upon successful authentication.
type AuthResponse struct {
	Token string `json:"token"`
}

// ErrorResponse represents a generic error response payload.
// It contains a string describing the encountered error.
type ErrorResponse struct {
	Errors string `json:"errors"`
}

// User represents a user in the system.
// It holds the user's identifier, credentials, and current coin balance.
type User struct {
	ID       int32
	Username string
	Password string
	Coins    int
}

// Item represents an item available in the merch store.
// It includes details such as the item's identifier, name, and price.
type Item struct {
	ID    int
	Name  string
	Price int
}

// SendCoinRequest represents the payload for transferring coins between users.
// It contains the recipient's username and the amount of coins to transfer.
type SendCoinRequest struct {
	ToUser string `json:"toUser"`
	Amount int    `json:"amount"`
}

// InventoryItem represents an entry in a user's inventory.
// It includes the type of item and the quantity owned by the user.
type InventoryItem struct {
	Type     string `json:"type"`
	Quantity int    `json:"quantity"`
}

// TransactionDetail contains detailed information about a coin transaction.
// It may include details about the sender, the recipient, and the amount transferred.
type TransactionDetail struct {
	FromUser string `json:"fromUser,omitempty"`
	ToUser   string `json:"toUser,omitempty"`
	Amount   int    `json:"amount"`
}

// CoinHistory represents the history of coin transactions for a user.
// It maintains separate lists for coins received and sent.
type CoinHistory struct {
	Received []TransactionDetail `json:"received"`
	Sent     []TransactionDetail `json:"sent"`
}

// InfoResponse represents the response payload for the /api/info endpoint.
// It contains the user's current coin balance, inventory details, and transaction history.
type InfoResponse struct {
	Coins       int             `json:"coins"`
	Inventory   []InventoryItem `json:"inventory"`
	CoinHistory *CoinHistory    `json:"coinHistory"`
}
