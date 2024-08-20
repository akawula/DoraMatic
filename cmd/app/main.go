package main

import (
	"log/slog"
	"os"

	"github.com/akawula/DoraMatic/handler"
	"github.com/akawula/DoraMatic/store"
	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
)

func debug() slog.Level {
	level := slog.LevelInfo
	if os.Getenv("DEBUG") == "1" {
		level = slog.LevelDebug
	}
	return level
}

func logger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: debug(),
	}))
}

func setupHome(app *echo.Echo, db store.Store, logger *slog.Logger) {
	homeHandler := handler.HomeHandler{Logger: logger, DB: db}
	app.GET("/", homeHandler.Show)
}

func setupRepositories(app *echo.Echo, db store.Store, logger *slog.Logger) {
	repositoryHandler := handler.RepositoryHandler{Logger: logger, DB: db}
	app.GET("/repository/list", repositoryHandler.Show)
	app.GET("/repository/list/:page", repositoryHandler.List)
	app.GET("/repository/list/refresh", repositoryHandler.Refresh)
}

func setupHandlers(app *echo.Echo, db store.Store, l *slog.Logger) {
	setupHome(app, db, l)
	setupRepositories(app, db, l)
}

func main() {
	l := logger()
	app := echo.New()
	app.HideBanner = true
	db := store.NewPostgres(l)
	defer db.Close()

	setupHandlers(app, db, l)
	if err := app.Start(":2137"); err != nil {
		l.Error("There was an error while starting app", "error", err)
	}
}
