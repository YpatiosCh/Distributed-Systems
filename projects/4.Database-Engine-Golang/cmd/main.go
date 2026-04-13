package main

import (
	"log"
	"net/http"

	"github.com/YpatiosCh/Database/api"
	"github.com/YpatiosCh/Database/db"
	"github.com/labstack/echo/v5"
)

func main() {
	// Initialize the database with default options (JSON encoding, default DB name).
	// The db.New constructor accepts functional options (e.g., db.WithDBName, db.WithEncoder)
	// to override defaults; here we rely on the built-in defaults.
	db, err := db.New()
	if err != nil {
		log.Fatal(err)
	}

	// Create the API server, injecting the database dependency.
	server := api.NewServer(db)

	// Configure the Echo HTTP router and register a global error handler
	// that returns all unhandled errors as JSON with a 500 status code.
	e := echo.New()
	e.HTTPErrorHandler = func(c *echo.Context, err error) {
		c.JSON(http.StatusInternalServerError, map[string]any{"error": err.Error()})
	}

	// Route definitions:
	e.POST("/api/:collname", server.HandlePostInsert)
	e.GET("/api/:collname", server.HandleGetQuery)

	// Start server
	log.Fatal(e.Start(":8080"))
}
