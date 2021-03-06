package api

import (
	"context"
	"github.com/labstack/echo/v4"
	"github.com/paysuper/paysuper-management-api/database/model"
	"github.com/paysuper/paysuper-tax-service/proto"
	"net/http"
	"strconv"
)

type taxesRoute struct {
	*Api
}

func (api *Api) initTaxesRoutes() *Api {
	route := &taxesRoute{Api: api}

	api.authUserRouteGroup.GET("/taxes", route.getTaxes)
	api.authUserRouteGroup.POST("/taxes", route.setTax)
	api.authUserRouteGroup.DELETE("/taxes/:id", route.deleteTax)

	return api
}

func (r *taxesRoute) getTaxes(ctx echo.Context) error {
	req := r.bindGetTaxes(ctx)
	res, err := r.taxService.GetRates(context.TODO(), req)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return ctx.JSON(http.StatusOK, res.Rates)
}

func (r *taxesRoute) bindGetTaxes(ctx echo.Context) *tax_service.GetRatesRequest {
	structure := &tax_service.GetRatesRequest{}

	params := ctx.QueryParams()

	if v, ok := params["country"]; ok {
		structure.Country = string(v[0])
	}

	if v, ok := params["city"]; ok {
		structure.City = string(v[0])
	}

	if v, ok := params["state"]; ok {
		structure.State = string(v[0])
	}

	if v, ok := params["zip"]; ok {
		structure.Zip = string(v[0])
	}

	if v, ok := params[requestParameterLimit]; ok {
		if i, err := strconv.ParseInt(v[0], 10, 32); err == nil {
			structure.Limit = int32(i)
		}
	} else {
		structure.Limit = LimitDefault
	}

	if v, ok := params[requestParameterOffset]; ok {
		if i, err := strconv.ParseInt(v[0], 10, 32); err == nil {
			structure.Offset = int32(i)
		}
	} else {
		structure.Offset = OffsetDefault
	}

	return structure
}

func (r *taxesRoute) setTax(ctx echo.Context) error {
	req := &tax_service.TaxRate{}
	err := ctx.Bind(req)

	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Bad request param: "+err.Error())
	}

	res, err := r.taxService.CreateOrUpdate(context.TODO(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return ctx.JSON(http.StatusOK, res)
}

func (r *taxesRoute) deleteTax(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, model.ResponseMessageInvalidRequestData)
	}

	value, err := strconv.Atoi(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, model.ResponseMessageInvalidRequestData)
	}

	res, err := r.taxService.DeleteRateById(context.TODO(), &tax_service.DeleteRateRequest{Id: uint32(value)})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return ctx.JSON(http.StatusOK, res)
}
