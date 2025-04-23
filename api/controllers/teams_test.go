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

func cleanupTeamTable(t *testing.T, client *dynamodb.Client, tableName string) {
	t.Helper()
	out, err := client.Scan(context.TODO(), &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		t.Fatalf("cleanup scan failed: %v", err)
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
			t.Fatalf("cleanup delete failed: %v", err)
		}
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
	if err != nil {
		t.Fatalf("failed to load AWS config: %v", err)
	}
	client := dynamodb.NewFromConfig(cfg)
	t.Cleanup(func() {
		cleanupTeamTable(t, client, "Teams")
	})
	s := &storage.DynamoTeamStorage{
		Client:    client,
		TableName: "Teams",
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
		body, _ := json.Marshal(req)

		r := httptest.NewRequest(http.MethodPost, "/api/meta/teams", bytes.NewBuffer(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Unhappy path - empty name", func(t *testing.T) {
		req := models.TeamCreateRequest{
			ID:          2,
			Name:        "",
			Description: "No name",
			Members:     []string{"X", "Y"},
		}
		body, _ := json.Marshal(req)

		r := httptest.NewRequest(http.MethodPost, "/api/meta/teams", bytes.NewBuffer(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, r)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("Unhappy path - duplicate ID", func(t *testing.T) {
		req := models.TeamCreateRequest{
			ID:          1,
			Name:        "Duplicate",
			Description: "Should conflict",
			Members:     []string{"Z"},
		}
		body, _ := json.Marshal(req)

		r := httptest.NewRequest(http.MethodPost, "/api/meta/teams", bytes.NewBuffer(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, r)

		if w.Code != http.StatusConflict {
			t.Fatalf("expected 409 Conflict, got %d: %s", w.Code, w.Body.String())
		}
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
		body, _ := json.Marshal(createReq)
		req := httptest.NewRequest(http.MethodPost, "/api/meta/teams", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Update: change name, description, remove Meowth
		updateReq := models.TeamUpdateRequest{
			Name:        "Team Rocket Updated",
			Description: "Still villains",
			Members:     []string{"Jessie", "James"},
		}
		updateBody, _ := json.Marshal(updateReq)
		req = httptest.NewRequest(http.MethodPut, "/api/meta/teams/10", bytes.NewBuffer(updateBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		// Get and validate
		req = httptest.NewRequest(http.MethodGet, "/api/meta/teams/10", nil)
		req.Header.Set("x-admin-token", "secret")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var res models.TeamResponse
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if res.Name != "Team Rocket Updated" || res.Description != "Still villains" ||
			len(res.Members) != 2 || res.ID != createReq.ID {
			t.Errorf("unexpected update result: %+v", res)
		}
	})

	t.Run("Happy path - add new member", func(t *testing.T) {
		// Update existing team with a new member
		updateReq := models.TeamUpdateRequest{
			Name:        "Team Rocket Final",
			Description: "Updated again",
			Members:     []string{"Jessie", "James", "Wobbuffet"},
		}
		body, _ := json.Marshal(updateReq)
		req := httptest.NewRequest(http.MethodPut, "/api/meta/teams/10", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/api/meta/teams/10", nil)
		req.Header.Set("x-admin-token", "secret")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		var res models.TeamResponse
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if len(res.Members) != 3 || res.Members[2] != "Wobbuffet" {
			t.Errorf("expected new member Wobbuffet, got %+v", res.Members)
		}
	})

	t.Run("Unhappy path - invalid ID in path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/meta/teams/notanumber", bytes.NewBuffer([]byte(`{}`)))
		req.Header.Set("x-admin-token", "secret")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid ID, got %d", w.Code)
		}
	})

	t.Run("Unhappy path - missing name", func(t *testing.T) {
		reqBody := models.TeamUpdateRequest{
			Name:        "",
			Description: "Missing name",
			Members:     []string{"Alpha"},
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPut, "/api/meta/teams/10", bytes.NewBuffer(body))
		req.Header.Set("x-admin-token", "secret")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty name, got %d", w.Code)
		}
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
		body, _ := json.Marshal(req)

		r := httptest.NewRequest(http.MethodPost, "/api/meta/teams", bytes.NewBuffer(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		getReq := httptest.NewRequest(http.MethodGet, "/api/meta/teams/20", nil)
		getReq.Header.Set("x-admin-token", "secret")
		getRes := httptest.NewRecorder()
		router.ServeHTTP(getRes, getReq)

		if getRes.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", getRes.Code, getRes.Body.String())
		}

		var res models.TeamResponse
		if err := json.Unmarshal(getRes.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if res.ID != 20 || res.Name != req.Name || res.Description != req.Description || len(res.Members) != 2 {
			t.Fatalf("unexpected data in team response: %+v", res)
		}
	})

	t.Run("Unhappy path - invalid ID format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/meta/teams/notanumber", nil)
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid ID, got %d", w.Code)
		}
	})

	t.Run("Unhappy path - non-existing ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/meta/teams/999", nil)
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for non-existing ID, got %d", w.Code)
		}
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
		body, _ := json.Marshal(req)
		r := httptest.NewRequest(http.MethodPost, "/api/meta/teams", bytes.NewBuffer(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("POST team %d failed: %d - %s", i, w.Code, w.Body.String())
		}
	}

	// Get all teams
	getReq := httptest.NewRequest(http.MethodGet, "/api/meta/teams", nil)
	getReq.Header.Set("x-admin-token", "secret")
	getRes := httptest.NewRecorder()
	router.ServeHTTP(getRes, getReq)

	if getRes.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", getRes.Code)
	}

	var teams []models.TeamResponse
	if err := json.Unmarshal(getRes.Body.Bytes(), &teams); err != nil {
		t.Fatalf("failed to parse team list: %v", err)
	}
	if len(teams) != 5 {
		t.Fatalf("expected 5 teams, got %d", len(teams))
	}

	for _, team := range teams {
		if team.ID < 1 || team.ID > 5 {
			t.Errorf("unexpected team ID: %d", team.ID)
		}
		if team.Name == "" || team.Description == "" {
			t.Errorf("missing name or description for team ID %d", team.ID)
		}
		if len(team.Members) != 2 {
			t.Errorf("unexpected members for team ID %d: %+v", team.ID, team.Members)
		}
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
		body, _ := json.Marshal(req)
		createReq := httptest.NewRequest(http.MethodPost, "/api/meta/teams", bytes.NewBuffer(body))
		createReq.Header.Set("Content-Type", "application/json")
		createReq.Header.Set("x-admin-token", "secret")
		createRes := httptest.NewRecorder()
		router.ServeHTTP(createRes, createReq)
		if createRes.Code != http.StatusOK {
			t.Fatalf("setup create failed: %d - %s", createRes.Code, createRes.Body.String())
		}

		// Delete the team
		delReq := httptest.NewRequest(http.MethodDelete, "/api/meta/teams/99", nil)
		delReq.Header.Set("x-admin-token", "secret")
		delRes := httptest.NewRecorder()
		router.ServeHTTP(delRes, delReq)
		if delRes.Code != http.StatusOK {
			t.Fatalf("expected 200 on delete, got %d", delRes.Code)
		}

		// Ensure it's gone
		getReq := httptest.NewRequest(http.MethodGet, "/api/meta/teams/99", nil)
		getReq.Header.Set("x-admin-token", "secret")
		getRes := httptest.NewRecorder()
		router.ServeHTTP(getRes, getReq)
		if getRes.Code != http.StatusNotFound {
			t.Fatalf("expected 404 after delete, got %d", getRes.Code)
		}
	})

	t.Run("Unhappy path - delete non-existing ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/meta/teams/1000", nil)
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 for idempotent delete, got %d", w.Code)
		}
	})

	t.Run("Unhappy path - invalid ID format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/meta/teams/notanumber", nil)
		req.Header.Set("x-admin-token", "secret")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for invalid ID, got %d", w.Code)
		}
	})
}
