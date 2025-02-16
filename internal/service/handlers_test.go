package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"merch_store/internal/app"
	"merch_store/internal/config"
	"merch_store/internal/models"
	"merch_store/internal/pkg/auth"
	"merch_store/internal/pkg/logger"
	"merch_store/internal/storage/mocks"
)

func testRequest(t *testing.T, ts *httptest.Server, method, path string, requestBody []byte) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, bytes.NewBuffer(requestBody))
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp, string(body)
}

func testRequestWithAuth(t *testing.T, ts *httptest.Server, method, path string, requestBody []byte, token string) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, bytes.NewBuffer(requestBody))
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(body)
}

func TestAuthHandler_Gomock(t *testing.T) {

	l, err := logger.CreateLogger(config.LogLevel)
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mocks.NewMockStorage(ctrl)

	appInstance := app.NewApp(mockDB, l)

	service := NewService(appInstance, config.ServerRunAddress, l)
	testServer := httptest.NewServer(service.NewRouter())
	defer testServer.Close()

	type expectedData struct {
		expectedContentType string
		expectedStatusCode  int
		expectedBody        string
	}

	testCases := []struct {
		name        string
		requestBody []byte
		setupMock   func()
		expected    expectedData
	}{
		{
			name:        "Invalid JSON",
			requestBody: []byte("some body"),
			setupMock:   func() {},
			expected: expectedData{
				expectedContentType: "application/json",
				expectedStatusCode:  http.StatusBadRequest,
				expectedBody:        "{\"errors\":\"invalid character 's' looking for beginning of value\"}\n",
			},
		},
		{
			name:        "Missing username",
			requestBody: []byte(`{"username": "", "password": "pass"}`),
			setupMock:   func() {},
			expected: expectedData{
				expectedContentType: "application/json",
				expectedStatusCode:  http.StatusBadRequest,
				expectedBody:        "{\"errors\":\"missing username or password\"}\n",
			},
		},
		{
			name:        "Missing password",
			requestBody: []byte(`{"username": "user", "password": ""}`),
			setupMock:   func() {},
			expected: expectedData{
				expectedContentType: "application/json",
				expectedStatusCode:  http.StatusBadRequest,
				expectedBody:        "{\"errors\":\"missing username or password\"}\n",
			},
		},
		{
			name:        "Incorrect password",
			requestBody: []byte(`{"username": "incorrect_password_user", "password": "wrongpass"}`),
			setupMock: func() {
				mockDB.EXPECT().CheckUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					DoAndReturn(func(ctx context.Context, user *models.User) (*models.User, error) {
						return &models.User{ID: 1, Username: user.Username}, bcrypt.ErrMismatchedHashAndPassword
					})
			},
			expected: expectedData{
				expectedContentType: "application/json",
				expectedStatusCode:  http.StatusUnauthorized,
				expectedBody:        "{\"errors\":\"incorrect password\"}\n",
			},
		},
		{
			name:        "User already exists (unique violation)",
			requestBody: []byte(`{"username": "new_existing_user", "password": "pass"}`),
			setupMock: func() {
				mockDB.EXPECT().CheckUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					DoAndReturn(func(ctx context.Context, user *models.User) (*models.User, error) {
						return &models.User{ID: 0, Username: user.Username}, nil
					})
				mockDB.EXPECT().CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					DoAndReturn(func(ctx context.Context, user *models.User) (*models.User, error) {
						pgErr := &pgconn.PgError{Code: pgerrcode.UniqueViolation}
						return nil, pgErr
					})
			},
			expected: expectedData{
				expectedContentType: "application/json",
				expectedStatusCode:  http.StatusUnauthorized,
				expectedBody:        "{\"errors\":\"user with provided name already exists\"}\n",
			},
		},
		{
			name:        "Successful authorization - new user",
			requestBody: []byte(`{"username": "new_user_for_auth", "password": "pass"}`),
			setupMock: func() {
				mockDB.EXPECT().CheckUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					DoAndReturn(func(ctx context.Context, user *models.User) (*models.User, error) {
						return &models.User{ID: 0, Username: user.Username}, nil
					})

				mockDB.EXPECT().CreateUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					DoAndReturn(func(ctx context.Context, user *models.User) (*models.User, error) {
						return &models.User{ID: 123, Username: user.Username, Coins: 1000}, nil
					})
			},
			expected: expectedData{
				expectedContentType: "application/json",
				expectedStatusCode:  http.StatusOK,
				expectedBody:        "",
			},
		},
		{
			name:        "Successful authorization - existing user",
			requestBody: []byte(`{"username": "existing_user", "password": "pass"}`),
			setupMock: func() {
				mockDB.EXPECT().CheckUser(gomock.Any(), gomock.AssignableToTypeOf(&models.User{})).
					DoAndReturn(func(ctx context.Context, user *models.User) (*models.User, error) {
						return &models.User{ID: 456, Username: user.Username}, nil
					})
			},
			expected: expectedData{
				expectedContentType: "application/json",
				expectedStatusCode:  http.StatusOK,
				expectedBody:        "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			tc.setupMock()
			resp, body := testRequest(t, testServer, http.MethodPost, "/api/auth", tc.requestBody)
			assert.Equal(t, tc.expected.expectedStatusCode, resp.StatusCode)
			assert.Equal(t, tc.expected.expectedContentType, resp.Header.Get("Content-Type"))

			if tc.expected.expectedStatusCode == http.StatusOK {

				var authResp models.AuthResponse
				err := json.Unmarshal([]byte(body), &authResp)
				require.NoError(t, err)
				assert.NotEmpty(t, authResp.Token, "token should not be empty")
			} else {
				assert.Equal(t, tc.expected.expectedBody, body)
			}
		})
	}
}

func TestBuyItemHandler_Gomock(t *testing.T) {
	l, err := logger.CreateLogger(config.LogLevel)
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mocks.NewMockStorage(ctrl)

	appInstance := app.NewApp(mockDB, l)

	service := NewService(appInstance, config.ServerRunAddress, l)
	testServer := httptest.NewServer(service.NewRouter())
	defer testServer.Close()

	token, err := auth.GenerateToken(1)
	require.NoError(t, err)

	type expectedData struct {
		expectedStatusCode  int
		expectedContentType string
		expectedBody        string
	}

	testCases := []struct {
		name      string
		method    string
		path      string
		token     string
		setupMock func()
		expected  expectedData
	}{
		{
			name:      "Unauthorized - no token",
			method:    http.MethodGet,
			path:      "/api/buy/item1",
			token:     "",
			setupMock: func() {},
			expected: expectedData{
				expectedStatusCode:  http.StatusUnauthorized,
				expectedContentType: "application/json",
				expectedBody:        "{\"errors\":\"missing auth header\"}\n",
			},
		},
		{
			name:   "Invalid item name (sql.ErrNoRows)",
			method: http.MethodGet,
			path:   "/api/buy/item1",
			token:  token,
			setupMock: func() {
				mockDB.EXPECT().BuyItem(gomock.Any(), int32(1), "item1").
					Return(sql.ErrNoRows)
			},
			expected: expectedData{
				expectedStatusCode:  http.StatusBadRequest,
				expectedContentType: "application/json",
				expectedBody:        "{\"errors\":\"invalid item name provided\"}\n",
			},
		},
		{
			name:   "Generic error in buying item",
			method: http.MethodGet,
			path:   "/api/buy/item1",
			token:  token,
			setupMock: func() {
				mockDB.EXPECT().BuyItem(gomock.Any(), int32(1), "item1").
					Return(errors.New("buy error"))
			},
			expected: expectedData{
				expectedStatusCode:  http.StatusInternalServerError,
				expectedContentType: "application/json",
				expectedBody:        "{\"errors\":\"buy error\"}\n",
			},
		},
		{
			name:   "Successful purchase",
			method: http.MethodGet,
			path:   "/api/buy/item1",
			token:  token,
			setupMock: func() {
				mockDB.EXPECT().BuyItem(gomock.Any(), int32(1), "item1").
					Return(nil)
			},
			expected: expectedData{
				expectedStatusCode:  http.StatusOK,
				expectedContentType: "",
				expectedBody:        "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()
			resp, body := testRequestWithAuth(t, testServer, tc.method, tc.path, nil, tc.token)
			assert.Equal(t, tc.expected.expectedStatusCode, resp.StatusCode)
			if tc.expected.expectedContentType != "" {
				assert.Equal(t, tc.expected.expectedContentType, resp.Header.Get("Content-Type"))
			}
			assert.Equal(t, tc.expected.expectedBody, body)
		})
	}
}

func TestSendCoinHandler_Gomock(t *testing.T) {
	l, err := logger.CreateLogger(config.LogLevel)
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mocks.NewMockStorage(ctrl)

	appInstance := app.NewApp(mockDB, l)

	service := NewService(appInstance, config.ServerRunAddress, l)
	testServer := httptest.NewServer(service.NewRouter())
	defer testServer.Close()

	token, err := auth.GenerateToken(1)
	require.NoError(t, err)

	type expectedData struct {
		expectedStatusCode  int
		expectedContentType string
		expectedBody        string
	}

	testCases := []struct {
		name        string
		method      string
		path        string
		token       string
		requestBody []byte
		setupMock   func()
		expected    expectedData
	}{
		{
			name:        "Unauthorized - no token",
			method:      http.MethodPost,
			path:        "/api/sendCoin",
			token:       "",
			requestBody: []byte(`{"to_user": "recipient", "amount": 100}`),
			setupMock:   func() {},
			expected: expectedData{
				expectedStatusCode:  http.StatusUnauthorized,
				expectedContentType: "application/json",
				expectedBody:        "{\"errors\":\"missing auth header\"}\n",
			},
		},
		{
			name:        "Invalid JSON",
			method:      http.MethodPost,
			path:        "/api/sendCoin",
			token:       token,
			requestBody: []byte("some body"),
			setupMock:   func() {},
			expected: expectedData{
				expectedStatusCode:  http.StatusBadRequest,
				expectedContentType: "application/json",
				expectedBody:        "{\"errors\":\"invalid character 's' looking for beginning of value\"}\n",
			},
		},
		{
			name:        "Missing username or amount",
			method:      http.MethodPost,
			path:        "/api/sendCoin",
			token:       token,
			requestBody: []byte(`{"toUser": "", "amount": 0}`),
			setupMock:   func() {},
			expected: expectedData{
				expectedStatusCode:  http.StatusBadRequest,
				expectedContentType: "application/json",
				expectedBody:        "{\"errors\":\"missing username or amount\"}\n",
			},
		},
		{
			name:        "Generic error in sending coin",
			method:      http.MethodPost,
			path:        "/api/sendCoin",
			token:       token,
			requestBody: []byte(`{"toUser": "recipient", "amount": 100}`),
			setupMock: func() {
				mockDB.EXPECT().TransferCoins(gomock.Any(), int32(1), gomock.AssignableToTypeOf(models.SendCoinRequest{})).
					DoAndReturn(func(ctx context.Context, userID int32, req models.SendCoinRequest) error {
						return errors.New("send coin error")
					})
			},
			expected: expectedData{
				expectedStatusCode:  http.StatusInternalServerError,
				expectedContentType: "application/json",
				expectedBody:        "{\"errors\":\"send coin error\"}\n",
			},
		},
		{
			name:        "Successful coin transfer",
			method:      http.MethodPost,
			path:        "/api/sendCoin",
			token:       token,
			requestBody: []byte(`{"toUser": "recipient", "amount": 100}`),
			setupMock: func() {
				mockDB.EXPECT().TransferCoins(gomock.Any(), int32(1), gomock.AssignableToTypeOf(models.SendCoinRequest{})).
					Return(nil)
			},
			expected: expectedData{
				expectedStatusCode:  http.StatusOK,
				expectedContentType: "",
				expectedBody:        "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()
			resp, body := testRequestWithAuth(t, testServer, tc.method, tc.path, tc.requestBody, tc.token)
			assert.Equal(t, tc.expected.expectedStatusCode, resp.StatusCode)
			if tc.expected.expectedContentType != "" {
				assert.Equal(t, tc.expected.expectedContentType, resp.Header.Get("Content-Type"))
			}
			assert.Equal(t, tc.expected.expectedBody, body)
		})
	}
}

func TestInfoHandler_Gomock(t *testing.T) {
	l, err := logger.CreateLogger(config.LogLevel)
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDB := mocks.NewMockStorage(ctrl)

	appInstance := app.NewApp(mockDB, l)

	service := NewService(appInstance, config.ServerRunAddress, l)
	testServer := httptest.NewServer(service.NewRouter())
	defer testServer.Close()

	token, err := auth.GenerateToken(1)
	require.NoError(t, err)

	type expectedData struct {
		expectedStatusCode  int
		expectedContentType string
		expectedBody        string
	}

	testCases := []struct {
		name      string
		method    string
		path      string
		token     string
		setupMock func()
		expected  expectedData
	}{
		{
			name:      "Unauthorized - no token",
			method:    http.MethodGet,
			path:      "/api/info",
			token:     "",
			setupMock: func() {},
			expected: expectedData{
				expectedStatusCode:  http.StatusUnauthorized,
				expectedContentType: "application/json",
				expectedBody:        "{\"errors\":\"missing auth header\"}\n",
			},
		},
		{
			name:   "Info error",
			method: http.MethodGet,
			path:   "/api/info",
			token:  token,
			setupMock: func() {
				mockDB.EXPECT().GetInfo(gomock.Any(), int32(1)).
					Return(nil, errors.New("info error"))
			},
			expected: expectedData{
				expectedStatusCode:  http.StatusInternalServerError,
				expectedContentType: "application/json",
				expectedBody:        "{\"errors\":\"info error\"}\n",
			},
		},
		{
			name:   "Successful info retrieval",
			method: http.MethodGet,
			path:   "/api/info",
			token:  token,
			setupMock: func() {
				infoResp := &models.InfoResponse{
					Coins: 500,
					Inventory: []models.InventoryItem{
						{Type: "tshirt", Quantity: 2},
					},
					CoinHistory: &models.CoinHistory{
						Sent:     []models.TransactionDetail{{ToUser: "user2", Amount: 100}},
						Received: []models.TransactionDetail{{FromUser: "user3", Amount: 50}},
					},
				}
				mockDB.EXPECT().GetInfo(gomock.Any(), int32(1)).
					Return(infoResp, nil)
			},
			expected: expectedData{
				expectedStatusCode:  http.StatusOK,
				expectedContentType: "application/json",
				expectedBody:        "\"coins\":500",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock()
			resp, body := testRequestWithAuth(t, testServer, tc.method, tc.path, nil, tc.token)
			assert.Equal(t, tc.expected.expectedStatusCode, resp.StatusCode)
			if tc.expected.expectedContentType != "" {
				assert.Equal(t, tc.expected.expectedContentType, resp.Header.Get("Content-Type"))
			}
			if tc.name == "Successful info retrieval" {
				assert.Contains(t, body, tc.expected.expectedBody)
			} else {
				assert.Equal(t, tc.expected.expectedBody, body)
			}
		})
	}
}
