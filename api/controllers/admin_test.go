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
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/http/httptest"
	"testing"
)

//nolint:staticcheck
func setupTestAdminController(t *testing.T) (*AdminController, *gin.Engine) {
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
	s := &storage.DynamoVotingCodesStorage{
		Client:    db,
		TableName: "VotingCodes",
	}

	// teardown
	t.Cleanup(func() {
		cleanupTable(t, db, "VotingCodes")
	})

	controller := NewAdminController(s)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/admin/codes", controller.createCode)
	r.GET("/api/admin/codes", controller.listCodes)
	r.GET("/api/admin/codes/:category", controller.getCodesByCategory)
	r.DELETE("/api/admin/codes/:code", controller.deleteCode)
	r.POST("/api/admin/codes/:code/reset", controller.resetCode)

	return controller, r
}

func TestGetCodesByCategory(t *testing.T) {
	_, router := setupTestAdminController(t)

	t.Run("Happy path - create and get by category", func(t *testing.T) {
		payload := models.CreateCodeRequest{
			Count:    2,
			Category: "other_team",
		}
		body, _ := json.Marshal(payload)

		postReq := httptest.NewRequest(http.MethodPost, "/api/admin/codes", bytes.NewBuffer(body))
		postReq.Header.Set("Content-Type", "application/json")
		postReq.Header.Set("x-admin-token", "secret")
		postRes := httptest.NewRecorder()
		router.ServeHTTP(postRes, postReq)

		if postRes.Code != http.StatusOK {
			t.Fatalf("expected 200 from POST, got %d", postRes.Code)
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/admin/codes/other_team", nil)
		getReq.Header.Set("x-admin-token", "secret")
		getRes := httptest.NewRecorder()
		router.ServeHTTP(getRes, getReq)

		if getRes.Code != http.StatusOK {
			t.Fatalf("expected 200 from GET, got %d", getRes.Code)
		}

		var result []*models.CodeResponse
		if err := json.Unmarshal(getRes.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 codes, got %d", len(result))
		}
	})

	t.Run("Unhappy path - valid but unused category", func(t *testing.T) {
		getReq := httptest.NewRequest(http.MethodGet, "/api/admin/codes/grand_jury", nil)
		getReq.Header.Set("x-admin-token", "secret")
		getRes := httptest.NewRecorder()
		router.ServeHTTP(getRes, getReq)

		if getRes.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", getRes.Code)
		}
		var result []*models.CodeResponse
		if err := json.Unmarshal(getRes.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if len(result) != 0 {
			t.Fatalf("expected 0 codes, got %d", len(result))
		}
	})

	t.Run("Unhappy path - invalid category", func(t *testing.T) {
		getReq := httptest.NewRequest(http.MethodGet, "/api/admin/codes/invalid_category", nil)
		getReq.Header.Set("x-admin-token", "secret")
		getRes := httptest.NewRecorder()
		router.ServeHTTP(getRes, getReq)

		if getRes.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", getRes.Code)
		}
	})
}

func cleanupTable(t *testing.T, client *dynamodb.Client, tableName string) {
	t.Helper()

	out, err := client.Scan(context.TODO(), &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		t.Fatalf("cleanup failed to scan %s: %v", tableName, err)
	}

	for _, item := range out.Items {
		key := map[string]types.AttributeValue{
			"PK": item["PK"],
		}
		_, err := client.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
			TableName: aws.String(tableName),
			Key:       key,
		})
		if err != nil {
			t.Fatalf("cleanup failed to delete item: %v", err)
		}
	}
}

func TestDeleteVotingCodes(t *testing.T) {
	_, router := setupTestAdminController(t)

	// Create 5 codes
	payload := models.CreateCodeRequest{
		Count:    5,
		Category: "general_public",
	}
	body, _ := json.Marshal(payload)
	postReq := httptest.NewRequest(http.MethodPost, "/api/admin/codes", bytes.NewBuffer(body))
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("x-admin-token", "secret")
	postRes := httptest.NewRecorder()
	router.ServeHTTP(postRes, postReq)

	if postRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from POST, got %d", postRes.Code)
	}

	var created []*models.CodeResponse
	if err := json.Unmarshal(postRes.Body.Bytes(), &created); err != nil || len(created) != 5 {
		t.Fatalf("Expected 5 codes created, got %d (err: %v)", len(created), err)
	}

	// Delete each code
	for _, code := range created {
		delReq := httptest.NewRequest(http.MethodDelete, "/api/admin/codes/"+code.Code, nil)
		delReq.Header.Set("x-admin-token", "secret")
		delRes := httptest.NewRecorder()
		router.ServeHTTP(delRes, delReq)

		if delRes.Code != http.StatusOK {
			t.Errorf("Failed to delete code %s: expected 200, got %d", code.Code, delRes.Code)
		}
	}

	// Verify all deleted
	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/codes", nil)
	getReq.Header.Set("x-admin-token", "secret")
	getRes := httptest.NewRecorder()
	router.ServeHTTP(getRes, getReq)

	if getRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from GET, got %d", getRes.Code)
	}

	var remaining []*models.CodeResponse
	if err := json.Unmarshal(getRes.Body.Bytes(), &remaining); err != nil {
		t.Fatalf("failed to parse GET response: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected no codes remaining, got %d", len(remaining))
	}

	// Attempt delete non-existing code
	nonExistReq := httptest.NewRequest(http.MethodDelete, "/api/admin/codes/DOESNOTEXIST", nil)
	nonExistReq.Header.Set("x-admin-token", "secret")
	nonExistRes := httptest.NewRecorder()
	router.ServeHTTP(nonExistRes, nonExistReq)

	if nonExistRes.Code != http.StatusOK {
		t.Errorf("expected 200 when deleting non-existent code (idempotent), got %d", nonExistRes.Code)
	}
}

func TestListVotingCodes(t *testing.T) {
	_, router := setupTestAdminController(t)

	categories := []string{"grand_jury", "other_team", "general_public"}
	createdCodes := make(map[string]string)

	// Arrange - create 5 codes across different categories
	for i := 0; i < 5; i++ {
		category := categories[i%len(categories)]
		payload := models.CreateCodeRequest{
			Count:    1,
			Category: category,
		}
		body, _ := json.Marshal(payload)
		postReq := httptest.NewRequest(http.MethodPost, "/api/admin/codes", bytes.NewBuffer(body))
		postReq.Header.Set("Content-Type", "application/json")
		postReq.Header.Set("x-admin-token", "secret")
		postRes := httptest.NewRecorder()
		router.ServeHTTP(postRes, postReq)

		if postRes.Code != http.StatusOK {
			t.Fatalf("POST failed: expected 200, got %d", postRes.Code)
		}

		var created []*models.CodeResponse
		if err := json.Unmarshal(postRes.Body.Bytes(), &created); err != nil || len(created) == 0 {
			t.Fatalf("Failed to parse created code response: %v", err)
		}
		createdCodes[created[0].Code] = created[0].Category
	}

	// Act - retrieve codes
	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/codes", nil)
	getReq.Header.Set("x-admin-token", "secret")
	getRes := httptest.NewRecorder()
	router.ServeHTTP(getRes, getReq)

	if getRes.Code != http.StatusOK {
		t.Fatalf("GET failed: expected 200, got %d", getRes.Code)
	}

	var codes []*models.CodeResponse
	if err := json.Unmarshal(getRes.Body.Bytes(), &codes); err != nil {
		t.Fatalf("Failed to parse GET response: %v", err)
	}

	// Assert - verify all created codes are in the response
	for code, category := range createdCodes {
		var found *models.CodeResponse
		for _, c := range codes {
			if c.Code == code {
				found = c
				break
			}
		}
		if found == nil {
			t.Errorf("Expected code %s not found in GET response", code)
		} else {
			if found.Category != category {
				t.Errorf("Expected category %s, got %s for code %s", category, found.Category, code)
			}
			if found.Used != false {
				t.Errorf("Expected Used=false for code %s, got %v", code, found.Used)
			}
			if found.CreatedAt.IsZero() {
				t.Errorf("Expected non-zero CreatedAt for code %s", code)
			}
		}
	}
}

func TestCreateVotingCodes(t *testing.T) {
	_, router := setupTestAdminController(t)

	t.Run("Create one code", func(t *testing.T) {
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

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if len(w.Body.Bytes()) == 0 {
			t.Fatalf("expected response body to contain codes")
		}
	})

	t.Run("Create multiple codes", func(t *testing.T) {
		payload := models.CreateCodeRequest{
			Count:    3,
			Category: "other_team",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/codes", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		if len(w.Body.Bytes()) == 0 {
			t.Fatalf("expected response body to contain codes")
		}
	})

	t.Run("Create with invalid category", func(t *testing.T) {
		payload := models.CreateCodeRequest{
			Count:    1,
			Category: "invalid_category",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/codes", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Create with missing or zero count", func(t *testing.T) {
		payload := models.CreateCodeRequest{
			Count:    0,
			Category: "general_public",
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/admin/codes", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

}

func TestGetCategories(t *testing.T) {
	logging.Log = logrus.New()
	controller := NewAdminController(nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/admin/categories", controller.listCategories)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/categories", nil)
	req.Header.Set("x-admin-token", "secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var result []map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(result) != len(models.ValidCategories) {
		t.Fatalf("expected %d categories, got %d", len(models.ValidCategories), len(result))
	}

	for _, category := range result {
		if category["key"] == "" || category["label"] == "" {
			t.Fatalf("expected key and label to be present, got: %+v", category)
		}
	}
}

//nolint:staticcheck
func TestResetVotingCode(t *testing.T) {
	_, router := setupTestAdminController(t)

	// Create a code
	payload := models.CreateCodeRequest{
		Count:    1,
		Category: "general_public",
	}
	body, _ := json.Marshal(payload)
	postReq := httptest.NewRequest(http.MethodPost, "/api/admin/codes", bytes.NewBuffer(body))
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("x-admin-token", "secret")
	postRes := httptest.NewRecorder()
	router.ServeHTTP(postRes, postReq)

	if postRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from POST, got %d", postRes.Code)
	}

	var created []*models.CodeResponse
	if err := json.Unmarshal(postRes.Body.Bytes(), &created); err != nil || len(created) == 0 {
		t.Fatalf("failed to parse created code: %v", err)
	}
	code := created[0].Code

	// Manually mark the code as used
	created[0].Used = true
	client := storage.DynamoVotingCodesStorage{
		Client: dynamodb.NewFromConfig(aws.Config{Region: "us-east-1", EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "http://localhost:4566", HostnameImmutable: true}, nil
		})}),
		TableName: "VotingCodes",
	}
	err := client.MarkUsed(context.TODO(), created[0].Code)
	if err != nil {
		t.Fatalf("failed to manually update voting code as used: %v", err)
	}

	// Reset the code via the API
	resetReq := httptest.NewRequest(http.MethodPost, "/api/admin/codes/"+code+"/reset", nil)
	resetReq.Header.Set("x-admin-token", "secret")
	resetRes := httptest.NewRecorder()
	router.ServeHTTP(resetRes, resetReq)

	if resetRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from reset, got %d", resetRes.Code)
	}

	// Fetch and verify the code is no longer used
	getReq := httptest.NewRequest(http.MethodGet, "/api/admin/codes", nil)
	getReq.Header.Set("x-admin-token", "secret")
	getRes := httptest.NewRecorder()
	router.ServeHTTP(getRes, getReq)

	if getRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from GET, got %d", getRes.Code)
	}

	var codes []*models.CodeResponse
	if err := json.Unmarshal(getRes.Body.Bytes(), &codes); err != nil {
		t.Fatalf("failed to parse codes: %v", err)
	}
	for _, c := range codes {
		if c.Code == code && c.Used {
			t.Fatalf("expected code %s to be marked as unused", code)
		}
	}
}
