package api

import (
	"context"
	"github.com/labstack/echo/v4"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/grpc"
	"github.com/paysuper/paysuper-payment-link/proto"
	"net/http"
)

type paylinkRoute struct {
	*Api
}

func (api *Api) InitPaylinkRoutes() *Api {
	paylinkApiV1 := paylinkRoute{
		Api: api,
	}

	api.authUserRouteGroup.GET("/paylinks/project/:project_id", paylinkApiV1.getPaylinksList)
	api.authUserRouteGroup.GET("/paylinks/:id", paylinkApiV1.getPaylink)
	api.authUserRouteGroup.GET("/paylinks/:id/stat", paylinkApiV1.getPaylinkStat)
	api.authUserRouteGroup.GET("/paylinks/:id/url", paylinkApiV1.getPaylinkUrl)
	api.authUserRouteGroup.DELETE("/paylinks/:id", paylinkApiV1.deletePaylink)
	api.authUserRouteGroup.POST("/paylinks", paylinkApiV1.createPaylink)
	api.authUserRouteGroup.PUT("/paylinks/:id", paylinkApiV1.updatePaylink)

	return api
}

// @Description Get list of paylinks for project, for authenticated merchant
// @Example GET /admin/api/v1/paylinks/project/21784001599a47e5a69ac28f7af2ec22?offset=0&limit=10
func (r *paylinkRoute) getPaylinksList(ctx echo.Context) error {
	req := &paylink.GetPaylinksRequest{}
	err := (&PaylinksListBinder{}).Bind(req, ctx)

	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, errorQueryParamsIncorrect)
	}

	merchant, err := r.billingService.GetMerchantBy(context.TODO(), &grpc.GetMerchantByRequest{UserId: r.authUser.Id})
	if err != nil || merchant.Item == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, errorUnknown)
	}

	req.MerchantId = merchant.Item.Id

	err = r.validate.Struct(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	res, err := r.paylinkService.GetPaylinks(context.TODO(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(http.StatusOK, res)
}

// @Description Get paylink, for authenticated merchant
// @Example GET /admin/api/v1/paylinks/21784001599a47e5a69ac28f7af2ec22
func (r *paylinkRoute) getPaylink(ctx echo.Context) error {
	id := ctx.Param(requestParameterId)

	req := &paylink.PaylinkRequest{
		Id: id,
	}
	err := r.validate.Struct(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	res, err := r.paylinkService.GetPaylink(context.TODO(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(http.StatusOK, res)
}

// @Description Get stat for paylink
// @Example GET /admin/api/v1/paylinks/21784001599a47e5a69ac28f7af2ec22/stat
func (r *paylinkRoute) getPaylinkStat(ctx echo.Context) error {
	id := ctx.Param(requestParameterId)

	req := &paylink.PaylinkRequest{
		Id: id,
	}
	err := r.validate.Struct(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	res, err := r.paylinkService.GetPaylinkStat(context.TODO(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(http.StatusOK, res)
}

// @Description paylink public url
// @Example GET /admin/api/v1/paylinks/21784001599a47e5a69ac28f7af2ec22/url?utm_source=3wefwe&utm_medium=njytrn&utm_campaign=bdfbh5
func (r *paylinkRoute) getPaylinkUrl(ctx echo.Context) error {
	req := &paylink.GetPaylinkURLRequest{}
	err := (&PaylinksUrlBinder{}).Bind(req, ctx)

	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, errorQueryParamsIncorrect)
	}

	err = r.validate.Struct(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	res, err := r.paylinkService.GetPaylinkURL(context.TODO(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(http.StatusOK, res)
}

// @Description Get paylink, for authenticated merchant
// @Example DELETE /admin/api/v1/paylinks/21784001599a47e5a69ac28f7af2ec22
func (r *paylinkRoute) deletePaylink(ctx echo.Context) error {
	id := ctx.Param(requestParameterId)

	req := &paylink.PaylinkRequest{
		Id: id,
	}
	err := r.validate.Struct(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	_, err = r.paylinkService.DeletePaylink(context.TODO(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return ctx.NoContent(http.StatusNoContent)
}

// @Description Create paylink, for authenticated merchant
// @Example POST /admin/api/v1/paylinks
func (r *paylinkRoute) createPaylink(ctx echo.Context) error {
	return r.createOrUpdatePaylink(ctx, &PaylinksCreateBinder{})
}

// @Description Update paylink, for authenticated merchant
// @Example PUT /admin/api/v1/paylinks/21784001599a47e5a69ac28f7af2ec22
func (r *paylinkRoute) updatePaylink(ctx echo.Context) error {
	return r.createOrUpdatePaylink(ctx, &PaylinksUpdateBinder{})
}

func (r *paylinkRoute) createOrUpdatePaylink(ctx echo.Context, binder echo.Binder) error {
	req := &paylink.CreatePaylinkRequest{}
	err := binder.Bind(req, ctx)

	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	merchant, err := r.billingService.GetMerchantBy(context.TODO(), &grpc.GetMerchantByRequest{UserId: r.authUser.Id})
	if err != nil || merchant.Item == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, errorUnknown)
	}

	req.MerchantId = merchant.Item.Id

	err = r.validate.Struct(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	res, err := r.paylinkService.CreateOrUpdatePaylink(context.TODO(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(http.StatusOK, res)
}
