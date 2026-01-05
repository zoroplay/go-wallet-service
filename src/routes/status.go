package routes

import (
	"github.com/labstack/echo/v4"
)

func (a *App) Status(c echo.Context) error {

	return c.JSON(200, "OK")
}
