package integrations

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"merch_store/internal/app"
	"merch_store/internal/models"
	"merch_store/internal/pkg/logger"
	"merch_store/internal/service"
	"merch_store/internal/storage"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/suite"
)

var testDatabaseURI, testServerPort string

func init() {
	if err := godotenv.Load("../integration/.env"); err != nil {
		log.Println("No .env file found, using default values")
	}

	testDatabaseURI = os.Getenv("TEST_DATABASE_URI")
	testServerPort = os.Getenv("TEST_SERVER_PORT")
}

type IntegrationTestSuite struct {
	suite.Suite
	server *httptest.Server
	client *http.Client
	db     *storage.PostgreSQL
}

func (s *IntegrationTestSuite) SetupSuite() {

	var l *logger.Logger
	var err error
	if l, err = logger.CreateLogger("info"); err != nil {
		log.Fatal("Failed to create logger:", err)
	}

	log.Printf("%v", testDatabaseURI)

	s.db, err = storage.NewPostgreSQL(testDatabaseURI, l)
	s.Require().NoError(err, "Error connecting to test database")

	appInstance := app.NewApp(s.db, l)
	serviceInstance := service.NewService(appInstance, "localhost:"+testServerPort, l)

	s.server = httptest.NewServer(serviceInstance.NewRouter())
	s.client = s.server.Client()
}

func (s *IntegrationTestSuite) TearDownSuite() {
	s.server.Close()
	s.db.Close()
}

func (s *IntegrationTestSuite) TestBuyMerch() {
	authReq := models.AuthRequest{
		Username: "employee1",
		Password: "password",
	}
	reqBody, err := json.Marshal(authReq)
	s.Require().NoError(err, "Error marshaling authentication request")

	resp, err := s.client.Post(s.server.URL+"/api/auth", "application/json", bytes.NewBuffer(reqBody))
	s.Require().NoError(err, "Error sending authentication request")
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for authentication")

	var authResp models.AuthResponse
	err = json.NewDecoder(resp.Body).Decode(&authResp)
	resp.Body.Close()
	s.Require().NoError(err, "Error decoding authentication response")
	s.Require().NotEmpty(authResp.Token, "Token should not be empty")

	itemName := "t-shirt"
	req, err := http.NewRequest("GET", s.server.URL+"/api/buy/"+itemName, nil)
	s.Require().NoError(err, "Error creating merch purchase request")
	req.Header.Set("Authorization", "Bearer "+authResp.Token)

	resp, err = s.client.Do(req)
	s.Require().NoError(err, "Error executing merch purchase request")
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for merch purchase")
	resp.Body.Close()

	req, err = http.NewRequest("GET", s.server.URL+"/api/info", nil)
	s.Require().NoError(err, "Error creating request to retrieve user info")
	req.Header.Set("Authorization", "Bearer "+authResp.Token)
	resp, err = s.client.Do(req)
	s.Require().NoError(err, "Error executing request to retrieve user info")
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for retrieving user info")

	var infoResp models.InfoResponse
	err = json.NewDecoder(resp.Body).Decode(&infoResp)
	resp.Body.Close()
	s.Require().NoError(err, "Error decoding user info")

	s.T().Logf("User coins after purchase: %d", infoResp.Coins)
	s.T().Logf("User inventory: %+v", infoResp.Inventory)
}

func (s *IntegrationTestSuite) TestSendCoin() {
	employee1 := models.AuthRequest{
		Username: "employee2",
		Password: "password",
	}
	employee2 := models.AuthRequest{
		Username: "employee3",
		Password: "password",
	}

	getToken := func(authReq models.AuthRequest) string {
		reqBody, err := json.Marshal(authReq)
		s.Require().NoError(err, "Error marshaling authentication request")

		resp, err := s.client.Post(s.server.URL+"/api/auth", "application/json", bytes.NewBuffer(reqBody))
		s.Require().NoError(err, "Error sending authentication request")
		s.Require().Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for authentication")

		var authResp models.AuthResponse
		err = json.NewDecoder(resp.Body).Decode(&authResp)
		resp.Body.Close()
		s.Require().NoError(err, "Error decoding authentication response")
		s.Require().NotEmpty(authResp.Token, "Token should not be empty")
		return authResp.Token
	}

	tokenSender := getToken(employee1)
	tokenReceiver := getToken(employee2)

	sendReq := models.SendCoinRequest{
		ToUser: employee2.Username,
		Amount: 100,
	}
	reqBody, err := json.Marshal(sendReq)
	s.Require().NoError(err, "Error marshalling coin transfer request")

	req, err := http.NewRequest("POST", s.server.URL+"/api/sendCoin", bytes.NewBuffer(reqBody))
	s.Require().NoError(err, "Error creating coin transfer request")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenSender)

	resp, err := s.client.Do(req)
	s.Require().NoError(err, "Error executing coin transfer request")
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for coin transfer")
	resp.Body.Close()

	reqSenderInfo, err := http.NewRequest("GET", s.server.URL+"/api/info", nil)
	s.Require().NoError(err, "Error creating request for sender info")
	reqSenderInfo.Header.Set("Authorization", "Bearer "+tokenSender)

	resp, err = s.client.Do(reqSenderInfo)
	s.Require().NoError(err, "Error executing request for sender info")
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for retrieving sender info")

	var senderInfo models.InfoResponse
	err = json.NewDecoder(resp.Body).Decode(&senderInfo)
	resp.Body.Close()
	s.Require().NoError(err, "Error decoding sender info")

	reqReceiverInfo, err := http.NewRequest("GET", s.server.URL+"/api/info", nil)
	s.Require().NoError(err, "Error creating request for receiver info")
	reqReceiverInfo.Header.Set("Authorization", "Bearer "+tokenReceiver)

	resp, err = s.client.Do(reqReceiverInfo)
	s.Require().NoError(err, "Error executing request for receiver info")
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for retrieving receiver info")

	var receiverInfo models.InfoResponse
	err = json.NewDecoder(resp.Body).Decode(&receiverInfo)
	resp.Body.Close()
	s.Require().NoError(err, "Error decoding receiver info")

	s.T().Logf("Sender coins: %d", senderInfo.Coins)
	s.T().Logf("Receiver coins: %d", receiverInfo.Coins)
	s.Require().Equal(900, senderInfo.Coins, "Sender should have 900 coins")
	s.Require().Equal(1100, receiverInfo.Coins, "Receiver should have 1100 coins")
}

func (s *IntegrationTestSuite) TestInfo() {
	// Authenticate user employee4
	employee4Auth := models.AuthRequest{
		Username: "employee4",
		Password: "password",
	}
	reqBody, err := json.Marshal(employee4Auth)
	s.Require().NoError(err, "Error marshaling authentication request for employee4")

	resp, err := s.client.Post(s.server.URL+"/api/auth", "application/json", bytes.NewBuffer(reqBody))
	s.Require().NoError(err, "Error sending authentication request for employee4")
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for employee4 authentication")

	var authResp models.AuthResponse
	err = json.NewDecoder(resp.Body).Decode(&authResp)
	resp.Body.Close()
	s.Require().NoError(err, "Error decoding employee4 authentication response")
	s.Require().NotEmpty(authResp.Token, "Employee4 token should not be empty")

	// Purchase item 'book'
	req, err := http.NewRequest("GET", s.server.URL+"/api/buy/book", nil)
	s.Require().NoError(err, "Error creating purchase request for book")
	req.Header.Set("Authorization", "Bearer "+authResp.Token)

	resp, err = s.client.Do(req)
	s.Require().NoError(err, "Error executing purchase request for book")
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for purchasing book")
	resp.Body.Close()

	// Purchase item 'umbrella'
	req, err = http.NewRequest("GET", s.server.URL+"/api/buy/umbrella", nil)
	s.Require().NoError(err, "Error creating purchase request for umbrella")
	req.Header.Set("Authorization", "Bearer "+authResp.Token)

	resp, err = s.client.Do(req)
	s.Require().NoError(err, "Error executing purchase request for umbrella")
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for purchasing umbrella")
	resp.Body.Close()

	// Transfer 113 coins to employee1
	coinTransferReq := models.SendCoinRequest{
		ToUser: "employee1",
		Amount: 113,
	}
	reqBody, err = json.Marshal(coinTransferReq)
	s.Require().NoError(err, "Error marshaling coin transfer request for employee4")

	req, err = http.NewRequest("POST", s.server.URL+"/api/sendCoin", bytes.NewBuffer(reqBody))
	s.Require().NoError(err, "Error creating coin transfer request for employee4")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+authResp.Token)

	resp, err = s.client.Do(req)
	s.Require().NoError(err, "Error executing coin transfer request for employee4")
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for coin transfer")
	resp.Body.Close()

	// Retrieve transaction history
	req, err = http.NewRequest("GET", s.server.URL+"/api/info", nil)
	s.Require().NoError(err, "Error creating request for employee4 info")
	req.Header.Set("Authorization", "Bearer "+authResp.Token)

	resp, err = s.client.Do(req)
	s.Require().NoError(err, "Error executing request for employee4 info")
	s.Require().Equal(http.StatusOK, resp.StatusCode, "Expected status 200 for retrieving employee4 info")

	var infoResp models.InfoResponse
	err = json.NewDecoder(resp.Body).Decode(&infoResp)
	resp.Body.Close()
	s.Require().NoError(err, "Error decoding employee4 info response")

	s.T().Logf("Employee4 coins after transactions: %d", infoResp.Coins)
	s.T().Logf("Employee4 inventory: %+v", infoResp.Inventory)
	s.T().Logf("Employee4 coin history: %+v", infoResp.CoinHistory)
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
