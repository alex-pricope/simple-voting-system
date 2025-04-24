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
	r.GET("/api/votes/result", votingController.computeVoteResults)
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

func TestVoteResultsEndpoint(t *testing.T) {
	_, router := setupTestVoteController(t)

	// 1. Create codes
	//categories := []string{"grand_jury", "other_team", "general_public"}
	codeCount := map[string]int{"grand_jury": 3, "other_team": 3, "general_public": 4}
	var codes []string

	for category, count := range codeCount {
		payload := models.CreateCodeRequest{Count: count, Category: category}
		headers := map[string]string{"Content-Type": "application/json", "x-admin-token": "secret"}
		res := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", payload, headers)

		var created []*models.CodeResponse
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &created))
		assert.Equal(t, count, len(created))

		for _, c := range created {
			codes = append(codes, c.Code)
		}
	}

	// 2. Create categories
	voteCategories := []models.VotingCategoryCreateRequest{
		{ID: 1, Name: "Cat1", Description: "C1", Weight: 0.3},
		{ID: 2, Name: "Cat2", Description: "C2", Weight: 0.25},
		{ID: 3, Name: "Cat3", Description: "C3", Weight: 0.25},
		{ID: 4, Name: "Cat4", Description: "C4", Weight: 0.2},
	}
	for _, cat := range voteCategories {
		headers := map[string]string{"Content-Type": "application/json", "x-admin-token": "secret"}
		res := testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", cat, headers)
		require.Equal(t, http.StatusOK, res.Code)
	}

	// 3. Create teams
	for i := 1; i <= 5; i++ {
		team := models.TeamCreateRequest{
			ID: i, Name: fmt.Sprintf("Team %d", i), Members: []string{fmt.Sprintf("Member%d", i)}, Description: "Test",
		}
		headers := map[string]string{"Content-Type": "application/json", "x-admin-token": "secret"}
		res := testutils.PerformRequest(router, http.MethodPost, "/api/meta/teams", team, headers)
		require.Equal(t, http.StatusOK, res.Code)
	}

	// 4. Use 9 of the codes and vote (ensure Team3 > Team2 > Team1 > Team4)
	teamIDs := []int{1, 2, 3, 4, 5}
	categoryRatings := [][]int{
		{5, 4, 3, 2, 1}, // Vote 1
		{4, 5, 3, 2, 1}, // Vote 2
		{3, 2, 5, 1, 4}, // Vote 3
		{2, 3, 4, 1, 5}, // Vote 4
		{1, 2, 3, 5, 4}, // Vote 5
		{1, 1, 5, 2, 3}, // Vote 6
		{1, 1, 5, 4, 2}, // Vote 7
		{3, 4, 2, 5, 1}, // Vote 8
		{5, 1, 2, 3, 4}, // Vote 9
	}

	for i, code := range codes[:9] {
		var entries []models.VoteEntry
		for catID := 1; catID <= 4; catID++ {
			for j, teamID := range teamIDs {
				rating := categoryRatings[i][j]
				entries = append(entries, models.VoteEntry{
					CategoryID: catID,
					TeamID:     teamID,
					Rating:     rating,
				})
			}
		}
		vote := models.RegisterVoteRequest{Code: code, Votes: entries}
		headers := map[string]string{"Content-Type": "application/json"}
		res := testutils.PerformRequest(router, http.MethodPost, "/api/vote/", vote, headers)
		require.Equal(t, http.StatusOK, res.Code)
	}

	// 5. Retrieve results
	res := testutils.PerformRequest(router, http.MethodGet, "/api/votes/result", nil, nil)
	require.Equal(t, http.StatusOK, res.Code)

	var result models.VoteResultsResponse
	err := json.Unmarshal(res.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result.Results, 5)

	// 6. Assert team order
	assert.Equal(t, "Team 3", result.Results[0].TeamName)
	assert.Equal(t, "Team 1", result.Results[1].TeamName)
	assert.Equal(t, "Team 2", result.Results[2].TeamName)
	assert.Equal(t, "Team 5", result.Results[3].TeamName)
	assert.Equal(t, "Team 4", result.Results[4].TeamName)

	// 7. Assert each team has all 4 categories and category scores
	for _, team := range result.Results {
		assert.Len(t, team.Categories, 4)
		for _, c := range team.Categories {
			assert.Greater(t, c.Score, 0.0)
			assert.Contains(t, []string{"Cat1", "Cat2", "Cat3", "Cat4"}, c.CategoryName)
		}
	}

	// 8. Assert the number of codes used
	usedCodes := make(map[string]bool)
	for _, code := range codes[:9] {
		usedCodes[code] = true
	}
	assert.Equal(t, 9, len(usedCodes), "Expected 9 used codes")
}

func TestSingleVotePerCategoryWeighting_GrandJury(t *testing.T) {
	runSingleVoteCategoryWeightingTest(t, "grand_jury", 0.5)
}

func TestSingleVotePerCategoryWeighting_GeneralPublic(t *testing.T) {
	runSingleVoteCategoryWeightingTest(t, "general_public", 0.2)
}

func TestSingleVotePerCategoryWeighting_OtherTeam(t *testing.T) {
	runSingleVoteCategoryWeightingTest(t, "other_team", 0.3)
}

func TestTwoCategoryVoteWeighting_GrandJury(t *testing.T) {
	runTwoCategoryVoteWeightingTest(t, "grand_jury", 0.5)
}

func TestTwoCategoryVoteWeighting_GeneralPublic(t *testing.T) {
	runTwoCategoryVoteWeightingTest(t, "general_public", 0.2)
}

func TestTwoCategoryVoteWeighting_OtherTeam(t *testing.T) {
	runTwoCategoryVoteWeightingTest(t, "other_team", 0.3)
}

func TestTwoCategoryTwoVotesWeighting_GrandJury(t *testing.T) {
	runTwoVotesTwoCategoryWeightingTest(t, "grand_jury", 0.5)
}

func TestTwoCategoryTwoVotesWeighting_GeneralPublic(t *testing.T) {
	runTwoVotesTwoCategoryWeightingTest(t, "general_public", 0.2)
}

func TestTwoCategoryTwoVotesWeighting_OtherTeam(t *testing.T) {
	runTwoVotesTwoCategoryWeightingTest(t, "other_team", 0.3)
}

func runTwoVotesTwoCategoryWeightingTest(t *testing.T, categoryName string, voterWeight float64) {
	_, router := setupTestVoteController(t)

	// Create two categories
	cat1 := models.VotingCategoryCreateRequest{ID: 1, Name: "Cat1", Description: "Category 1", Weight: 0.4}
	cat2 := models.VotingCategoryCreateRequest{ID: 2, Name: "Cat2", Description: "Category 2", Weight: 0.6}
	headers := map[string]string{"Content-Type": "application/json", "x-admin-token": "secret"}
	require.Equal(t, http.StatusOK, testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", cat1, headers).Code)
	require.Equal(t, http.StatusOK, testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", cat2, headers).Code)

	// Create teams
	for i := 1; i <= 5; i++ {
		team := models.TeamCreateRequest{ID: i, Name: fmt.Sprintf("Team %d", i)}
		testutils.PerformRequest(router, http.MethodPost, "/api/meta/teams", team, headers)
	}

	// Create 2 codes
	codePayload := models.CreateCodeRequest{Count: 2, Category: categoryName}
	res := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", codePayload, headers)
	var codes []*models.CodeResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &codes))
	require.Equal(t, 2, len(codes))

	twoVotes := [][]models.VoteEntry{
		{
			{CategoryID: 1, TeamID: 1, Rating: 5},
			{CategoryID: 1, TeamID: 2, Rating: 4},
			{CategoryID: 1, TeamID: 3, Rating: 3},
			{CategoryID: 1, TeamID: 4, Rating: 2},
			{CategoryID: 1, TeamID: 5, Rating: 1},
			{CategoryID: 2, TeamID: 1, Rating: 1},
			{CategoryID: 2, TeamID: 2, Rating: 2},
			{CategoryID: 2, TeamID: 3, Rating: 3},
			{CategoryID: 2, TeamID: 4, Rating: 4},
			{CategoryID: 2, TeamID: 5, Rating: 5},
		},
		{
			{CategoryID: 1, TeamID: 1, Rating: 1},
			{CategoryID: 1, TeamID: 2, Rating: 2},
			{CategoryID: 1, TeamID: 3, Rating: 3},
			{CategoryID: 1, TeamID: 4, Rating: 4},
			{CategoryID: 1, TeamID: 5, Rating: 5},
			{CategoryID: 2, TeamID: 1, Rating: 5},
			{CategoryID: 2, TeamID: 2, Rating: 4},
			{CategoryID: 2, TeamID: 3, Rating: 3},
			{CategoryID: 2, TeamID: 4, Rating: 2},
			{CategoryID: 2, TeamID: 5, Rating: 1},
		},
	}

	for i, vote := range twoVotes {
		voteReq := models.RegisterVoteRequest{Code: codes[i].Code, Votes: vote}
		res := testutils.PerformRequest(router, http.MethodPost, "/api/vote/", voteReq, map[string]string{"Content-Type": "application/json"})
		require.Equal(t, http.StatusOK, res.Code)
	}

	res = testutils.PerformRequest(router, http.MethodGet, "/api/votes/result", nil, nil)
	require.Equal(t, http.StatusOK, res.Code)

	var result models.VoteResultsResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &result))

	expected := map[int]float64{
		1: ((5*0.4 + 1*0.6) + (1*0.4 + 5*0.6)) / 2 * voterWeight,
		2: ((4*0.4 + 2*0.6) + (2*0.4 + 4*0.6)) / 2 * voterWeight,
		3: ((3*0.4 + 3*0.6) + (3*0.4 + 3*0.6)) / 2 * voterWeight,
		4: ((2*0.4 + 4*0.6) + (4*0.4 + 2*0.6)) / 2 * voterWeight,
		5: ((1*0.4 + 5*0.6) + (5*0.4 + 1*0.6)) / 2 * voterWeight,
	}

	for _, r := range result.Results {
		assert.InDelta(t, expected[r.TeamID], r.TotalScore, 0.0001, fmt.Sprintf("Expected total score for Team %d", r.TeamID))
		assert.Len(t, r.Categories, 2)
	}
}

func runSingleVoteCategoryWeightingTest(t *testing.T, categoryName string, voterWeight float64) {
	_, router := setupTestVoteController(t)

	category := models.VotingCategoryCreateRequest{
		ID: 1, Name: "Cat1", Description: "Test category", Weight: 0.5,
	}
	headers := map[string]string{"Content-Type": "application/json", "x-admin-token": "secret"}
	res := testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", category, headers)
	require.Equal(t, http.StatusOK, res.Code)

	for i := 1; i <= 5; i++ {
		team := models.TeamCreateRequest{ID: i, Name: fmt.Sprintf("Team %d", i)}
		testutils.PerformRequest(router, http.MethodPost, "/api/meta/teams", team, headers)
	}

	codePayload := models.CreateCodeRequest{Count: 1, Category: categoryName}
	res = testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", codePayload, headers)
	var codes []*models.CodeResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &codes))
	code := codes[0].Code

	votes := []models.VoteEntry{
		{CategoryID: 1, TeamID: 1, Rating: 5},
		{CategoryID: 1, TeamID: 2, Rating: 4},
		{CategoryID: 1, TeamID: 3, Rating: 3},
		{CategoryID: 1, TeamID: 4, Rating: 2},
		{CategoryID: 1, TeamID: 5, Rating: 1},
	}
	voteReq := models.RegisterVoteRequest{Code: code, Votes: votes}
	voteHeaders := map[string]string{"Content-Type": "application/json"}
	res = testutils.PerformRequest(router, http.MethodPost, "/api/vote/", voteReq, voteHeaders)
	require.Equal(t, http.StatusOK, res.Code)

	res = testutils.PerformRequest(router, http.MethodGet, "/api/votes/result", nil, nil)
	require.Equal(t, http.StatusOK, res.Code)

	var result models.VoteResultsResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &result))

	expected := map[int]float64{
		1: 5 * 0.5 * voterWeight,
		2: 4 * 0.5 * voterWeight,
		3: 3 * 0.5 * voterWeight,
		4: 2 * 0.5 * voterWeight,
		5: 1 * 0.5 * voterWeight,
	}

	for _, r := range result.Results {
		assert.InDelta(t, expected[r.TeamID], r.TotalScore, 0.0001, fmt.Sprintf("Expected score for Team %d", r.TeamID))
		assert.Len(t, r.Categories, 1)
	}
}

func runTwoCategoryVoteWeightingTest(t *testing.T, categoryName string, voterWeight float64) {
	_, router := setupTestVoteController(t)

	// Create two categories
	cat1 := models.VotingCategoryCreateRequest{ID: 1, Name: "Cat1", Description: "Category 1", Weight: 0.4}
	cat2 := models.VotingCategoryCreateRequest{ID: 2, Name: "Cat2", Description: "Category 2", Weight: 0.6}
	headers := map[string]string{"Content-Type": "application/json", "x-admin-token": "secret"}
	require.Equal(t, http.StatusOK, testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", cat1, headers).Code)
	require.Equal(t, http.StatusOK, testutils.PerformRequest(router, http.MethodPost, "/api/meta/categories", cat2, headers).Code)

	// Create teams
	for i := 1; i <= 5; i++ {
		team := models.TeamCreateRequest{ID: i, Name: fmt.Sprintf("Team %d", i)}
		testutils.PerformRequest(router, http.MethodPost, "/api/meta/teams", team, headers)
	}

	// Create code
	codePayload := models.CreateCodeRequest{Count: 1, Category: categoryName}
	res := testutils.PerformRequest(router, http.MethodPost, "/api/admin/codes", codePayload, headers)
	var codes []*models.CodeResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &codes))
	code := codes[0].Code

	// Vote entries for two categories
	votes := []models.VoteEntry{
		{CategoryID: 1, TeamID: 1, Rating: 5}, {CategoryID: 1, TeamID: 2, Rating: 4},
		{CategoryID: 1, TeamID: 3, Rating: 3}, {CategoryID: 1, TeamID: 4, Rating: 2},
		{CategoryID: 1, TeamID: 5, Rating: 1},

		{CategoryID: 2, TeamID: 1, Rating: 1}, {CategoryID: 2, TeamID: 2, Rating: 2},
		{CategoryID: 2, TeamID: 3, Rating: 3}, {CategoryID: 2, TeamID: 4, Rating: 4},
		{CategoryID: 2, TeamID: 5, Rating: 5},
	}
	voteReq := models.RegisterVoteRequest{Code: code, Votes: votes}
	res = testutils.PerformRequest(router, http.MethodPost, "/api/vote/", voteReq, map[string]string{"Content-Type": "application/json"})
	require.Equal(t, http.StatusOK, res.Code)

	// Get results
	res = testutils.PerformRequest(router, http.MethodGet, "/api/votes/result", nil, nil)
	require.Equal(t, http.StatusOK, res.Code)

	var result models.VoteResultsResponse
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &result))

	expected := map[int]float64{
		1: (5*0.4 + 1*0.6) * voterWeight,
		2: (4*0.4 + 2*0.6) * voterWeight,
		3: (3*0.4 + 3*0.6) * voterWeight,
		4: (2*0.4 + 4*0.6) * voterWeight,
		5: (1*0.4 + 5*0.6) * voterWeight,
	}

	for _, r := range result.Results {
		assert.InDelta(t, expected[r.TeamID], r.TotalScore, 0.0001, fmt.Sprintf("Expected total score for Team %d", r.TeamID))
		assert.Len(t, r.Categories, 2)
	}
}
