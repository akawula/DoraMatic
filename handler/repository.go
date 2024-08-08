package handler

import (
	"log/slog"
	"strconv"

	"github.com/akawula/DoraMatic/github/repositories"
	"github.com/akawula/DoraMatic/store"
	Repository "github.com/akawula/DoraMatic/view/repository"
	"github.com/labstack/echo/v4"
)

type RepositoryHandler struct {
	Logger *slog.Logger
	DB     store.Store
}

var updateRepoList chan int

func (h RepositoryHandler) Show(c echo.Context) error {
	repos, total, err := h.DB.GetRepos(1, c.QueryParam("search"))
	if err != nil {
		h.Logger.Info("There was an error while getting Repos", "error", err)
		return err
	}
	return render(c, Repository.Show(total, repos))
}

func (h RepositoryHandler) List(c echo.Context) error {
	page, err := strconv.Atoi(c.Param("page"))
	if err != nil {
		page = 1 // screw errors, it needs to be an int (period)
	}
	repos, total, err := h.DB.GetRepos(page, c.QueryParam("search"))
	if err != nil {
		h.Logger.Info("there was an error while getting Repos", "error", err)
		return err
	}

	return render(c, Repository.List(page, total, repos))
}

func (h RepositoryHandler) Refresh(c echo.Context) error {
	r, err := repositories.Get()
	if err != nil {
		h.Logger.Error("there was an error while fetching repos", "error", err)
	}
	if err := h.DB.SaveRepos(r); err != nil {
		h.Logger.Error("there was an error while saving repos", "error", err)
	}

	return render(c, Repository.Button())
}
