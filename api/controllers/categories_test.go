package controllers

import (
	"context"
	"encoding/json"
	testutils "github.com/alex-pricope/simple-voting-system/api/controllers/testing"
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:staticcheck
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
	require.NoError(t, err, "failed to load config")

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
	require.NoError(t, err, "cleanup failed to scan %s", tableName)

	for _, item := range out.Items {
		key := map[string]types.AttributeValue{
			"PK": item["PK"],
		}
		_, err := client.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
			TableName: aws.String(tableName),
			Key:       key,
		})
		require.NoError(t, err, "cleanup failed to delete item")
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
		createReq := testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", reqBody, map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusOK, createReq.Code, "failed to create category")

		// Delete it
		delReq := testutils.PerformRequest(router, http.MethodDelete, "/api/meta/categories/500", nil, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusOK, delReq.Code, "failed to delete category")

		// Get should now 404
		getRes := testutils.PerformRequest(router, http.MethodGet, "/api/meta/categories/500", nil, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusNotFound, getRes.Code, "expected 404 after delete")
	})

	t.Run("Unhappy path - delete non-existing ID", func(t *testing.T) {
		w := testutils.PerformRequest(router, http.MethodDelete, "/api/meta/categories/9999", nil, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusOK, w.Code, "expected 200 for idempotent delete")
	})

	t.Run("Unhappy path - invalid ID format", func(t *testing.T) {
		w := testutils.PerformRequest(router, http.MethodDelete, "/api/meta/categories/notanumber", nil, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusBadRequest, w.Code, "expected 400 for invalid ID")
	})
}

func TestGetVotingCategoryByID(t *testing.T) {
	_, router := setupCategoryTestController(t)

	t.Run("Happy path - get category by ID", func(t *testing.T) {
		reqBody := models.VotingCategoryCreateRequest{
			ID:          300,
			Name:        "Performance",
			Description: "Measures how well the app performs",
			Weight:      0.5,
		}
		createReq := testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", reqBody, map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, createReq.Code, "failed to create category")

		getRes := testutils.PerformRequest(router, http.MethodGet, "/api/meta/categories/300", nil, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, getRes.Code, "expected 200 OK")

		var res models.VotingCategoryResponse
		err := json.Unmarshal(getRes.Body.Bytes(), &res)
		require.NoError(t, err, "failed to unmarshal response")
		require.Equal(t, 300, res.ID)
		require.Equal(t, "Performance", res.Name)
		require.Equal(t, "Measures how well the app performs", res.Description)
		require.Equal(t, 0.5, res.Weight)
	})

	t.Run("Unhappy path - invalid ID format", func(t *testing.T) {
		w := testutils.PerformRequest(router, http.MethodGet, "/api/meta/categories/notanumber", nil, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusBadRequest, w.Code, "expected 400 for invalid ID")
	})

	t.Run("Unhappy path - non-existing ID", func(t *testing.T) {
		w := testutils.PerformRequest(router, http.MethodGet, "/api/meta/categories/999", nil, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusNotFound, w.Code, "expected 404 for invalid ID")
	})
}

func TestCreateVotingCategory(t *testing.T) {
	_, router := setupCategoryTestController(t)

	t.Run("Happy path - create category", func(t *testing.T) {
		reqBody := models.VotingCategoryCreateRequest{
			ID:          1,
			Name:        "Creativity",
			Description: "Creativity of the project",
			Weight:      0.5,
		}
		req := testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", reqBody, map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, req.Code, "expected 200 OK")
	})

	t.Run("Unhappy path - missing name", func(t *testing.T) {
		reqBody := models.VotingCategoryCreateRequest{
			ID:          2,
			Description: "Creativity of the project",
			Weight:      0.5,
		}
		req := testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", reqBody, map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusBadRequest, req.Code, "expected 400 Bad Request")
	})

	t.Run("Unhappy path - duplicate ID", func(t *testing.T) {
		// Create two unique categories first
		for i := 3; i <= 4; i++ {
			reqBody := models.VotingCategoryCreateRequest{
				ID:          i,
				Name:        "Category " + strconv.Itoa(i),
				Description: "Test category",
				Weight:      0.5,
			}
			req := testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", reqBody, map[string]string{
				"Content-Type":  "application/json",
				"x-admin-token": "secret",
			})

			require.Equal(t, http.StatusOK, req.Code, "setup POST failed for ID %d", i)
		}

		// Attempt to create a third category with a duplicate ID
		duplicate := models.VotingCategoryCreateRequest{
			ID:          3,
			Name:        "Duplicate",
			Description: "This should fail",
			Weight:      0.5,
		}
		req := testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", duplicate, map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusConflict, req.Code, "expected 409 Conflict for duplicate ID")
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
		req := testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", createReq, map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusOK, req.Code, "failed to create category")

		// Update category
		updateReq := models.VotingCategoryUpdateRequest{
			Name:        "Updated Name",
			Description: "Updated Description",
			Weight:      0.4,
		}
		req = testutils.PerformRequest(router, http.MethodPut, "/api/meta/categories/100", updateReq, map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusOK, req.Code, "failed to update category")

		// Get updated category
		getReq := httptest.NewRequest(http.MethodGet, "/api/meta/categories/100", nil)
		getReq.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, getReq)

		var res models.VotingCategoryResponse
		err := json.Unmarshal(w.Body.Bytes(), &res)
		require.NoError(t, err, "failed to parse get response")
		assert.Equal(t, "Updated Name", res.Name)
		assert.Equal(t, "Updated Description", res.Description)
	})

	t.Run("Unhappy path - invalid ID", func(t *testing.T) {
		reqBody := models.VotingCategoryUpdateRequest{
			Name:        "Should Fail",
			Description: "Invalid ID test",
		}
		req := testutils.PerformRequest(router, http.MethodPut, "/api/meta/categories/notanumber", reqBody, map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusBadRequest, req.Code, "expected 400 for invalid ID")
	})

	t.Run("Unhappy path - empty name", func(t *testing.T) {
		reqBody := models.VotingCategoryUpdateRequest{
			Name:        "",
			Description: "Missing name field",
		}
		req := testutils.PerformRequest(router, http.MethodPut, "/api/meta/categories/200", reqBody, map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusBadRequest, req.Code, "expected 400 for empty name")
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
		req := testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", reqBody, map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusOK, req.Code, "failed to create category %d", i)
	}

	// Act: List categories
	getRes := testutils.PerformRequest(router, http.MethodGet, "/api/meta/categories", nil, map[string]string{
		"x-admin-token": "secret",
	})

	require.Equal(t, http.StatusOK, getRes.Code, "expected 200 OK")

	var categories []models.VotingCategoryResponse
	err := json.Unmarshal(getRes.Body.Bytes(), &categories)
	require.NoError(t, err, "failed to unmarshal response")
	assert.Len(t, categories, 3)

	expected := map[int]string{
		101: "Category 101",
		102: "Category 102",
		103: "Category 103",
	}
	for _, cat := range categories {
		assert.Equal(t, expected[cat.ID], cat.Name, "unexpected category data")
	}
}
