package controllers

import (
	"errors"
	"github.com/alex-pricope/simple-voting-system/api/models"
	"github.com/alex-pricope/simple-voting-system/api/transport"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/alex-pricope/simple-voting-system/storage"
	"github.com/gin-gonic/gin"
	"net/http"
	"sort"
	"strconv"
)

type CategoryMetaController struct {
	storage storage.VotingCategoryStorage
}

func NewCategoryMetaController(s storage.VotingCategoryStorage) *CategoryMetaController {
	return &CategoryMetaController{storage: s}
}

func (c *CategoryMetaController) RegisterRoutes(engine *gin.Engine) {
	group := engine.Group("/api/meta/categories")

	group.GET("", c.getAll)
	group.GET("/:id", transport.AdminAuthMiddleware(), c.get)
	group.POST("", transport.AdminAuthMiddleware(), c.create)
	group.PUT("/:id", transport.AdminAuthMiddleware(), c.update)
	group.DELETE("/:id", transport.AdminAuthMiddleware(), c.delete)
}

// @Summary Get all voting categories
// @Tags Meta/Categories
// @Produce json
// @Success 200 {array} models.VotingCategoryResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/meta/categories [get]
func (c *CategoryMetaController) getAll(g *gin.Context) {
	categories, err := c.storage.GetAll(g.Request.Context())
	if err != nil {
		logging.Log.Errorf("META: failed to get all categories: %v", err)
		g.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Sort this so it shows the same for everyone
	sort.SliceStable(categories, func(i, j int) bool {
		return categories[i].ID < categories[j].ID
	})

	responses := make([]models.VotingCategoryResponse, 0, len(categories))
	for _, cat := range categories {
		responses = append(responses, models.TransformVotingCategoryFromStorage(cat))
	}
	g.JSON(http.StatusOK, responses)
}

// @Summary Get a voting category by ID
// @Tags Meta/Categories
// @Produce json
// @Param id path int true "Category ID"
// @Success 200 {object} models.VotingCategoryResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/meta/categories/{id} [get]
func (c *CategoryMetaController) get(g *gin.Context) {
	idStr := g.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid category id"})
		return
	}
	category, err := c.storage.Get(g.Request.Context(), id)
	if err != nil {
		logging.Log.Errorf("META: failed to get category: %v", err)
		g.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}
	if category == nil {
		g.JSON(http.StatusNotFound, models.ErrorResponse{Error: "category not found"})
		return
	}
	g.JSON(http.StatusOK, models.TransformVotingCategoryFromStorage(category))
}

// @Security AdminToken
// @Summary Create a new voting category
// @Tags Meta/Categories
// @Accept json
// @Produce json
// @Param category body models.VotingCategoryCreateRequest true "VotingCategory object"
// @Success 200 {object} models.VotingCategoryResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/meta/categories [post]
func (c *CategoryMetaController) create(g *gin.Context) {
	var req models.VotingCategoryCreateRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		logging.Log.Errorf("META: invalid create category request: %v", err)
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request"})
		return
	}

	if req.Name == "" {
		logging.Log.Errorf("META: invalid create category request: %v", req)
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request empty name"})
		return
	}
	//TODO: Weight is not checked
	
	category := &storage.VotingCategory{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Weight:      req.Weight,
	}

	if err := c.storage.Create(g.Request.Context(), category); err != nil {
		if errors.Is(err, storage.ErrItemWithIDAlreadyExists) {
			logging.Log.Warnf("META: category with ID %d already exists", req.ID)
			g.JSON(http.StatusConflict, models.ErrorResponse{Error: "category with ID already exists"})
			return
		}

		logging.Log.Errorf("META: failed to create category: %v", err)
		g.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}
	g.JSON(http.StatusOK, models.TransformVotingCategoryFromStorage(category))
}

// @Security AdminToken
// @Summary Update an existing voting category
// @Tags Meta/Categories
// @Accept json
// @Produce json
// @Param id path int true "Category ID"
// @Param category body models.VotingCategoryUpdateRequest true "VotingCategory object"
// @Success 200 {object} models.VotingCategoryResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/meta/categories/{id} [put]
func (c *CategoryMetaController) update(g *gin.Context) {
	idStr := g.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid category id"})
		return
	}

	var req models.VotingCategoryUpdateRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		logging.Log.Errorf("META: invalid update category request: %v", err)
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request"})
		return
	}

	if req.Name == "" {
		logging.Log.Errorf("META: invalid update category request: %v", req)
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request empty name"})
		return
	}

	category := &storage.VotingCategory{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Weight:      req.Weight,
	}

	if err := c.storage.Update(g.Request.Context(), category); err != nil {
		logging.Log.Errorf("META: failed to update category: %v", err)
		g.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}
	g.JSON(http.StatusOK, models.TransformVotingCategoryFromStorage(category))
}

// @Security AdminToken
// @Summary Delete a voting category
// @Tags Meta/Categories
// @Produce json
// @Param id path int true "Category ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/meta/categories/{id} [delete]
func (c *CategoryMetaController) delete(g *gin.Context) {
	idStr := g.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		g.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid category id"})
		return
	}
	if err := c.storage.Delete(g.Request.Context(), id); err != nil {
		logging.Log.Errorf("META: failed to delete category: %v", err)
		g.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}
	g.JSON(http.StatusOK, gin.H{"message": "category deleted"})
}
