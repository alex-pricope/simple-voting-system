package controllers

import (
	"github.com/alex-pricope/simple-voting-system/api/models"
	"github.com/alex-pricope/simple-voting-system/api/transport"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/alex-pricope/simple-voting-system/storage"
	"github.com/gin-gonic/gin"
	"github.com/matoous/go-nanoid/v2"
	"net/http"
	"time"
)

type AdminController struct {
	codesStorage storage.VotingCodeStorage
}

func NewAdminController(s storage.VotingCodeStorage) *AdminController {
	return &AdminController{
		codesStorage: s,
	}
}

func (c *AdminController) RegisterRoutes(engine *gin.Engine) {
	group := engine.Group("/api/admin", transport.AdminAuthMiddleware())

	group.GET("/codes", c.listCodes)
	group.POST("/codes", c.createCode)
	group.DELETE("/codes/:code", c.deleteCode)
	group.POST("/codes/reset", c.resetVotes)
	group.GET("/categories", c.listCategories)
	group.GET("/codes/:category", c.getCodesByCategory)
}

// @Security AdminToken
// listCodes godoc
// @Summary List all voting codes
// @Tags admin
// @Produce json
// @Success 200 {array} storage.VotingCode
// @Failure 500 {object} models.ErrorResponse
// @Router /api/admin/codes [get]
func (c *AdminController) listCodes(g *gin.Context) {
	codes, err := c.codesStorage.GetAll(g.Request.Context())
	if err != nil {
		logging.Log.Errorf("ADMIN: failed to list codes: %v", err)
		g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logging.Log.Infof("ADMIN: listed %d codes", len(codes))
	g.JSON(http.StatusOK, codes)
}

// @Security AdminToken
// createCode godoc
// @Summary Create one or more voting codes
// @Tags admin
// @Accept json
// @Produce json
// @Param request body models.CreateCodeRequest true "Create Code Request"
// @Success 200 {array} storage.VotingCode
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/admin/codes [post]
func (c *AdminController) createCode(g *gin.Context) {
	var req models.CreateCodeRequest
	if err := g.ShouldBindJSON(&req); err != nil || req.Category == "" || req.Count < 1 {
		g.JSON(http.StatusBadRequest, gin.H{"error": "invalid request, missing category or count"})
		return
	}

	if _, ok := models.ValidCategories[models.VotingCategory(req.Category)]; !ok {
		g.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
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

	g.JSON(http.StatusOK, codes)
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
		g.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
		return
	}
	if err := c.codesStorage.Delete(g.Request.Context(), code); err != nil {
		logging.Log.Errorf("ADMIN: failed to delete code %s: %v", code, err)
		g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	updated := 0
	for _, code := range codes {
		code.Used = false
		if err := c.codesStorage.Put(g.Request.Context(), code); err != nil {
			logging.Log.Errorf("ADMIN: failed to reset code %s: %v", code.Code, err)
		} else {
			updated++
		}
	}

	logging.Log.Infof("ADMIN: reset %d codes", updated)
	g.JSON(http.StatusOK, gin.H{"message": "All codes reset"})
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
// @Success 200 {array} storage.VotingCode
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/admin/codes/{category} [get]
func (c *AdminController) getCodesByCategory(g *gin.Context) {
	category := g.Param("category")
	if _, ok := models.ValidCategories[models.VotingCategory(category)]; !ok {
		logging.Log.Warnf("ADMIN: invalid category requested: %s", category)
		g.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
		return
	}

	all, err := c.codesStorage.GetAll(g.Request.Context())
	if err != nil {
		logging.Log.Errorf("ADMIN: failed to get codes: %v", err)
		g.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filtered := make([]*storage.VotingCode, 0)
	for _, code := range all {
		if code.Category == category {
			filtered = append(filtered, code)
		}
	}

	logging.Log.Infof("ADMIN: listed %d codes for category: %s", len(filtered), category)
	g.JSON(http.StatusOK, filtered)
}

func (c *AdminController) generateShortCode() string {
	code, err := gonanoid.Generate(models.Alphabet, 5)
	if err != nil {
		logging.Log.Errorf("ADMIN: failed to generate code: %v", err)
		return "ERROR"
	}
	return code
}
