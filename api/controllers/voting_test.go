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
	"github.com/stretchr/testify/assert"
	"net/http"
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
	teamStorage := &storage.DynamoTeamStorage{
		Client:    db,
		TableName: "VotingTeams",
	}
	categoriesStorage := &storage.DynamoVotingCategoryStorage{
		Client:    db,
		TableName: "VotingCategories",
	}

	t.Cleanup(func() {
		cleanupTable(t, db, "VotingCodes")
		cleanupTableVotes(t, db)
		cleanupTable(t, db, "VotingTeams")
		cleanupTable(t, db, "VotingCategories")
	})

	votingController := NewVotingController(codeStorage, voteStorage, teamStorage, categoriesStorage)
	adminController := NewAdminController(codeStorage)
	teamsController := NewTeamMetaController(teamStorage)
	categoriesController := NewCategoryMetaController(categoriesStorage)
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.GET("/api/verify/:code", votingController.validateVotingCode)
	r.POST("/api/vote/", votingController.registerVote)
	r.GET("/api/vote/:code", votingController.getVotesByCode)
	r.POST("/api/admin/codes", adminController.createCode)
	r.POST("/api/meta/teams", teamsController.create)
	r.POST("/api/meta/categories", categoriesController.create)

	return votingController, r
}

func cleanupTableVotes(t *testing.T, client *dynamodb.Client) {
	t.Helper()

	out, err := client.Scan(context.TODO(), &dynamodb.ScanInput{
		TableName: aws.String("Votes"),
	})
	if err != nil {
		t.Fatalf("cleanup failed to scan %s: %v", "Votes", err)
	}

	for _, item := range out.Items {
		key := make(map[string]types.AttributeValue)

		if pk, ok := item["PK"]; ok {
			key["PK"] = pk
		}
		if sk, ok := item["SK"]; ok {
			key["SK"] = sk
		}

		_, err := client.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
			TableName: aws.String("Votes"),
			Key:       key,
		})
		if err != nil {
			t.Fatalf("cleanup failed to delete item from %s: %v", "Votes", err)
		}
	}
}

//nolint:staticcheck
func TestValidateVotingCode(t *testing.T) {
	_, router := setupTestVoteController(t)

	t.Run("Happy path - verify valid code", func(t *testing.T) {
		payload := models.CreateCodeRequest{
			Count:    1,
			Category: "general_public",
		}
		headers := map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		}
		w := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, headers)

		var created []*models.CodeResponse
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &created), "Should unmarshal created codes")
		assert.NotEmpty(t, created, "Created codes should not be empty")
		code := created[0].Code

		validateRes := testutils.PerformRequest(router, http.MethodGet, "/api/verify/"+code, nil, nil)

		assert.Equal(t, http.StatusOK, validateRes.Code, "Expected 200 from verify")

		var response models.CodeValidationResponse
		assert.NoError(t, json.Unmarshal(validateRes.Body.Bytes(), &response), "Should parse verification response")
		assert.True(t, response.Valid, "Response should be valid")
		assert.Equal(t, "general_public", response.Category, "Expected category to match")
	})

	t.Run("Unhappy path - non-existent code", func(t *testing.T) {
		res := testutils.PerformRequest(router, http.MethodGet, "/api/verify/NOTEXIST", nil, nil)

		assert.Equal(t, http.StatusNotFound, res.Code, "Expected 404 for non-existent code")
	})

	t.Run("Unhappy path - used code", func(t *testing.T) {
		// Create a code
		payload := models.CreateCodeRequest{
			Count:    1,
			Category: "general_public",
		}
		headers := map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		}
		w := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, headers)

		var created []*models.CodeResponse
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &created), "Should unmarshal created codes")
		assert.NotEmpty(t, created, "Created codes should not be empty")
		code := created[0].Code

		// Mark as used
		client := storage.DynamoVotingCodesStorage{
			Client: dynamodb.NewFromConfig(aws.Config{Region: "us-east-1", EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: "http://localhost:4566", HostnameImmutable: true}, nil
			})}),
			TableName: "VotingCodes",
		}
		err := client.MarkUsed(context.TODO(), code)
		assert.NoError(t, err, "Should mark code as used without error")

		// Validate again
		validateRes := testutils.PerformRequest(router, http.MethodGet, "/api/verify/"+code, nil, nil)

		assert.Equal(t, http.StatusOK, validateRes.Code, "Expected 200 after re-validation")
		var response models.CodeValidationResponse
		assert.NoError(t, json.Unmarshal(validateRes.Body.Bytes(), &response), "Should parse re-validation response")
		assert.True(t, response.Valid, "Code should still be valid")
		assert.True(t, response.Used, "Code should be marked as used")
	})
}

func TestGetVotesByCode(t *testing.T) {
	_, router := setupTestVoteController(t)

	t.Run("Happy path - submit and retrieve vote", func(t *testing.T) {
		// Step 1: Create code
		payload := models.CreateCodeRequest{
			Count:    1,
			Category: "general_public",
		}
		headers := map[string]string{
			"Content-Type":  "application/json",
			"x-admin-token": "secret",
		}
		w := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, headers)

		var created []*models.CodeResponse
		if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil || len(created) == 0 {
			t.Fatalf("failed to unmarshal created code: %v", err)
		}
		code := created[0].Code

		// Create teams
		teams := []models.TeamCreateRequest{
			{ID: 1, Name: "Team Alpha", Members: []string{"Alice"}, Description: "Alpha team"},
			{ID: 2, Name: "Team Beta", Members: []string{"Bob"}, Description: "Beta team"},
		}
		for _, team := range teams {
			teamHeaders := map[string]string{
				"Content-Type":  "application/json",
				"x-admin-token": "secret",
			}
			testutils.PerformRequest(router, http.MethodPost, "/api/meta/teams", team, teamHeaders)
		}

		// Create categories
		categories := []models.VotingCategoryCreateRequest{
			{ID: 1, Name: "Tech / Design / Innovation", Description: "Evaluate implementation and creativity"},
			{ID: 2, Name: "Fun / Potential", Description: "How fun and engaging it is"},
		}
		for _, cat := range categories {
			catHeaders := map[string]string{
				"Content-Type":  "application/json",
				"x-admin-token": "secret",
			}
			testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", cat, catHeaders)
		}

		// Step 2: Submit vote
		votePayload := models.RegisterVoteRequest{
			Code: code,
			Votes: []models.VoteEntry{
				{CategoryID: 1, TeamID: 1, Rating: 3},
				{CategoryID: 1, TeamID: 2, Rating: 4},
				{CategoryID: 2, TeamID: 1, Rating: 5},
			},
		}
		voteHeaders := map[string]string{
			"Content-Type": "application/json",
		}

		voteRes := testutils.PerformRequest(router, http.MethodPost, "/api/vote/", votePayload, voteHeaders)
		assert.Equal(t, http.StatusOK, voteRes.Code, "Vote POST should return 200 OK")

		// Step 3: Retrieve vote
		getRes := testutils.PerformRequest(router, http.MethodGet, "/api/vote/"+code, nil, nil)

		assert.Equal(t, http.StatusOK, getRes.Code, "GET /vote/:code should return 200 OK")

		var voteResponse models.GetVoteResponse
		err := json.Unmarshal(getRes.Body.Bytes(), &voteResponse)
		assert.NoError(t, err, "Should unmarshal vote response without error")

		assert.Equal(t, code, voteResponse.Code, "Returned code should match submitted")
		assert.Len(t, voteResponse.Votes, 3, "Should contain 3 vote entries")

		expectedTeams := map[int]string{
			1: "Team Alpha",
			2: "Team Beta",
		}
		expectedCategories := map[int]string{
			1: "Tech / Design / Innovation",
			2: "Fun / Potential",
		}

		for _, v := range voteResponse.Votes {
			expectedTeam, teamOk := expectedTeams[v.TeamID]
			expectedCategory, catOk := expectedCategories[v.CategoryID]

			if teamOk {
				assert.Equal(t, expectedTeam, v.Team, "Team name should match expected")
			}
			if catOk {
				assert.Equal(t, expectedCategory, v.Category, "Category name should match expected")
			}
		}
	})
}
