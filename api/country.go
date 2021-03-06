package api

import (
	"github.com/labstack/echo/v4"
	"github.com/paysuper/paysuper-management-api/manager"
	"net/http"
	"strconv"
)

type CountryApiV1 struct {
	*Api
	countryManager *manager.CountryManager
}

func (api *Api) InitCountryRoutes() *Api {
	cApiV1 := CountryApiV1{
		Api:            api,
		countryManager: manager.InitCountryManager(api.database, api.logger),
	}

	api.Http.GET("/api/v1/country", cApiV1.get)
	api.Http.GET("/api/v1/country/:id", cApiV1.getById)

	return api
}

// @Summary Get list of countries
// @Description Get full list of currencies or get list of currencies filtered by name
// @Tags Country
// @Accept json
// @Produce json
// @Param name query string false "name or symbolic ISO 3166-1 code of country"
// @Success 200 {array} model.Country "OK"
// @Failure 500 {object} model.Error "Some unknown error"
// @Router /api/v1/country [get]
func (cApiV1 *CountryApiV1) get(ctx echo.Context) error {
	name := ctx.QueryParam("name")

	if name != "" {
		return ctx.JSON(http.StatusOK, cApiV1.countryManager.FindByName(name))
	}

	return ctx.JSON(http.StatusOK, cApiV1.countryManager.FindAll(cApiV1.limit, cApiV1.offset))
}

// @Summary Get country by numeric ISO 3166-1 code
// @Description Get country object by numeric ISO 3166-1 code
// @Tags Country
// @Accept json
// @Produce json
// @Param id path int true "numeric ISO 3166-1 country code"
// @Success 200 {object} model.Country "OK"
// @Failure 400 {object} model.Error "Invalid request data"
// @Failure 404 {object} model.Error "Not found"
// @Failure 500 {object} model.Error "Some unknown error"
// @Router /api/v1/country/{id} [get]
func (cApiV1 *CountryApiV1) getById(ctx echo.Context) error {
	codeInt, err := strconv.Atoi(ctx.Param("id"))

	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Incorrect currency identifier")
	}

	c := cApiV1.countryManager.FindByCodeInt(codeInt)

	if c == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Currency not found")
	}

	return ctx.JSON(http.StatusOK, c)
}
