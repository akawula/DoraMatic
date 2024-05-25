package handler

import (
	"context"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

func render(c echo.Context, comp templ.Component) error {
	ctx := context.WithValue(c.Request().Context(), "activeUrl", c.Request().URL.String())
	return comp.Render(ctx, c.Response())
}
