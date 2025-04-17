package controllers

import (
	"errors"
	"fmt"
	"github.com/alex-pricope/simple-voting-system/api/models"
	"github.com/alex-pricope/simple-voting-system/api/transport"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/alex-pricope/simple-voting-system/storage"
	"github.com/gin-gonic/gin"
	"net/http"
)

type VotingController struct {
	codesStorage storage.Storage
}

func NewVotingController(s storage.Storage) *VotingController {
	return &VotingController{
		codesStorage: s,
	}
}

func (c *VotingController) RegisterRoutes(r *transport.Router) {
	r.GET("/verify", c.validateVotingCode)
}

// validateVotingCode godoc
// @Summary Validate a voting code
// @Description Checks if a voting code exists and returns its category and usage status
// @Tags voting
// @Accept json
// @Produce json
// @Param code query string true "Voting Code"
// @Success 200 {object} models.CodeValidationResponse
// @Failure 400 {object} models.ErrorResponse "Missing code from request"
// @Failure 404 {object} models.ErrorResponse "Code not found in storage"
// @Failure 500 {object} models.ErrorResponse "Unexpected internal error"
// @Router /verify [get]
func (c *VotingController) validateVotingCode(g *gin.Context) {
	//validate request
	code := g.Query("code")
	if code == "" {
		g.JSON(http.StatusBadRequest, &models.ErrorResponse{Error: "code is required"})
		return
	}

	//get from DynamoDB
	votingCode, err := c.codesStorage.Get(g.Request.Context(), code)
	if err != nil {
		if errors.Is(err, storage.ErrCodeNotFound) {
			logging.Log.Errorf("code not found in storage: %s", code)
			g.JSON(http.StatusNotFound, &models.ErrorResponse{Error: fmt.Sprintf("code not found in storage: %s", code)})
			return
		}

		logging.Log.Errorf("error trying to get code from storage: %v", err)
		g.JSON(http.StatusInternalServerError, &models.ErrorResponse{Error: fmt.Sprintf("error trying to get code from storage: %v", err)})
		return
	}

	//transform and return
	r := models.TransformVotingCodeToValidationResponse(votingCode)
	g.JSON(http.StatusOK, r)
}
