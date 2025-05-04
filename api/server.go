package api

import (
	"context"
	"fmt"
	"github.com/alex-pricope/simple-voting-system/api/controllers"
	"github.com/alex-pricope/simple-voting-system/api/transport"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/alex-pricope/simple-voting-system/storage"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	"github.com/gin-gonic/gin"
	"os"
)

type Server struct {
	config *Config
}

func NewServer(config *Config) *Server {
	return &Server{
		config: config,
	}
}

func (s *Server) Start() {
	r := transport.NewRouter(gin.DebugMode)

	// Create storage
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		logging.Log.Errorf("failed to load AWS config: %v", err)
		panic("failed to load AWS config")
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)

	codeStorage := &storage.DynamoVotingCodesStorage{
		Client:    dynamoClient,
		TableName: s.config.TableNameCodes,
	}
	teamStorage := &storage.DynamoTeamStorage{
		Client:    dynamoClient,
		TableName: s.config.TableNameTeams,
	}
	categoryStorage := &storage.DynamoVotingCategoryStorage{
		Client:    dynamoClient,
		TableName: s.config.TableNameVotingCategories,
	}
	votesStorage := &storage.DynamoVoteStorage{
		Client:    dynamoClient,
		TableName: s.config.TableNameVotes,
	}

	//Register controllers
	votingController := controllers.NewVotingController(codeStorage, votesStorage, teamStorage, categoryStorage)
	votingController.RegisterRoutes(r)
	adminController := controllers.NewAdminController(codeStorage, teamStorage)
	adminController.RegisterRoutes(r)
	metaVotingCategoriesController := controllers.NewCategoryMetaController(categoryStorage)
	metaVotingCategoriesController.RegisterRoutes(r)
	metaTeamController := controllers.NewTeamMetaController(teamStorage)
	metaTeamController.RegisterRoutes(r)

	//Do not run lambda helper locally
	if os.Getenv("APP_ENV") == "local" {
		startLocal(r, s.config.Port)
	} else {
		startLambda(r)
	}
}

// StartLambda sets up for AWS Lambda
func startLambda(engine *gin.Engine) {
	ginLambda := ginadapter.NewV2(engine)

	handler := func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		logging.Log.Infof("Lambda handler triggered on path: %s", req.RawPath)
		return ginLambda.ProxyWithContext(ctx, req)
	}

	logging.Log.Info("Starting lambda")
	lambda.Start(handler)
}

// StartLocal starts a normal HTTP server on port 8080
func startLocal(engine *gin.Engine, port int) {
	logging.Log.Info(fmt.Sprintf("Starting server on http://localhost:%d", port))

	if err := engine.Run(fmt.Sprintf(":%d", port)); err != nil {
		logging.Log.Fatalf("Failed to run server: %v", err)
	}
}
