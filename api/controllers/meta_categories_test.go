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
	"strconv"
	"testing"
)

func setupCategoryTestController(t *testing.T) (*CategoryMetaController, *gin.Engine) {
	t.Helper()
	logging.Log = logrus.New()

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: "http://localhost:4566", HostnameImmutable: true}, nil
			}),
		),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	client := dynamodb.NewFromConfig(cfg)
	t.Cleanup(func() {
		cleanupCategoryTable(t, client, "VotingCategories")
	})

	s := &storage.DynamoVotingCategoryStorage{
		Client:    client,
		TableName: "VotingCategories",
	}

	controller := NewCategoryMetaController(s)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/meta/categories", controller.create)
	r.PUT("/api/meta/categories/:id", controller.update)
	r.GET("/api/meta/categories/:id", controller.get)
	r.GET("/api/meta/categories", controller.getAll)
	r.DELETE("/api/meta/categories/:id", controller.delete)

	return controller, r
}

func cleanupCategoryTable(t *testing.T, client *dynamodb.Client, tableName string) {
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

func TestDeleteVotingCategory(t *testing.T) {
	_, router := setupCategoryTestController(t)

	t.Run("Happy path - delete category", func(t *testing.T) {
		// Create category
		reqBody := models.VotingCategoryCreateRequest{
			ID:          500,
			Name:        "ToDelete",
			Description: "To be removed",
		}
		body, _ := json.Marshal(reqBody)

		createReq := httptest.NewRequest(http.MethodPost, "/api/meta/categories", bytes.NewBuffer(body))
		createReq.Header.Set("Content-Type", "application/json")
		createReq.Header.Set("x-admin-token", "secret")
		createRes := httptest.NewRecorder()
		router.ServeHTTP(createRes, createReq)
		if createRes.Code != http.StatusOK {
			t.Fatalf("failed to create category: %d", createRes.Code)
		}

		// Delete it
		delReq := httptest.NewRequest(http.MethodDelete, "/api/meta/categories/500", nil)
		delReq.Header.Set("x-admin-token", "secret")
		delRes := httptest.NewRecorder()
		router.ServeHTTP(delRes, delReq)
		if delRes.Code != http.StatusOK {
			t.Fatalf("failed to delete category: %d", delRes.Code)
		}

		// Get should now 404
		getReq := httptest.NewRequest(http.MethodGet, "/api/meta/categories/500", nil)
		getReq.Header.Set("x-admin-token", "secret")
		getRes := httptest.NewRecorder()
		router.ServeHTTP(getRes, getReq)
		if getRes.Code != http.StatusNotFound {
			t.Fatalf("expected 404 after delete, got %d", getRes.Code)
		}
	})

	t.Run("Unhappy path - delete non-existing ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/meta/categories/9999", nil)
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for idempotent delete, got %d", w.Code)
		}
	})

	t.Run("Unhappy path - invalid ID format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/meta/categories/notanumber", nil)
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid ID, got %d", w.Code)
		}
	})
}

func TestGetVotingCategoryByID(t *testing.T) {
	_, router := setupCategoryTestController(t)

	t.Run("Happy path - get category by ID", func(t *testing.T) {
		reqBody := models.VotingCategoryCreateRequest{
			ID:          300,
			Name:        "Performance",
			Description: "Measures how well the app performs",
		}
		body, _ := json.Marshal(reqBody)

		createReq := httptest.NewRequest(http.MethodPost, "/api/meta/categories", bytes.NewBuffer(body))
		createReq.Header.Set("Content-Type", "application/json")
		createReq.Header.Set("x-admin-token", "secret")
		createRes := httptest.NewRecorder()
		router.ServeHTTP(createRes, createReq)

		if createRes.Code != http.StatusOK {
			t.Fatalf("failed to create category: %d", createRes.Code)
		}

		getReq := httptest.NewRequest(http.MethodGet, "/api/meta/categories/300", nil)
		getReq.Header.Set("x-admin-token", "secret")
		getRes := httptest.NewRecorder()
		router.ServeHTTP(getRes, getReq)

		if getRes.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", getRes.Code)
		}

		var res models.VotingCategoryResponse
		if err := json.Unmarshal(getRes.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if res.ID != 300 || res.Name != "Performance" || res.Description != "Measures how well the app performs" {
			t.Fatalf("unexpected category data: %+v", res)
		}
	})

	t.Run("Unhappy path - invalid ID format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/meta/categories/notanumber", nil)
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid ID, got %d", w.Code)
		}
	})

	t.Run("Unhappy path - non-existing ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/meta/categories/999", nil)
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for invalid ID, got %d", w.Code)
		}
	})
}

func TestCreateVotingCategory(t *testing.T) {
	_, router := setupCategoryTestController(t)

	t.Run("Happy path - create category", func(t *testing.T) {
		reqBody := models.VotingCategoryCreateRequest{
			ID:          1,
			Name:        "Creativity",
			Description: "Creativity of the project",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/meta/categories", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Unhappy path - missing name", func(t *testing.T) {
		reqBody := models.VotingCategoryCreateRequest{
			ID:          2,
			Description: "Creativity of the project",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/meta/categories", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Unhappy path - duplicate ID", func(t *testing.T) {
		// Create two unique categories first
		for i := 3; i <= 4; i++ {
			reqBody := models.VotingCategoryCreateRequest{
				ID:          i,
				Name:        "Category " + strconv.Itoa(i),
				Description: "Test category",
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/api/meta/categories", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-admin-token", "secret")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("setup POST failed for ID %d: got %d", i, w.Code)
			}
		}

		// Attempt to create a third category with a duplicate ID
		duplicate := models.VotingCategoryCreateRequest{
			ID:          3,
			Name:        "Duplicate",
			Description: "This should fail",
		}
		body, _ := json.Marshal(duplicate)

		req := httptest.NewRequest(http.MethodPost, "/api/meta/categories", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict for duplicate ID, got %d: %s", w.Code, w.Body.String())
		}
	})

}

func TestPutVotingCategory(t *testing.T) {
	_, router := setupCategoryTestController(t)

	t.Run("Happy path - update category", func(t *testing.T) {
		// Create category
		createReq := models.VotingCategoryCreateRequest{
			ID:          100,
			Name:        "Original Name",
			Description: "Original Description",
		}
		createBody, _ := json.Marshal(createReq)

		req := httptest.NewRequest(http.MethodPost, "/api/meta/categories", bytes.NewBuffer(createBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("failed to create category: %d", w.Code)
		}

		// Update category
		updateReq := models.VotingCategoryUpdateRequest{
			Name:        "Updated Name",
			Description: "Updated Description",
		}
		updateBody, _ := json.Marshal(updateReq)

		req = httptest.NewRequest(http.MethodPut, "/api/meta/categories/100", bytes.NewBuffer(updateBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("failed to update category: %d", w.Code)
		}

		// Get updated category
		req = httptest.NewRequest(http.MethodGet, "/api/meta/categories/100", nil)
		req.Header.Set("x-admin-token", "secret")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var res models.VotingCategoryResponse
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to parse get response: %v", err)
		}
		if res.Name != "Updated Name" || res.Description != "Updated Description" {
			t.Fatalf("category update did not persist: got %+v", res)
		}
	})

	t.Run("Unhappy path - invalid ID", func(t *testing.T) {
		reqBody := models.VotingCategoryUpdateRequest{
			Name:        "Should Fail",
			Description: "Invalid ID test",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/api/meta/categories/notanumber", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid ID, got %d", w.Code)
		}
	})

	t.Run("Unhappy path - empty name", func(t *testing.T) {
		reqBody := models.VotingCategoryUpdateRequest{
			Name:        "",
			Description: "Missing name field",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPut, "/api/meta/categories/200", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty name, got %d", w.Code)
		}
	})
}

func TestListVotingCategories(t *testing.T) {
	_, router := setupCategoryTestController(t)

	// Arrange: Create 3 categories
	for i := 101; i <= 103; i++ {
		reqBody := models.VotingCategoryCreateRequest{
			ID:          i,
			Name:        "Category " + strconv.Itoa(i),
			Description: "Description " + strconv.Itoa(i),
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/meta/categories", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("failed to create category %d: got %d", i, w.Code)
		}
	}

	// Act: List categories
	getReq := httptest.NewRequest(http.MethodGet, "/api/meta/categories", nil)
	getReq.Header.Set("x-admin-token", "secret")
	getRes := httptest.NewRecorder()
	router.ServeHTTP(getRes, getReq)

	if getRes.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", getRes.Code)
	}

	var categories []models.VotingCategoryResponse
	if err := json.Unmarshal(getRes.Body.Bytes(), &categories); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(categories) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(categories))
	}

	expected := map[int]string{
		101: "Category 101",
		102: "Category 102",
		103: "Category 103",
	}
	for _, cat := range categories {
		if expected[cat.ID] != cat.Name {
			t.Errorf("unexpected category data: %+v", cat)
		}
	}
}
