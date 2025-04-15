package controllers

import (
	"errors"
	"fmt"
	"github.com/alex-pricope/simple-voting-system/api/models"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/alex-pricope/simple-voting-system/storage"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

type VotingController struct {
	codesStorage storage.VotingCodeStorage
	votesStorage storage.VoteStorage
}

func NewVotingController(codeStorage storage.VotingCodeStorage, voteStorage storage.VoteStorage) *VotingController {
	return &VotingController{
		codesStorage: codeStorage,
		votesStorage: voteStorage,
	}
}

func (c *VotingController) RegisterRoutes(engine *gin.Engine) {
	group := engine.Group("/api")

	group.GET("/verify/:code", c.validateVotingCode)
	group.POST("/vote", c.registerVote)
}

// registerVote godoc
// @Summary Register a vote
// @Description Accepts a vote submission for a given code
// @Tags voting
// @Accept json
// @Produce json
// @Param vote body models.RegisterVoteRequest true "Vote submission"
// @Success 200 {object} models.RegisterVoteResponse
// @Failure 400 {object} models.ErrorResponse "Invalid vote data"
// @Failure 409 {object} models.ErrorResponse "Code already used or invalid"
// @Failure 500 {object} models.ErrorResponse "Unexpected internal error"
// @Router /api/vote [post]
func (c *VotingController) registerVote(g *gin.Context) {
	var req models.RegisterVoteRequest
	if err := g.ShouldBindJSON(&req); err != nil {
		g.JSON(http.StatusBadRequest, &models.ErrorResponse{Error: "invalid request format"})
		return
	}

	// Check code validity
	votingCode, err := c.codesStorage.Get(g.Request.Context(), req.Code)
	if err != nil || votingCode == nil || votingCode.Used {
		g.JSON(http.StatusConflict, &models.ErrorResponse{Error: "code not valid or already used"})
		return
	}

	// Save all votes
	for _, v := range req.Votes {
		vote := &storage.Vote{
			Code:       req.Code,
			SortKey:    fmt.Sprintf("cat#%d#team#%d", v.CategoryID, v.TeamID),
			CategoryID: v.CategoryID,
			TeamID:     v.TeamID,
			Rating:     v.Rating,
			Timestamp:  time.Now().UTC(),
		}
		if err := c.votesStorage.Create(g.Request.Context(), vote); err != nil {
			logging.Log.Errorf("failed to create vote: %v", err)
			g.JSON(http.StatusInternalServerError, &models.ErrorResponse{Error: "could not save vote"})
			return
		}
	}

	// Mark the code as used
	votingCode.Used = true
	if err := c.codesStorage.Put(g.Request.Context(), votingCode); err != nil {
		logging.Log.Errorf("failed to mark code as used: %v", err)
		g.JSON(http.StatusInternalServerError, &models.ErrorResponse{Error: "could not mark code as used"})
		return
	}

	g.JSON(http.StatusOK, &models.RegisterVoteResponse{Message: "vote registered"})
}

// validateVotingCode godoc
// @Summary Validate a voting code
// @Description Checks if a voting code exists and returns its category and usage status
// @Tags voting
// @Produce json
// @Param code path string true "Voting Code"
// @Success 200 {object} models.CodeValidationResponse
// @Failure 400 {object} models.ErrorResponse "Missing code from request"
// @Failure 404 {object} models.ErrorResponse "Code not found in storage"
// @Failure 500 {object} models.ErrorResponse "Unexpected internal error"
// @Router /api/verify/{code} [get]
func (c *VotingController) validateVotingCode(g *gin.Context) {
	// Validate request
	code := g.Param("code")
	if code == "" {
		g.JSON(http.StatusBadRequest, &models.ErrorResponse{Error: "code is required"})
		return
	}

	// Get from DynamoDB
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

	// Transform and return
	r := models.TransformVotingCodeToValidationResponse(votingCode)
	g.JSON(http.StatusOK, r)
}
