package controllers

import (
	"errors"
	"fmt"
	"github.com/alex-pricope/simple-voting-system/api/models"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/alex-pricope/simple-voting-system/storage"
	"github.com/gin-gonic/gin"
	"net/http"
	"sort"
	"strings"
	"time"
)

type VotingController struct {
	codesStorage      storage.VotingCodeStorage
	votesStorage      storage.VoteStorage
	teamsStorage      storage.TeamStorage
	categoriesStorage storage.VotingCategoryStorage
}

func NewVotingController(codeStorage storage.VotingCodeStorage, voteStorage storage.VoteStorage, teamStorage storage.TeamStorage, categoriesStorage storage.VotingCategoryStorage) *VotingController {
	return &VotingController{
		codesStorage:      codeStorage,
		votesStorage:      voteStorage,
		teamsStorage:      teamStorage,
		categoriesStorage: categoriesStorage,
	}
}

func (c *VotingController) RegisterRoutes(engine *gin.Engine) {
	group := engine.Group("/api")

	group.GET("/verify/:code", c.validateVotingCode)
	group.POST("/vote", c.registerVote)
	group.GET("/vote/:code", c.getVotesByCode)
	group.GET("/vote/results", c.computeVoteResults)
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
		logging.Log.Infof("Writing vote PK: %s, SK: %s, R: %d", vote.Code, vote.SortKey, vote.Rating)
		if err := c.votesStorage.Create(g.Request.Context(), vote); err != nil {
			logging.Log.Errorf("Failed to create vote PK: %s, SK: %s, R: %d,  %v",
				vote.Code, vote.SortKey, vote.Rating, err)
			if strings.Contains(err.Error(), "ConditionalCheckFailedException") {
				g.JSON(http.StatusConflict, &models.ErrorResponse{Error: fmt.Sprintf("vote already exists or was submitted before PK: %s, SK: %s, R: %d",
					vote.Code, vote.SortKey, vote.Rating)})
			} else {
				g.JSON(http.StatusInternalServerError, &models.ErrorResponse{Error: fmt.Sprintf("could not save vote PK: %s, SK: %s, R: %d", vote.Code, vote.SortKey, vote.Rating)})
			}
			return
		}
	}

	// Mark the code as used
	votingCode.Used = true
	if err := c.codesStorage.MarkUsed(g.Request.Context(), votingCode.Code); err != nil {
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

// getVotesByCode godoc
// @Summary Get votes by code
// @Description Retrieves all votes for a specific code with team and category info
// @Tags voting
// @Produce json
// @Param code path string true "Voting Code"
// @Success 200 {object} models.GetVoteResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/vote/{code} [get]
func (c *VotingController) getVotesByCode(g *gin.Context) {
	code := g.Param("code")
	if code == "" {
		g.JSON(http.StatusBadRequest, &models.ErrorResponse{Error: "code is required"})
		return
	}

	votes, err := c.votesStorage.GetByCode(g.Request.Context(), code)
	if err != nil {
		logging.Log.Errorf("failed to retrieve votes for code %s: %v", code, err)
		g.JSON(http.StatusInternalServerError, &models.ErrorResponse{Error: "could not retrieve votes"})
		return
	}
	if len(votes) == 0 {
		g.JSON(http.StatusNotFound, &models.ErrorResponse{Error: "no votes found for the given code"})
		return
	}

	categories, err := c.categoriesStorage.GetAll(g.Request.Context())
	if err != nil {
		logging.Log.Errorf("failed to load categories: %v", err)
		g.JSON(http.StatusInternalServerError, &models.ErrorResponse{Error: "could not load categories"})
		return
	}

	teams, err := c.teamsStorage.GetAll(g.Request.Context())
	if err != nil {
		logging.Log.Errorf("failed to load teams: %v", err)
		g.JSON(http.StatusInternalServerError, &models.ErrorResponse{Error: "could not load teams"})
		return
	}

	categoryMap := make(map[int]string)
	for _, c := range categories {
		categoryMap[c.ID] = c.Name
	}

	teamMap := make(map[int]string)
	for _, t := range teams {
		teamMap[t.ID] = t.Name
	}

	response := models.GetVoteResponse{
		Code:  code,
		Votes: make([]models.GetVoteEntry, 0, len(votes)),
	}

	for _, v := range votes {
		response.Votes = append(response.Votes, models.GetVoteEntry{
			VoteEntry: models.VoteEntry{
				CategoryID: v.CategoryID,
				TeamID:     v.TeamID,
				Rating:     v.Rating,
			},
			Team:     teamMap[v.TeamID],
			Category: categoryMap[v.CategoryID],
		})
	}

	g.JSON(http.StatusOK, response)
}

// computeVoteResults godoc
// @Summary Compute voting results
// @Description Aggregates votes per team and category, applying category and voter weights
// @Tags voting
// @Produce json
// @Success 200 {object} models.VoteResultsResponse
// @Failure 500 {object} models.ErrorResponse "Unexpected internal error"
// @Router /api/vote/results [get]
func (c *VotingController) computeVoteResults(g *gin.Context) {
	ctx := g.Request.Context()

	// Load all votes from the DB
	allVotes, err := c.votesStorage.GetAll(ctx)
	if err != nil {
		logging.Log.Errorf("failed to retrieve all votes: %v", err)
		g.JSON(http.StatusInternalServerError, &models.ErrorResponse{Error: "could not load votes"})
		return
	}

	// Load all voting categories so we can map the IDs
	votingCategories, err := c.categoriesStorage.GetAll(ctx)
	if err != nil {
		logging.Log.Errorf("failed to load categories: %v", err)
		g.JSON(http.StatusInternalServerError, &models.ErrorResponse{Error: "could not load categories"})
		return
	}

	// Load all teams so we can map the teamID to name
	allTeams, err := c.teamsStorage.GetAll(ctx)
	if err != nil {
		logging.Log.Errorf("failed to load teams: %v", err)
		g.JSON(http.StatusInternalServerError, &models.ErrorResponse{Error: "could not load teams"})
		return
	}

	// Load all the unique codes
	allUniqueCodes, err := c.codesStorage.GetAll(ctx)
	if err != nil {
		logging.Log.Errorf("failed to load voting codes (defined): %v", err)
		g.JSON(http.StatusInternalServerError, &models.ErrorResponse{Error: "could not load voting codes"})
		return
	}

	// Count how many codes have been used
	usedCodesCount := 0
	for _, code := range allUniqueCodes {
		if code.Used {
			usedCodesCount++
		}
	}

	results := calculateVoteResults(allVotes, allUniqueCodes, votingCategories, allTeams)
	g.JSON(http.StatusOK, models.VoteResultsResponse{
		Results:    results,
		TotalVotes: len(allVotes),
		UsedCodes:  usedCodesCount})
}

func calculateVoteResults(
	allVotes []*storage.Vote, allCodes []*storage.VotingCode,
	categories []*storage.VotingCategory, teams []*storage.Team,
) []models.VoteResult {
	uniqueCodesWithCategoryMap := make(map[string]string)
	for _, c := range allCodes {
		uniqueCodesWithCategoryMap[c.Code] = c.Category
	}

	categoryMap := make(map[int]storage.VotingCategory)
	for _, c := range categories {
		categoryMap[c.ID] = *c
	}

	teamMap := make(map[int]storage.Team)
	for _, t := range teams {
		teamMap[t.ID] = *t
	}

	type entry struct {
		sum   float64
		count int
	}
	// scoreMap holds the aggregated and weighted scores for each team and category.
	// Structure: map[teamID]map[categoryID]*entry where 'entry' contains the sum of weighted scores and the count of votes.
	scoreMap := make(map[int]map[int]*entry)

	// Parse the votes
	for _, v := range allVotes {
		codeCategory := uniqueCodesWithCategoryMap[v.Code]
		votingCategory := categoryMap[v.CategoryID]
		teamID := v.TeamID

		// Create both weights
		voterWeight, categoryWeight := models.CodeCategoryWeights[models.VotingCategory(codeCategory)], votingCategory.Weight

		if voterWeight == 0 {
			logging.Log.Warnf("Unknown voter category %q; weight is 0", codeCategory)
		}

		// The computed rating for each vote line
		// (user_rating) × (importance of who votes) × (importance of what they're voting on)
		weighted := float64(v.Rating) * voterWeight * categoryWeight

		// Iterate and sum the individual weighted score
		if _, ok := scoreMap[teamID]; !ok {
			scoreMap[teamID] = make(map[int]*entry)
		}
		if _, ok := scoreMap[teamID][v.CategoryID]; !ok {
			scoreMap[teamID][v.CategoryID] = &entry{}
		}
		scoreMap[teamID][v.CategoryID].sum += weighted
		scoreMap[teamID][v.CategoryID].count++
	}

	// Note: Each voter is required (via frontend enforcement) to vote for every team in every category.
	// This ensures that all codes contribute equally in terms of vote quantity, and only the category weight
	// and the code's voter weight (e.g., grand_jury = 0.5) influence the outcome.
	// This also means the average calculation per category is valid and fair.

	var results []models.VoteResult

	// Iterate through each category's aggregated score for the current team,
	// compute the average total score, and build the result structure.
	for teamID, catScores := range scoreMap {
		var total float64
		var categories []models.CategoryScore

		for catID, entry := range catScores {
			name := categoryMap[catID].Name

			// Compute the average weighted score for this category and team.
			// Since every voter votes for every team in each category, this average
			// fairly represents their influence.
			score := entry.sum / float64(entry.count)
			categories = append(categories, models.CategoryScore{
				CategoryID:   catID,
				CategoryName: name,
				Score:        score,
			})
			total += score
		}

		// Sort the categories by score
		sort.Slice(categories, func(i, j int) bool {
			return categories[i].Score > categories[j].Score
		})

		results = append(results, models.VoteResult{
			TeamID:      teamID,
			TeamName:    teamMap[teamID].Name,
			TotalScore:  total,
			Categories:  categories,
			TeamMembers: teamMap[teamID].Members,
		})
	}

	// Sort the teams by final total score
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalScore > results[j].TotalScore
	})
	return results
}
