package api

import (
	"github.com/labstack/echo"
	"net/http"
	"strconv"
)

const (
	defaultLimit  = 100
	defaultOffset = 0
)

func (api *Api) LimitOffsetMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		if ctx.Request().Method != http.MethodGet {
			return next(ctx)
		}

		limit, err := strconv.Atoi(ctx.QueryParam("limit"))

		if err != nil {
			limit = defaultLimit
		}

		offset, err := strconv.Atoi(ctx.QueryParam("offset"))

		if err != nil {
			offset = defaultOffset
		}

		api.limit = limit
		api.offset = offset

		return next(ctx)
	}
}