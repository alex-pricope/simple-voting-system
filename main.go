// @title Simple Voting System API
// @version 1.0
// @description Backend API for managing voting and admin functionality

// @securityDefinitions.apikey AdminToken
// @in header
// @name x-admin-token
package main

import (
	_ "github.com/alex-pricope/simple-voting-system/docs"

	"github.com/alex-pricope/simple-voting-system/api"
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/spf13/viper"
)

func main() {
	logging.BoostrapLogger()

	// Load env
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		logging.Log.Errorf("Failed to read config file: %v", err)
		panic("Failed to read config file: " + err.Error())
	}

	// Read config
	config := api.ReadConfig()

	// Start the service (inside the lambda)
	service := api.NewServer(config)
	service.Start()
}
