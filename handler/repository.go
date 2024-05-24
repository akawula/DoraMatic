package handler

import (
	"log/slog"
	"strconv"

	"github.com/akawula/DoraMatic/store"
	Repository "github.com/akawula/DoraMatic/view/repository"
	"github.com/labstack/echo/v4"
)

type RepositoryHandler struct {
	Logger *slog.Logger
	DB     store.Store
}

func (h RepositoryHandler) Show(c echo.Context) error {
	repos, err := h.DB.GetRepos(1)
	if err != nil {
		h.Logger.Info("There was an error while getting Repos", "error", err)
		return err
	}
	return render(c, Repository.Show(repos))
}

func (h RepositoryHandler) List(c echo.Context) error {
	page, err := strconv.Atoi(c.Param("page"))
	if err != nil {
		page = 1 // screw errors, it needs to be an int (period)
	}
	repos, err := h.DB.GetRepos(page)
	if err != nil {
		h.Logger.Info("There was an error while getting Repos", "error", err)
		return err
	}

	return render(c, Repository.List(page, repos))
}
