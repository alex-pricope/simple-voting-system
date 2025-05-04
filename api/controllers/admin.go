package controllers

import (
	"github.com/alex-pricope/simple-voting-system/api/models"
	"github.com/alex-pricope/simple-voting-system/api/transport"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/alex-pricope/simple-voting-system/storage"
	"github.com/gin-gonic/gin"
	"github.com/matoous/go-nanoid/v2"
	"net/http"
	"strconv"
	"time"
)

type AdminController struct {
	codesStorage storage.VotingCodeStorage
	teamsStorage storage.TeamStorage
}

func NewAdminController(codes storage.VotingCodeStorage, teams storage.TeamStorage) *AdminController {
	return &AdminController{
		codesStorage: codes,
		teamsStorage: teams,
	}
}

func (c *AdminController) RegisterRoutes(engine *gin.Engine) {
	group := engine.Group("/api/admin", transport.AdminAuthMiddleware())

	group.GET("/codes", c.listCodes)
	group.POST("/codes", c.createCode)
	group.DELETE("/codes/:code", c.deleteCode)
	group.POST("/codes/reset", c.resetVotes)
	group.POST("/codes/:code/reset", c.resetCode)
	group.POST("/codes/:code/attach-team/:teamId", c.attachTeam)
	group.GET("/categories", c.listCategories)
	group.GET("/codes/:category", c.getCodesByCategory)
}

// @Security AdminToken
// listCodes godoc
// @Summary List all voting codes
// @Tags admin
// @Produce json
// @Success 200 {array} models.CodeResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/admin/codes [get]
func (c *AdminController) listCodes(g *gin.Context) {
	codes, err := c.codesStorage.GetAll(g.Request.Context())
	if err != nil {
		logging.Log.Errorf("ADMIN: failed to list codes: %v", err)
		g.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	responseCodes := make([]*models.CodeResponse, 0, len(codes))
	for _, code := range codes {
		responseCodes = append(responseCodes, models.TransformVotingCodeToCodeResponse(code))
	}
	logging.Log.Infof("ADMIN: listed %d codes", len(responseCodes))
	g.JSON(http.StatusOK, responseCodes)
}

// @Security AdminToken
// createCode godoc
// @Summary Create one or more voting codes
// @Tags admin
// @Accept json
// @Produce json
// @Param request body models.CreateCodeRequest true "Create Code Request"
// @Success 200 {array} models.CodeResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/admin/codes [post]
func (c *AdminController) createCode(g *gin.Context) {
	var req models.CreateCodeRequest
	if err := g.ShouldBindJSON(&req); err != nil || req.Category == "" || req.Count < 1 {
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request, missing category or count"})
		return
	}

	if _, ok := models.ValidCategories[models.VotingCategory(req.Category)]; !ok {
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid category"})
		logging.Log.Warnf("ADMIN: attempted to create code with invalid category: %s", req.Category)
		return
	}

	codes := make([]*storage.VotingCode, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		code := &storage.VotingCode{
			Code:      c.generateShortCode(),
			Category:  req.Category,
			CreatedAt: time.Now().UTC(),
			Used:      false,
		}
		if err := c.codesStorage.Put(g.Request.Context(), code); err == nil {
			logging.Log.Infof("ADMIN: created code: %s with category %s", code.Code, code.Category)
			codes = append(codes, code)
		} else {
			logging.Log.Errorf("ADMIN: failed to store code: %v", err)
		}
	}

	responseCodes := make([]*models.CodeResponse, 0, len(codes))
	for _, code := range codes {
		responseCodes = append(responseCodes, models.TransformVotingCodeToCodeResponse(code))
	}
	g.JSON(http.StatusOK, responseCodes)
}

// @Security AdminToken
// deleteCode godoc
// @Summary Delete a voting code by its value
// @Tags admin
// @Produce json
// @Param code path string true "Voting code"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/admin/codes/{code} [delete]
func (c *AdminController) deleteCode(g *gin.Context) {
	code := g.Param("code")
	if code == "" {
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "missing code"})
		return
	}
	if err := c.codesStorage.Delete(g.Request.Context(), code); err != nil {
		logging.Log.Errorf("ADMIN: failed to delete code %s: %v", code, err)
		g.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}
	logging.Log.Infof("ADMIN: deleted code: %s", code)
	g.JSON(http.StatusOK, gin.H{"deleted": code})
}

// @Security AdminToken
// resetVotes godoc
// @Summary Reset all voting codes to unused
// @Tags admin
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} models.ErrorResponse
// @Router /api/admin/codes/reset [post]
func (c *AdminController) resetVotes(g *gin.Context) {
	codes, err := c.codesStorage.GetAll(g.Request.Context())
	if err != nil {
		logging.Log.Errorf("ADMIN: failed to get codes for reset: %v", err)
		g.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	updated := 0
	for _, voteCode := range codes {
		voteCode.Used = false
		if err := c.codesStorage.MarkUnused(g.Request.Context(), voteCode.Code); err != nil {
			logging.Log.Errorf("ADMIN: failed to reset code %s: %v", voteCode.Code, err)
		} else {
			updated++
		}
	}

	logging.Log.Infof("ADMIN: reset %d codes", updated)
	g.JSON(http.StatusOK, gin.H{"message": "All codes reset"})
}

// @Security AdminToken
// resetCode godoc
// @Summary Reset a specific voting code to unused
// @Tags admin
// @Produce json
// @Param code path string true "Voting code"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/admin/codes/{code}/reset [post]
func (c *AdminController) resetCode(g *gin.Context) {
	code := g.Param("code")
	if code == "" {
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "missing code"})
		return
	}

	voteCode, err := c.codesStorage.Get(g.Request.Context(), code)
	if err != nil {
		logging.Log.Errorf("ADMIN: failed to retrieve code %s for reset: %v", code, err)
		g.JSON(http.StatusNotFound, models.ErrorResponse{Error: "code not found"})
		return
	}

	voteCode.Used = false
	if err := c.codesStorage.MarkUnused(g.Request.Context(), voteCode.Code); err != nil {
		logging.Log.Errorf("ADMIN: failed to reset code %s: %v", code, err)
		g.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to reset code"})
		return
	}

	logging.Log.Infof("ADMIN: reset code: %s", code)
	g.JSON(http.StatusOK, gin.H{"reset": code})
}

// @Security AdminToken
// attachTeam godoc
// @Summary Attach a team to a voting code
// @Tags admin
// @Produce json
// @Param code path string true "Voting code"
// @Param teamId path int true "Team ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/admin/codes/{code}/attach-team/{teamId} [post]
func (c *AdminController) attachTeam(g *gin.Context) {
	code := g.Param("code")
	teamIDStr := g.Param("teamId")
	if code == "" || teamIDStr == "" {
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "missing code or teamId"})
		return
	}

	teamID, err := strconv.Atoi(teamIDStr)
	if err != nil {
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid teamId"})
		return
	}

	team, err := c.teamsStorage.Get(g.Request.Context(), teamID)
	if err != nil || team == nil {
		logging.Log.Errorf("ADMIN: team %d not found or lookup failed: %v", teamID, err)
		g.JSON(http.StatusNotFound, models.ErrorResponse{Error: "team not found"})
		return
	}

	voteCode, err := c.codesStorage.Get(g.Request.Context(), code)
	if err != nil {
		logging.Log.Errorf("ADMIN: failed to get code %s: %v", code, err)
		g.JSON(http.StatusNotFound, models.ErrorResponse{Error: "code not found"})
		return
	}

	voteCode.TeamID = &teamID
	if err := c.codesStorage.Overwrite(g.Request.Context(), voteCode); err != nil {
		logging.Log.Errorf("ADMIN: failed to update code %s with team ID: %v", code, err)
		g.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "could not update team ID"})
		return
	}

	logging.Log.Infof("ADMIN: attached team %d to code %s", teamID, code)
	g.JSON(http.StatusOK, gin.H{"message": "team attached", "code": code, "teamId": teamID})
}

// @Security AdminToken
// listCategories godoc
// @Summary List all available voting categories
// @Tags admin
// @Produce json
// @Success 200 {array} map[string]string
// @Router /api/admin/categories [get]
func (c *AdminController) listCategories(g *gin.Context) {
	categories := make([]gin.H, 0, len(models.ValidCategories))
	for k, label := range models.ValidCategories {
		categories = append(categories, gin.H{
			"key":   string(k),
			"label": label,
		})
	}
	logging.Log.Infof("ADMIN: listed %d categories", len(categories))
	g.JSON(http.StatusOK, categories)
}

// @Security AdminToken
// getCodesByCategory godoc
// @Summary List voting codes by category
// @Tags admin
// @Produce json
// @Param category path string true "Voting category"
// @Success 200 {array} models.CodeResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/admin/codes/{category} [get]
func (c *AdminController) getCodesByCategory(g *gin.Context) {
	category := g.Param("category")
	if _, ok := models.ValidCategories[models.VotingCategory(category)]; !ok {
		logging.Log.Warnf("ADMIN: invalid category requested: %s", category)
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid category"})
		return
	}

	codes, err := c.codesStorage.GetAll(g.Request.Context())
	if err != nil {
		logging.Log.Errorf("ADMIN: failed to get codes: %v", err)
		g.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	filtered := make([]*storage.VotingCode, 0)
	for _, code := range codes {
		if code.Category == category {
			filtered = append(filtered, code)
		}
	}

	responseCodes := make([]*models.CodeResponse, 0, len(filtered))
	for _, code := range filtered {
		responseCodes = append(responseCodes, models.TransformVotingCodeToCodeResponse(code))
	}

	logging.Log.Infof("ADMIN: listed %d codes for category: %s", len(filtered), category)
	g.JSON(http.StatusOK, responseCodes)
}

func (c *AdminController) generateShortCode() string {
	code, err := gonanoid.Generate(models.Alphabet, 5)
	if err != nil {
		logging.Log.Errorf("ADMIN: failed to generate code: %v", err)
		return "ERROR"
	}
	return code
}
