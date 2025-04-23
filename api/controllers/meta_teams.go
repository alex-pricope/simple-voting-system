package controllers

import (
	"errors"
	"github.com/alex-pricope/simple-voting-system/api/models"
	"github.com/alex-pricope/simple-voting-system/api/transport"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/alex-pricope/simple-voting-system/storage"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

type TeamMetaController struct {
	storage storage.TeamStorage
}

func NewTeamMetaController(s storage.TeamStorage) *TeamMetaController {
	return &TeamMetaController{storage: s}
}

func (c *TeamMetaController) RegisterRoutes(engine *gin.Engine) {
	group := engine.Group("/api/meta/teams")

	group.GET("", c.getAll)
	group.GET("/:id", transport.AdminAuthMiddleware(), c.get)
	group.POST("", transport.AdminAuthMiddleware(), c.create)
	group.PUT("/:id", transport.AdminAuthMiddleware(), c.update)
	group.DELETE("/:id", transport.AdminAuthMiddleware(), c.delete)
}

// @Summary Get all teams
// @Tags Meta/Teams
// @Produce json
// @Success 200 {array} models.TeamResponse
// @Failure 500 {object} map[string]string
// @Router /api/meta/teams [get]
func (c *TeamMetaController) getAll(g *gin.Context) {
	teams, err := c.storage.GetAll(g.Request.Context())
	if err != nil {
		logging.Log.Errorf("META: failed to get all teams: %v", err)
		g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	responses := make([]models.TeamResponse, 0, len(teams))
	for _, t := range teams {
		responses = append(responses, models.TransformTeamFromStorage(t))
	}
	g.JSON(http.StatusOK, responses)
}

// @Summary Get a team by ID
// @Tags Meta/Teams
// @Produce json
// @Param id path int true "Team ID"
// @Success 200 {object} models.TeamResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/meta/teams/{id} [get]
func (c *TeamMetaController) get(g *gin.Context) {
	idStr := g.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}
	team, err := c.storage.Get(g.Request.Context(), id)
	if err != nil {
		logging.Log.Errorf("META: failed to get team: %v", err)
		g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if team == nil {
		g.JSON(http.StatusNotFound, gin.H{"error": "team not found"})
		return
	}
	g.JSON(http.StatusOK, models.TransformTeamFromStorage(team))
}

// @Security AdminToken
// @Summary Create a team
// @Tags Meta/Teams
// @Accept json
// @Produce json
// @Param team body models.TeamCreateRequest true "Team object"
// @Success 200 {object} models.TeamResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/meta/teams [post]
func (c *TeamMetaController) create(g *gin.Context) {
	var req models.TeamCreateRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		logging.Log.Errorf("META: invalid create team request: %v", err)
		g.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Name == "" {
		logging.Log.Errorf("META: invalid create team request: %v", req)
		g.JSON(http.StatusBadRequest, gin.H{"error": "invalid request empty name"})
		return
	}

	if req.Name == "" {
		logging.Log.Errorf("META: invalid update team request: %v", req)
		g.JSON(http.StatusBadRequest, gin.H{"error": "invalid request empty name"})
		return
	}

	team := &storage.Team{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Members:     req.Members,
	}

	if err := c.storage.Create(g.Request.Context(), team); err != nil {
		if errors.Is(err, storage.ErrItemWithIDAlreadyExists) {
			logging.Log.Warnf("META: team with ID %d already exists", req.ID)
			g.JSON(http.StatusConflict, gin.H{"error": "team with ID already exists"})
			return
		}

		logging.Log.Errorf("META: failed to create team: %v", err)
		g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	g.JSON(http.StatusOK, models.TransformTeamFromStorage(team))
}

// @Security AdminToken
// @Summary Update an existing team
// @Tags Meta/Teams
// @Accept json
// @Produce json
// @Param id path int true "Team ID"
// @Param team body models.TeamUpdateRequest true "Team update object"
// @Success 200 {object} models.TeamResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/meta/teams/{id} [put]
func (c *TeamMetaController) update(g *gin.Context) {
	idStr := g.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}

	var req models.TeamUpdateRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		logging.Log.Errorf("META: invalid update team request: %v", err)
		g.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Name == "" {
		logging.Log.Errorf("META: invalid update team request: %v", req)
		g.JSON(http.StatusBadRequest, gin.H{"error": "invalid request empty name"})
		return
	}

	team := &storage.Team{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Members:     req.Members,
	}

	if err := c.storage.Update(g.Request.Context(), team); err != nil {
		logging.Log.Errorf("META: failed to update team: %v", err)
		g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	g.JSON(http.StatusOK, models.TransformTeamFromStorage(team))
}

// @Security AdminToken
// @Summary Delete a team
// @Tags Meta/Teams
// @Produce json
// @Param id path int true "Team ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/meta/teams/{id} [delete]
func (c *TeamMetaController) delete(g *gin.Context) {
	idStr := g.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		g.JSON(http.StatusBadRequest, gin.H{"error": "invalid team id"})
		return
	}
	if err := c.storage.Delete(g.Request.Context(), id); err != nil {
		logging.Log.Errorf("META: failed to delete team: %v", err)
		g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	g.JSON(http.StatusOK, gin.H{"message": "team deleted"})
}
