package main

import (
	dependencyInjection "HM/dependency_injection"
	"time"
)

func main() {
	//set timezone
	viper.SetDefualt("SERVER_TIMEZONE", "Asia/Kolkata")
	loc, _ := time.LoadLocation(viper.GetString("SERVER_TIMEZONE"))
	time.Local = loc

	dependencyInjection.LoadDependencies()
}
