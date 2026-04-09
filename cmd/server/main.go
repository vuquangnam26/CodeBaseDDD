package main

import (
	"log"

	"github.com/namcuongq/order-service/internal/bootstrap"
)

// @title Order Service API
// @version 1.0
// @description A production-grade Go backend demonstrating CQRS, Transactional Outbox, and event-driven projection.
// @host localhost:8080
// @BasePath /
func main() {
	if err := bootstrap.Run(); err != nil {
		log.Fatal(err)
	}
}
