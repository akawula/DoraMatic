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
}

func setupHandlers(app *echo.Echo, db store.Store, l *slog.Logger) {
	setupHome(app, db, l)
	setupRepositories(app, db, l)
}

func main() {
	l := logger()
	app := echo.New()
	db := store.New(l)
	defer db.Close()

	setupHandlers(app, db, l)

	if err := app.Start(":2137"); err != nil {
		l.Error("There was an error while starting app", "error", err)
	}
}

/*
func updateDatabase() {
	gLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: debug(),
	}))

	jobs := make(chan string, numberOfWorkers)
	wg := sync.WaitGroup{}

	// open database only once
	db, err := sql.Open("sqlite3", "./database.db")
	if err != nil {
		gLogger.Error("There was an error while opening the database", "error", err)
	}
	defer db.Close()

	gLogger.Info("getting the repositires...")
	r, err := getRepositories()
	if err != nil {
		gLogger.Error("failed while fetching repositories", "error", err)
	}
	if err := saveRepositories(r, db); err != nil {
		gLogger.Error("There was an error while saving repositories", "error", err)
	}

	gLogger.Info("Starting processing repositories...")
	for _, repo := range r {
		wg.Add(1)
		jobs <- string(repo.Name)

		rprs := repositories.New(gLogger, string(repo.Owner.Login), string(repo.Name))
		go rprs.Process(db, jobs, &wg)
	}

	wg.Wait()
}

func saveRepositories(repos []repositories.Repository, db *sql.DB) (err error) {
	for _, repo := range repos {
		org, repoName := repo.Owner.Login, repo.Name
		_, err = db.Exec(fmt.Sprintf("INSERT OR IGNORE INTO repositories (org, slug, \"language\") VALUES (\"%s\", \"%s\", \"%s\")", org, repoName, repo.PrimaryLanguage.Name))
		if err != nil {
			return err
		}
	}

	return
}
*/
