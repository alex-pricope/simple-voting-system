package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/alex-pricope/simple-voting-system/api/models"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/alex-pricope/simple-voting-system/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/http/httptest"
	"testing"
)

//nolint:staticcheck
func setupTestVoteController(t *testing.T) (*VotingController, *gin.Engine) {
	t.Helper()
	logging.Log = logrus.New()

	// Load localstack config
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: "http://localhost:4566", HostnameImmutable: true}, nil
			}),
		),
	)
	if err != nil {
		t.Fatalf("failed to load AWS config: %v", err)
	}

	db := dynamodb.NewFromConfig(cfg)
	voteStorage := &storage.DynamoVoteStorage{
		Client:    db,
		TableName: "Votes",
	}
	codeStorage := &storage.DynamoVotingCodesStorage{
		Client:    db,
		TableName: "VotingCodes",
	}

	t.Cleanup(func() {
		cleanupTable(t, db, "VotingCodes")
		cleanupTable(t, db, "Votes")
	})

	controller := NewVotingController(codeStorage, voteStorage)
	admin := NewAdminController(codeStorage)
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/api/verify/:code", controller.validateVotingCode)
	r.POST("/api/vote/", controller.registerVote)
	r.POST("/api/admin/codes", admin.createCode)

	return controller, r
}

//nolint:staticcheck
func TestValidateVotingCode(t *testing.T) {
	_, router := setupTestVoteController(t)

	t.Run("Happy path - verify valid code", func(t *testing.T) {
		payload := models.CreateCodeRequest{
			Count:    1,
			Category: "general_public",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/codes", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var created []*models.CodeResponse
		if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || len(created) == 0 {
			t.Fatalf("failed to unmarshal created code: %v", err)
		}
		code := created[0].Code

		validateReq := httptest.NewRequest(http.MethodGet, "/api/verify/"+code, nil)
		validateRes := httptest.NewRecorder()
		router.ServeHTTP(validateRes, validateReq)

		if validateRes.Code != http.StatusOK {
			t.Fatalf("expected 200 from verify, got %d", validateRes.Code)
		}

		var response models.CodeValidationResponse
		if err := json.Unmarshal(validateRes.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !response.Valid || response.Category != "general_public" {
			t.Fatalf("unexpected validation response: %+v", response)
		}
	})

	t.Run("Unhappy path - non-existent code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/verify/NOTEXIST", nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)

		if res.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", res.Code)
		}
	})

	t.Run("Unhappy path - used code", func(t *testing.T) {
		// Create a code
		payload := models.CreateCodeRequest{
			Count:    1,
			Category: "general_public",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/codes", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var created []*models.CodeResponse
		if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || len(created) == 0 {
			t.Fatalf("failed to unmarshal created code: %v", err)
		}
		code := created[0].Code

		// Mark as used
		client := storage.DynamoVotingCodesStorage{
			Client: dynamodb.NewFromConfig(aws.Config{Region: "us-east-1", EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: "http://localhost:4566", HostnameImmutable: true}, nil
			})}),
			TableName: "VotingCodes",
		}
		err := client.MarkUsed(context.TODO(), code)
		if err != nil {
			t.Fatalf("failed to mark code as used: %v", err)
		}

		// Validate again
		validateReq := httptest.NewRequest(http.MethodGet, "/api/verify/"+code, nil)
		validateRes := httptest.NewRecorder()
		router.ServeHTTP(validateRes, validateReq)

		if validateRes.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", validateRes.Code)
		}
		var response models.CodeValidationResponse
		if err := json.Unmarshal(validateRes.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if !response.Valid || !response.Used {
			t.Fatalf("expected used=true for code %s, got %+v", code, response)
		}
	})
}
