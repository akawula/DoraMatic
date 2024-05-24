package handler

import (
	"log/slog"

	"github.com/akawula/DoraMatic/store"
	Home "github.com/akawula/DoraMatic/view/home"
	"github.com/labstack/echo/v4"
)

type HomeHandler struct {
	Logger *slog.Logger
	DB     store.Store
}

func (h HomeHandler) Show(c echo.Context) error {
	return render(c, Home.Show())
}
