package api

import (
	"context"
	"fmt"
	"github.com/alex-pricope/simple-voting-system/api/controllers"
	"github.com/alex-pricope/simple-voting-system/api/transport"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/alex-pricope/simple-voting-system/storage"
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

func NewServer(c *Config) *Server {
	return &Server{
		config: c,
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

	codesStorage := &storage.DynamoVotingCodesStorage{
		Client:    dynamoClient,
		TableName: s.config.TableNameCodes,
	}

	// Register any controller here
	votingController := controllers.NewVotingController(codesStorage)
	votingController.RegisterRoutes(r)
	adminController := controllers.NewAdminController(codesStorage)
	adminController.RegisterRoutes(r)

	if os.Getenv("APP_ENV") == "local" {
		startLocal(r, s.config.Port)
	} else {
		startLambda(r)
	}
}

// StartLambda sets up for AWS Lambda
func startLambda(router *transport.Router) {
	adapter := ginadapter.New(router.Engine)
	lambda.Start(adapter.Proxy)
}

// StartLocal starts a normal HTTP server on port 8080
func startLocal(router *transport.Router, port int) {
	logging.Log.Info(fmt.Sprintf("Starting server on http://localhost:%d", port))

	if err := router.Engine.Run(fmt.Sprintf(":%d", port)); err != nil {
		logging.Log.Fatalf("Failed to run server: %v", err)
	}
}
