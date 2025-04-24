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
	"github.com/stretchr/testify/require"
	"net/http"
	"strconv"
	"testing"
)

func cleanupTeamTable(t *testing.T, client *dynamodb.Client, tableName string) {
	t.Helper()
	out, err := client.Scan(context.TODO(), &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	require.NoError(t, err, "cleanup scan failed")
	for _, item := range out.Items {
		key := map[string]types.AttributeValue{
			"PK": item["PK"],
		}
		_, err := client.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
			TableName: aws.String(tableName),
			Key:       key,
		})
		require.NoError(t, err, "cleanup delete failed")
	}
}

func setupTeamTestController(t *testing.T) (*TeamMetaController, *gin.Engine) {
	t.Helper()
	logging.Log = logrus.New()

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-east-1"),
		//nolint:staticcheck
		config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: "http://localhost:4566", HostnameImmutable: true}, nil
			}),
		),
	)
	require.NoError(t, err, "failed to load AWS config")
	client := dynamodb.NewFromConfig(cfg)
	t.Cleanup(func() {
		cleanupTeamTable(t, client, "VotingTeams")
	})
	s := &storage.DynamoTeamStorage{
		Client:    client,
		TableName: "VotingTeams",
	}
	controller := NewTeamMetaController(s)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/meta/teams", controller.create)
	r.PUT("/api/meta/teams/:id", controller.update)
	r.GET("/api/meta/teams/:id", controller.get)
	r.GET("/api/meta/teams", controller.getAll)
	r.DELETE("/api/meta/teams/:id", controller.delete)
	return controller, r
}

func TestCreateTeam(t *testing.T) {
	_, router := setupTeamTestController(t)

	t.Run("Happy path - create team", func(t *testing.T) {
		req := models.TeamCreateRequest{
			ID:          1,
			Name:        "Team Alpha",
			Description: "Test team",
			Members:     []string{"Alice", "Bob"},
		}

		res := testutils.PerformRequest(router, http.MethodPost, "/api/meta/teams", req, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, res.Code, "expected 200 OK: %s", res.Body.String())
	})

	t.Run("Unhappy path - empty name", func(t *testing.T) {
		req := models.TeamCreateRequest{
			ID:          2,
			Name:        "",
			Description: "No name",
			Members:     []string{"X", "Y"},
		}
		body, err := json.Marshal(req)
		require.NoError(t, err)

		res := testutils.PerformRequest(router, http.MethodPost, "/api/meta/teams", body, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusBadRequest, res.Code, "expected 400: %s", res.Body.String())
	})

	t.Run("Unhappy path - duplicate ID", func(t *testing.T) {
		req := models.TeamCreateRequest{
			ID:          1,
			Name:        "Duplicate",
			Description: "Should conflict",
			Members:     []string{"Z"},
		}

		res := testutils.PerformRequest(router, http.MethodPost, "/api/meta/teams", req, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusConflict, res.Code, "expected 409 Conflict: %s", res.Body.String())
	})
}

func TestUpdateTeam(t *testing.T) {
	_, router := setupTeamTestController(t)

	t.Run("Happy path - update team fields and remove a member", func(t *testing.T) {
		// Create
		createReq := models.TeamCreateRequest{
			ID:          10,
			Name:        "Team Rocket",
			Description: "Villains",
			Members:     []string{"Jessie", "James", "Meowth"},
		}

		res := testutils.PerformRequest(router, http.MethodPost, "/api/meta/teams", createReq, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusOK, res.Code, "expected 200 OK on create")

		// Update: change name, description, remove Meowth
		updateReq := models.TeamUpdateRequest{
			Name:        "Team Rocket Updated",
			Description: "Still villains",
			Members:     []string{"Jessie", "James"},
		}

		res = testutils.PerformRequest(router, http.MethodPut, "/api/meta/teams/10", updateReq, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, res.Code, "expected 200 OK: %s", res.Body.String())

		// Get and validate
		res = testutils.PerformRequest(router, http.MethodGet, "/api/meta/teams/10", nil, map[string]string{
			"x-admin-token": "secret",
		})

		var response models.TeamResponse
		err := json.Unmarshal(res.Body.Bytes(), &response)
		require.NoError(t, err, "failed to unmarshal response")
		assert.Equal(t, "Team Rocket Updated", response.Name)
		assert.Equal(t, "Still villains", response.Description)
		assert.Len(t, response.Members, 2)
		assert.Equal(t, createReq.ID, response.ID)
	})

	t.Run("Happy path - add new member", func(t *testing.T) {
		// Update existing team with a new member
		updateReq := models.TeamUpdateRequest{
			Name:        "Team Rocket Final",
			Description: "Updated again",
			Members:     []string{"Jessie", "James", "Wobbuffet"},
		}

		res := testutils.PerformRequest(router, http.MethodPut, "/api/meta/teams/10", updateReq, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, res.Code, "expected 200 OK: %s", res.Body.String())

		res = testutils.PerformRequest(router, http.MethodGet, "/api/meta/teams/10", nil, map[string]string{
			"x-admin-token": "secret",
		})

		var response models.TeamResponse
		err := json.Unmarshal(res.Body.Bytes(), &response)
		require.NoError(t, err, "failed to unmarshal response")
		assert.Len(t, response.Members, 3)
		assert.Equal(t, "Wobbuffet", response.Members[2])
	})

	t.Run("Unhappy path - invalid ID in path", func(t *testing.T) {
		res := testutils.PerformRequest(router, http.MethodPut, "/api/meta/teams/notanumber", []byte(`{}`), map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusBadRequest, res.Code, "expected 400 for invalid ID")
	})

	t.Run("Unhappy path - missing name", func(t *testing.T) {
		reqBody := models.TeamUpdateRequest{
			Name:        "",
			Description: "Missing name",
			Members:     []string{"Alpha"},
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)
		res := testutils.PerformRequest(router, http.MethodPut, "/api/meta/teams/10", body, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusBadRequest, res.Code, "expected 400 for empty name")
	})
}

func TestGetTeamByID(t *testing.T) {
	_, router := setupTeamTestController(t)

	t.Run("Happy path - get team by ID", func(t *testing.T) {
		req := models.TeamCreateRequest{
			ID:          20,
			Name:        "Team Lookup",
			Description: "Search test",
			Members:     []string{"One", "Two"},
		}

		res := testutils.PerformRequest(router, http.MethodPost, "/api/meta/teams", req, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusOK, res.Code, "expected 200 OK on create")

		getRes := testutils.PerformRequest(router, http.MethodGet, "/api/meta/teams/20", nil, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, getRes.Code, "expected 200 OK: %s", getRes.Body.String())

		var response models.TeamResponse
		err := json.Unmarshal(getRes.Body.Bytes(), &response)
		require.NoError(t, err, "failed to parse response")
		require.Equal(t, 20, response.ID)
		require.Equal(t, req.Name, response.Name)
		require.Equal(t, req.Description, response.Description)
		require.Len(t, response.Members, 2)
	})

	t.Run("Unhappy path - invalid ID format", func(t *testing.T) {
		res := testutils.PerformRequest(router, http.MethodGet, "/api/meta/teams/notanumber", nil, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusBadRequest, res.Code, "expected 400 for invalid ID")
	})

	t.Run("Unhappy path - non-existing ID", func(t *testing.T) {
		res := testutils.PerformRequest(router, http.MethodGet, "/api/meta/teams/999", nil, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusNotFound, res.Code, "expected 404 for non-existing ID")
	})
}

func TestListTeams(t *testing.T) {
	_, router := setupTeamTestController(t)

	// Create 5 teams
	for i := 1; i <= 5; i++ {
		req := models.TeamCreateRequest{
			ID:          i,
			Name:        "Team " + strconv.Itoa(i),
			Description: "Description " + strconv.Itoa(i),
			Members:     []string{"MemberA", "MemberB"},
		}
		res := testutils.PerformRequest(router, http.MethodPost, "/api/meta/teams", req, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, res.Code, "POST team %d failed: %d - %s", i, res.Code, res.Body.String())
	}

	// Get all teams
	getRes := testutils.PerformRequest(router, http.MethodGet, "/api/meta/teams", nil, map[string]string{
		"x-admin-token": "secret",
	})

	require.Equal(t, http.StatusOK, getRes.Code, "expected 200 OK")

	var teams []models.TeamResponse
	err := json.Unmarshal(getRes.Body.Bytes(), &teams)
	require.NoError(t, err, "failed to parse team list")
	assert.Len(t, teams, 5)

	for _, team := range teams {
		assert.GreaterOrEqual(t, team.ID, 1, "unexpected team ID")
		assert.LessOrEqual(t, team.ID, 5, "unexpected team ID")
		assert.NotEmpty(t, team.Name, "missing name for team ID %d", team.ID)
		assert.NotEmpty(t, team.Description, "missing description for team ID %d", team.ID)
		assert.Len(t, team.Members, 2, "unexpected members for team ID %d", team.ID)
	}
}

func TestDeleteTeam(t *testing.T) {
	_, router := setupTeamTestController(t)

	t.Run("Happy path - delete existing team", func(t *testing.T) {
		req := models.TeamCreateRequest{
			ID:          99,
			Name:        "DeleteMe",
			Description: "To be deleted",
			Members:     []string{"Ghost"},
		}
		createRes := testutils.PerformRequest(router, http.MethodPost, "/api/meta/teams", req, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusOK, createRes.Code, "setup create failed: %d - %s", createRes.Code, createRes.Body.String())

		// Delete the team
		delRes := testutils.PerformRequest(router, http.MethodDelete, "/api/meta/teams/99", nil, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusOK, delRes.Code, "expected 200 on delete")

		// Ensure it's gone
		getRes := testutils.PerformRequest(router, http.MethodGet, "/api/meta/teams/99", nil, map[string]string{
			"x-admin-token": "secret",
		})
		require.Equal(t, http.StatusNotFound, getRes.Code, "expected 404 after delete")
	})

	t.Run("Unhappy path - delete non-existing ID", func(t *testing.T) {
		res := testutils.PerformRequest(router, http.MethodDelete, "/api/meta/teams/1000", nil, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusOK, res.Code, "expected 200 for idempotent delete")
	})

	t.Run("Unhappy path - invalid ID format", func(t *testing.T) {
		res := testutils.PerformRequest(router, http.MethodDelete, "/api/meta/teams/notanumber", nil, map[string]string{
			"x-admin-token": "secret",
		})

		require.Equal(t, http.StatusBadRequest, res.Code, "expected 400 for invalid ID")
	})
}
