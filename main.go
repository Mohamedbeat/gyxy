package main

import (
	"log"

	"github.com/mohamedbeat/gyxy/logger"
	"github.com/mohamedbeat/gyxy/proxy"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logg, err := logger.InitLogger()
	if err != nil {
		log.Fatal("Error initializing logger:", err)
	}
	defer logg.Sync()

	// Create and start proxy
	proxy := proxy.New(logg)
	if err := proxy.Start(":8080"); err != nil {
		logg.Fatal("Proxy server failed", zap.Error(err))
	}
}
