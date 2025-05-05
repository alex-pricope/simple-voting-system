package controllers

import (
	"context"
	"encoding/json"
	"fmt"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
	"time"
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
	require.NoError(t, err, "failed to load AWS config")

	db := dynamodb.NewFromConfig(cfg)
	v := &storage.DynamoVotingCodesStorage{
		Client:    db,
		TableName: "VotingCodes",
	}

	s := &storage.DynamoTeamStorage{
		Client:    db,
		TableName: "VotingTeams",
	}

	vv := &storage.DynamoVoteStorage{
		Client:    db,
		TableName: "Votes",
	}

	// teardown
	t.Cleanup(func() {
		cleanupTable(t, db, "VotingCodes")
		cleanupTable(t, db, "VotingTeams")
		cleanupTableVotes(t, db)
	})

	controller := NewAdminController(v, s, vv)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/admin/codes", controller.createCode)
	r.GET("/api/admin/codes", controller.listCodes)
	r.GET("/api/admin/codes/:category", controller.getCodesByCategory)
	r.DELETE("/api/admin/codes/:code", controller.deleteCode)
	r.POST("api/admin/codes/:code/attach-team/:teamId", controller.attachTeam)
	r.POST("/api/admin/codes/:code/reset", controller.resetCode)
	r.POST("api/admin/votes/delete-all", controller.deleteAllVotes)

	return controller, r
}

func TestGetCodesByCategory(t *testing.T) {
	_, router := setupTestAdminController(t)

	t.Run("Happy path - create and get by category", func(t *testing.T) {
		payload := models.CreateCodeRequest{
			Count:    2,
			Category: "other_team",
		}

		postRes := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, postRes.Code)

		getRes := testutils.PerformRequest(router, http.MethodGet, "/api/admin/codes/other_team", nil, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, getRes.Code)

		var result []*models.CodeResponse
		require.NoError(t, json.Unmarshal(getRes.Body.Bytes(), &result))
		require.Len(t, result, 2)
	})

	t.Run("Unhappy path - valid but unused category", func(t *testing.T) {
		getRes := testutils.PerformRequest(router, http.MethodGet, "/api/admin/codes/grand_jury", nil, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, getRes.Code)
		var result []*models.CodeResponse
		require.NoError(t, json.Unmarshal(getRes.Body.Bytes(), &result))
		require.Len(t, result, 0)
	})

	t.Run("Unhappy path - invalid category", func(t *testing.T) {
		getRes := testutils.PerformRequest(router, http.MethodGet, "/api/admin/codes/invalid_category", nil, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusBadRequest, getRes.Code)
	})
}

func cleanupTable(t *testing.T, client *dynamodb.Client, tableName string) {
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

func TestDeleteVotingCodes(t *testing.T) {
	_, router := setupTestAdminController(t)

	// Create 5 codes
	payload := models.CreateCodeRequest{
		Count:    5,
		Category: "general_public",
	}

	postRes := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, map[string]string{
		"x-admin-token": "secret",
	})

	require.Equal(t, http.StatusOK, postRes.Code)

	var created []*models.CodeResponse
	err := json.Unmarshal(postRes.Body.Bytes(), &created)
	require.NoError(t, err)
	require.Len(t, created, 5)

	// Delete each code
	for _, code := range created {
		delRes := testutils.PerformRequest(router, http.MethodDelete, "/api/admin/codes/"+code.Code, nil, map[string]string{
			"x-admin-token": "secret",
		})

		assert.Equal(t, http.StatusOK, delRes.Code, "Failed to delete code %s", code.Code)
	}

	// Verify all deleted
	getRes := testutils.PerformRequest(router, http.MethodGet, "/api/admin/codes", nil, map[string]string{
		"x-admin-token": "secret",
	})

	require.Equal(t, http.StatusOK, getRes.Code)

	var remaining []*models.CodeResponse
	err = json.Unmarshal(getRes.Body.Bytes(), &remaining)
	require.NoError(t, err)
	require.Len(t, remaining, 0)

	// Attempt delete non-existing code
	nonExistRes := testutils.PerformRequest(router, http.MethodDelete, "/api/admin/codes/DOESNOTEXIST", nil, map[string]string{
		"x-admin-token": "secret",
	})

	assert.Equal(t, http.StatusOK, nonExistRes.Code, "expected 200 when deleting non-existent code (idempotent)")
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
		postRes := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, postRes.Code)

		var created []*models.CodeResponse
		err := json.Unmarshal(postRes.Body.Bytes(), &created)
		require.NoError(t, err)
		require.NotEmpty(t, created)
		createdCodes[created[0].Code] = created[0].Category
	}

	// Act - retrieve codes
	getRes := testutils.PerformRequest(router, http.MethodGet, "/api/admin/codes", nil, map[string]string{
		"x-admin-token": "secret",
	})

	require.Equal(t, http.StatusOK, getRes.Code)

	var codes []*models.CodeResponse
	err := json.Unmarshal(getRes.Body.Bytes(), &codes)
	require.NoError(t, err)

	// Assert - verify all created codes are in the response
	for code, category := range createdCodes {
		var found *models.CodeResponse
		for _, c := range codes {
			if c.Code == code {
				found = c
				break
			}
		}
		if assert.NotNil(t, found, "Expected code %s not found in GET response", code) {
			assert.Equal(t, category, found.Category, "Expected category %s, got %s for code %s", category, found.Category, code)
			assert.False(t, found.Used, "Expected Used=false for code %s", code)
			assert.False(t, found.CreatedAt.IsZero(), "Expected non-zero CreatedAt for code %s", code)
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

		w := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, w.Code)
		require.NotEmpty(t, w.Body.Bytes())
	})

	t.Run("Create multiple codes", func(t *testing.T) {
		payload := models.CreateCodeRequest{
			Count:    3,
			Category: "other_team",
		}

		w := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, w.Code)
		require.NotEmpty(t, w.Body.Bytes())
	})

	t.Run("Create with invalid category", func(t *testing.T) {
		payload := models.CreateCodeRequest{
			Count:    1,
			Category: "invalid_category",
		}

		w := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Create with missing or zero count", func(t *testing.T) {
		payload := models.CreateCodeRequest{
			Count:    0,
			Category: "general_public",
		}

		w := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

}

func TestGetCategories(t *testing.T) {
	logging.Log = logrus.New()
	controller := NewAdminController(nil, nil, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/admin/categories", controller.listCategories)

	w := testutils.PerformRequest(r, http.MethodGet, "/api/admin/categories", nil, map[string]string{
		"x-admin-token": "secret",
	})

	require.Equal(t, http.StatusOK, w.Code)

	var result []map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))

	require.Len(t, result, len(models.ValidCategories))

	for _, category := range result {
		require.NotEmpty(t, category["key"])
		require.NotEmpty(t, category["label"])
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

	postRes := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, map[string]string{
		"x-admin-token": "secret",
	})

	require.Equal(t, http.StatusOK, postRes.Code)

	var created []*models.CodeResponse
	err := json.Unmarshal(postRes.Body.Bytes(), &created)
	require.NoError(t, err)
	require.NotEmpty(t, created)
	code := created[0].Code

	// Manually mark the code as used
	created[0].Used = true
	client := storage.DynamoVotingCodesStorage{
		Client: dynamodb.NewFromConfig(aws.Config{Region: "us-east-1", EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: "http://localhost:4566", HostnameImmutable: true}, nil
		})}),
		TableName: "VotingCodes",
	}
	err = client.MarkUsed(context.TODO(), created[0].Code)
	require.NoError(t, err)

	// Reset the code via the API
	resetRes := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes/"+code+"/reset", nil, map[string]string{
		"x-admin-token": "secret",
	})

	require.Equal(t, http.StatusOK, resetRes.Code)

	// Fetch and verify the code is no longer used
	getRes := testutils.PerformRequest(router, http.MethodGet, "/api/admin/codes", nil, map[string]string{
		"x-admin-token": "secret",
	})

	require.Equal(t, http.StatusOK, getRes.Code)

	var codes []*models.CodeResponse
	err = json.Unmarshal(getRes.Body.Bytes(), &codes)
	require.NoError(t, err)
	for _, c := range codes {
		if c.Code == code {
			assert.False(t, c.Used, "expected code %s to be marked as unused", code)
		}
	}
}

func TestAttachTeamToCode(t *testing.T) {
	controller, router := setupTestAdminController(t)

	t.Run("Happy path", func(t *testing.T) {
		// Create teams
		team1 := storage.Team{ID: 101, Name: "Team A"}
		team2 := storage.Team{ID: 102, Name: "Team B"}
		err := controller.teamsStorage.Create(context.TODO(), &team1)
		require.NoError(t, err)
		err = controller.teamsStorage.Create(context.TODO(), &team2)
		require.NoError(t, err)

		// Create 3 codes
		payload := models.CreateCodeRequest{
			Count:    3,
			Category: "other_team",
		}
		postRes := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusOK, postRes.Code)

		var codes []*models.CodeResponse
		err = json.Unmarshal(postRes.Body.Bytes(), &codes)
		require.NoError(t, err)
		require.Len(t, codes, 3)

		// Attach team1 to first code
		res1 := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes/"+codes[0].Code+"/attach-team/101", nil, map[string]string{
			"x-admin-token": "secret",
		})
		assert.Equal(t, http.StatusOK, res1.Code)

		// Attach team2 to second code
		res2 := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes/"+codes[1].Code+"/attach-team/102", nil, map[string]string{
			"x-admin-token": "secret",
		})
		assert.Equal(t, http.StatusOK, res2.Code)

		// Get codes by category and verify all 3
		getRes := testutils.PerformRequest(router, http.MethodGet, "/api/admin/codes/other_team", nil, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusOK, getRes.Code)

		var result []*models.CodeResponse
		err = json.Unmarshal(getRes.Body.Bytes(), &result)
		require.NoError(t, err)
		require.Len(t, result, 3)
	})

	t.Run("Unhappy path - non-existing code", func(t *testing.T) {
		invalidCodeRes := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes/INVALID/attach-team/101", nil, map[string]string{
			"x-admin-token": "secret",
		})
		assert.Equal(t, http.StatusNotFound, invalidCodeRes.Code)
	})
}
func TestDeleteAllVotes(t *testing.T) {
	controller, router := setupTestAdminController(t)

	// Create a team
	team := storage.Team{ID: 201, Name: "Team Z"}
	err := controller.teamsStorage.Create(context.TODO(), &team)
	require.NoError(t, err)

	// Create a voting code and attach the team
	codeReq := models.CreateCodeRequest{
		Count:    1,
		Category: "other_team",
	}
	codeRes := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", codeReq, map[string]string{
		"x-admin-token": "secret",
	})
	require.Equal(t, http.StatusOK, codeRes.Code)

	var codes []*models.CodeResponse
	err = json.Unmarshal(codeRes.Body.Bytes(), &codes)
	require.NoError(t, err)
	require.Len(t, codes, 1)
	voteCode := codes[0].Code

	// Attach team to code
	attachRes := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes/"+voteCode+"/attach-team/201", nil, map[string]string{
		"x-admin-token": "secret",
	})
	require.Equal(t, http.StatusOK, attachRes.Code)

	// Simulate 10 votes (each with 3 entries)
	for i := 0; i < 10; i++ {
		voteReq := models.RegisterVoteRequest{
			Code: voteCode,
			Votes: []models.VoteEntry{
				{CategoryID: 1, TeamID: 301, Rating: 5},
				{CategoryID: 2, TeamID: 302, Rating: 4},
				{CategoryID: 3, TeamID: 303, Rating: 3},
			},
		}
		_ = controller.votesStorage.Create(context.TODO(), &storage.Vote{
			Code:       voteReq.Code,
			SortKey:    fmt.Sprintf("cat#1#team#301#%d", i),
			CategoryID: 1,
			TeamID:     301,
			Rating:     5,
			Timestamp:  time.Now().UTC(),
		})
		_ = controller.votesStorage.Create(context.TODO(), &storage.Vote{
			Code:       voteReq.Code,
			SortKey:    fmt.Sprintf("cat#2#team#302#%d", i),
			CategoryID: 2,
			TeamID:     302,
			Rating:     4,
			Timestamp:  time.Now().UTC(),
		})
		_ = controller.votesStorage.Create(context.TODO(), &storage.Vote{
			Code:       voteReq.Code,
			SortKey:    fmt.Sprintf("cat#3#team#303#%d", i),
			CategoryID: 3,
			TeamID:     303,
			Rating:     3,
			Timestamp:  time.Now().UTC(),
		})
	}

	// Ensure votes exist
	votes, err := controller.votesStorage.GetAll(context.TODO())
	require.NoError(t, err)
	require.NotEmpty(t, votes)

	// Delete all votes
	deleteRes := testutils.PerformRequest(router, http.MethodPost, "/api/admin/votes/delete-all", nil, map[string]string{
		"x-admin-token": "secret",
	})
	require.Equal(t, http.StatusOK, deleteRes.Code)

	// Assert all votes are deleted
	votesAfter, err := controller.votesStorage.GetAll(context.TODO())
	require.NoError(t, err)
	require.Len(t, votesAfter, 0)
}
