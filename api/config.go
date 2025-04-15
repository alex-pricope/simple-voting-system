package api

import (
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/spf13/viper"
	"sync"
)

type Config struct {
	StorageConfig
	ServerConfig
}

type StorageConfig struct {
	TableNameCodes            string
	TableNameVotes            string
	TableNameTeams            string
	TableNameVotingCategories string
}

type ServerConfig struct {
	Port int
}

var settingsOnce sync.Once

func ReadConfig() *Config {

	var conf = &Config{
		StorageConfig: StorageConfig{
			TableNameCodes:            viper.GetString("storage.TableNameCodes"),
			TableNameVotes:            viper.GetString("storage.TableNameVotes"),
			TableNameTeams:            viper.GetString("storage.TableNameTeams"),
			TableNameVotingCategories: viper.GetString("storage.TableNameVotingCategories"),
		},
		ServerConfig: ServerConfig{
			Port: viper.GetInt("server.port"),
		},
	}

	settingsOnce.Do(func() {
		logging.Log.Print("Reading settings!")
	})

	return conf
}

func getString(name string) string {
	if viper.IsSet(name) {
		v := viper.GetString(name)
		logging.Log.Printf("found '%s' in viper", name)
		return v
	}
	logging.Log.Fatalf("required environment variable '%s' is missing", name)
	return ""
}

func getInt(name string) int {
	if viper.IsSet(name) {
		v := viper.GetInt(name)
		logging.Log.Printf("found '%s' in viper", name)
		return v
	}
	logging.Log.Fatalf("required environment variable '%s' is missing", name)
	return -1
}

func getInt32(name string) int32 {
	if viper.IsSet(name) {
		v := viper.GetInt32(name)
		logging.Log.Printf("found '%s' in viper", name)
		return v
	}
	logging.Log.Fatalf("required environment variable '%s' is missing", name)
	return -1
}

func getBool(name string) bool {
	if viper.IsSet(name) {
		v := viper.GetBool(name)
		logging.Log.Printf("found '%s' in viper", name)
		return v
	}
	logging.Log.Fatalf("required environment variable '%s' is missing", name)
	return false
}

func getIntOrDefault(name string, def int) int {
	if viper.IsSet(name) {
		v := viper.GetInt(name)
		logging.Log.Printf("found '%s' in viper", name)
		return v
	}
	logging.Log.Printf("could not find '%s' in viper! Returning default", name)
	return def
}

func getBoolOrDefault(name string, def bool) bool {
	if viper.IsSet(name) {
		v := viper.GetBool(name)
		logging.Log.Printf("found '%s' in viper", name)
		return v
	}
	logging.Log.Printf("could not find '%s' in viper! Returning default", name)
	return def
}

func getStringOrDefault(name string, def string) string {
	if viper.IsSet(name) {
		v := viper.GetString(name)
		logging.Log.Printf("found '%s' in viper", name)
		return v
	}
	logging.Log.Printf("could not find '%s' in viper! Returning default", name)
	return def
}
