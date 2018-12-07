package webhook

import (
	"bytes"
	"github.com/ProtocolONE/p1pay.api/database/dao"
	"github.com/ProtocolONE/p1pay.api/payment_system"
	"github.com/labstack/echo"
	"github.com/oschwald/geoip2-golang"
	"go.uber.org/zap"
	"gopkg.in/go-playground/validator.v9"
	"io/ioutil"
)

type WebHook struct {
	database                dao.Database
	logger                  *zap.SugaredLogger
	validate                *validator.Validate
	geoDbReader             *geoip2.Reader
	pspAccountingCurrencyA3 string
	webHookGroup            *echo.Group
	webHookRawBody          string
	paymentSystemConfig     map[string]interface{}
	paymentSystemSettings   *payment_system.PaymentSystemSetting
	rawBody                 string
}

func InitWebHook(
	database dao.Database,
	logger *zap.SugaredLogger,
	validate *validator.Validate,
	geoDbReader *geoip2.Reader,
	pspAccountingCurrencyA3 string,
	webHookGroup *echo.Group,
	paymentSystemConfig map[string]interface{},
	paymentSystemSettings *payment_system.PaymentSystemSetting,
) *WebHook {
	return &WebHook{
		database:                database,
		logger:                  logger,
		validate:                validate,
		geoDbReader:             geoDbReader,
		pspAccountingCurrencyA3: pspAccountingCurrencyA3,
		webHookGroup:            webHookGroup,
		paymentSystemConfig:     paymentSystemConfig,
		paymentSystemSettings:   paymentSystemSettings,
	}
}

func (wh *WebHook) RawBodyMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(ctx echo.Context) error {
		buf, _ := ioutil.ReadAll(ctx.Request().Body)
		rdr := ioutil.NopCloser(bytes.NewBuffer(buf))

		ctx.Request().Body = rdr
		wh.rawBody = string(buf)

		return next(ctx)
	}
}
