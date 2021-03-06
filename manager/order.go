package manager

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/ProtocolONE/geoip-service/pkg/proto"
	"github.com/ProtocolONE/rabbitmq/pkg"
	"github.com/dgrijalva/jwt-go"
	"github.com/globalsign/mgo/bson"
	"github.com/golang/protobuf/ptypes"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-management-api/database/dao"
	"github.com/paysuper/paysuper-management-api/database/model"
	"github.com/paysuper/paysuper-management-api/payment_system"
	"github.com/paysuper/paysuper-management-api/payment_system/entity"
	"github.com/paysuper/paysuper-management-api/utils"
	"github.com/paysuper/paysuper-recurring-repository/pkg/constant"
	"github.com/paysuper/paysuper-recurring-repository/pkg/proto/repository"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	orderErrorProjectNotFound                          = "project with specified identifier not found"
	orderErrorProjectInactive                          = "project with specified identifier is inactive"
	orderErrorPaymentMethodNotAllowed                  = "payment method not specified for project"
	orderErrorPaymentMethodNotFound                    = "payment method with specified id not found"
	orderErrorPaymentMethodInactive                    = "payment method with specified is inactive"
	orderErrorPaymentSystemNotFound                    = "payment system for specified payment method not found"
	orderErrorPaymentSystemInactive                    = "payment system for specified payment method is inactive"
	orderErrorPayerRegionUnknown                       = "payer region can't be found"
	orderErrorFixedPackageForRegionNotFound            = "project not have fixed packages for payer region"
	orderErrorFixedPackageNotFound                     = "project not have fixed package with specified amount or currency"
	orderErrorProjectOrderIdIsDuplicate                = "request with specified project order identifier processed early"
	orderErrorDynamicNotifyUrlsNotAllowed              = "dynamic verify url or notify url not allowed for project"
	orderErrorDynamicRedirectUrlsNotAllowed            = "dynamic payer redirect urls not allowed for project"
	orderErrorCurrencyNotFound                         = "currency received from request not found"
	orderErrorAmountLowerThanMinAllowed                = "order amount is lower than min allowed payment amount for project"
	orderErrorAmountGreaterThanMaxAllowed              = "order amount is greater than max allowed payment amount for project"
	orderErrorAmountLowerThanMinAllowedPaymentMethod   = "order amount is lower than min allowed payment amount for payment method"
	orderErrorAmountGreaterThanMaxAllowedPaymentMethod = "order amount is greater than max allowed payment amount for payment method"
	orderErrorCanNotCreate                             = "order can't create. try request later"
	orderErrorSignatureInvalid                         = "order request signature is invalid"
	orderErrorNotFound                                 = "order with specified identifier not found"
	orderErrorOrderAlreadyComplete                     = "order with specified identifier payed early"
	orderErrorOrderPaymentMethodIncomeCurrencyNotFound = "unknown currency received from payment system"
	orderErrorOrderPSPAccountingCurrencyNotFound       = "unknown PSP accounting currency"
	orderErrorOrderDeclined                            = "payment system decline order with specified identifier early"
	orderErrorOrderCanceled                            = "payment system cancel order with specified identifier early"

	orderErrorCreatePaymentRequiredFieldIdNotFound            = "required field with order identifier not found"
	orderErrorCreatePaymentRequiredFieldPaymentMethodNotFound = "required field with payment method identifier not found"
	orderErrorCreatePaymentRequiredFieldEmailNotFound         = "required field \"email\" not found"

	orderSignatureElementsGlue = "|"

	orderDefaultDescription = "Payment by order # %s"
)

type OrderManager struct {
	*Manager

	projectManager         *ProjectManager
	paymentSystemManager   *PaymentSystemManager
	paymentMethodManager   *PaymentMethodManager
	currencyRateManager    *CurrencyRateManager
	currencyManager        *CurrencyManager
	pspAccountingCurrency  *model.Currency
	paymentSystemsSettings *payment_system.PaymentSystemSetting
	vatManager             *VatManager
	commissionManager      *CommissionManager
	centrifugoSecret       string

	rep repository.RepositoryService
	geo proto.GeoIpService
	ctx context.Context
	pub *rabbitmq.Broker
}

type check struct {
	order         *model.OrderScalar
	project       *model.Project
	oCurrency     *model.Currency
	paymentMethod *model.PaymentMethod
}

type pmOutcomeData struct {
	amount   float64
	currency *model.Currency
}

type FindAll struct {
	Values   url.Values
	Projects map[bson.ObjectId]string
	Merchant *billing.Merchant
	Limit    int32
	Offset   int32
	SortBy   []string
}

type OrderHttp struct {
	Host   string
	Scheme string
}

func InitOrderManager(
	database dao.Database,
	logger *zap.SugaredLogger,
	publisher *rabbitmq.Broker,
	repository repository.RepositoryService,
	geoService proto.GeoIpService,
) *OrderManager {
	om := &OrderManager{
		Manager:              &Manager{Database: database, Logger: logger},
		projectManager:       InitProjectManager(database, logger, nil),
		paymentSystemManager: InitPaymentSystemManager(database, logger),
		paymentMethodManager: InitPaymentMethodManager(database, logger),
		currencyRateManager:  InitCurrencyRateManager(database, logger),
		currencyManager:      InitCurrencyManager(database, logger),
		vatManager:           InitVatManager(database, logger),
		commissionManager:    InitCommissionManager(database, logger),

		rep: repository,
		geo: geoService,
		ctx: context.TODO(),
		pub: publisher,
	}

	return om
}

func (om *OrderManager) Process(order *model.OrderScalar) (*model.Order, error) {
	var pm *model.PaymentMethod
	var pmOutcomeData *pmOutcomeData
	var gRecord *proto.GeoIpDataResponse
	var err error

	p := om.projectManager.FindProjectById(order.ProjectId)

	if p == nil {
		return nil, errors.New(orderErrorProjectNotFound)
	}

	if p.IsActive == false {
		return nil, errors.New(orderErrorProjectInactive)
	}

	var oCurrency *model.Currency

	if order.Currency != nil {
		oCurrency = om.currencyManager.FindByCodeA3(*order.Currency)

		if oCurrency == nil {
			return nil, errors.New(orderErrorCurrencyNotFound)
		}
	}

	check := &check{
		order: &model.OrderScalar{
			Amount:           order.Amount,
			Currency:         order.Currency,
			Region:           order.Region,
			CreateOrderIp:    order.CreateOrderIp,
			RawRequestParams: order.RawRequestParams,
			Signature:        order.Signature,
			PaymentMethod:    order.PaymentMethod,
		},
		project:   p,
		oCurrency: oCurrency,
	}

	if order.Signature != nil {
		err = om.checkSignature(check)

		if err != nil {
			return nil, err
		}
	}

	if order.PaymentMethod != nil {
		pm, err = om.checkPaymentMethod(check)

		if err != nil {
			return nil, err
		}
	}

	check.paymentMethod = pm

	if err = om.checkProjectLimits(check); err != nil {
		return nil, err
	}

	if order.PaymentMethod != nil {
		if pmOutcomeData, err = om.checkPaymentMethodLimits(check); err != nil {
			return nil, err
		}
	}

	if order.OrderId != "" {
		if err = om.checkProjectOrderIdUnique(order); err != nil {
			return nil, err
		}
	}

	if (order.UrlVerify != nil || order.UrlNotify != nil) && p.IsAllowDynamicNotifyUrls == false {
		return nil, errors.New(orderErrorDynamicNotifyUrlsNotAllowed)
	}

	if (order.UrlSuccess != nil || order.UrlFail != nil) && p.IsAllowDynamicRedirectUrls == false {
		return nil, errors.New(orderErrorDynamicRedirectUrlsNotAllowed)
	}

	mACAmount, _ := om.currencyRateManager.convert(oCurrency.CodeInt, int(p.Merchant.Banking.Currency.CodeInt), order.Amount)
	pOutAmount, _ := om.currencyRateManager.convert(oCurrency.CodeInt, int(p.CallbackCurrency.CodeInt), order.Amount)

	id := bson.NewObjectId()

	uSubdivision := ""

	if len(gRecord.Subdivisions) > 0 {
		uSubdivision = gRecord.Subdivisions[0].IsoCode
	}

	nOrder := &model.Order{
		Id: id,
		Project: &model.ProjectOrder{
			Id:                p.Id,
			Name:              p.Name,
			UrlSuccess:        p.URLRedirectSuccess,
			UrlFail:           p.URLRedirectFail,
			SendNotifyEmail:   p.SendNotifyEmail,
			NotifyEmails:      p.NotifyEmails,
			SecretKey:         p.SecretKey,
			URLCheckAccount:   p.URLCheckAccount,
			URLProcessPayment: p.URLProcessPayment,
			Merchant:          p.Merchant,
			CallbackProtocol:  p.CallbackProtocol,
		},
		Description:            fmt.Sprintf(orderDefaultDescription, id.Hex()),
		ProjectOrderId:         order.OrderId,
		ProjectAccount:         order.Account,
		ProjectIncomeAmount:    FormatAmount(order.Amount),
		ProjectIncomeCurrency:  oCurrency,
		ProjectOutcomeAmount:   FormatAmount(pOutAmount),
		ProjectOutcomeCurrency: p.CallbackCurrency,
		ProjectParams:          order.Other,
		PayerData: &model.PayerData{
			Ip:            order.CreateOrderIp,
			CountryCodeA2: gRecord.Country.IsoCode,
			CountryName:   &model.Name{EN: gRecord.Country.Names["en"], RU: gRecord.Country.Names["ru"]},
			City:          &model.Name{EN: gRecord.City.Names["en"], RU: gRecord.City.Names["ru"]},
			Subdivision:   uSubdivision,
			Timezone:      gRecord.Location.TimeZone,
			Phone:         order.PayerPhone,
			Email:         order.PayerEmail,
		},
		Status:                             model.OrderStatusNew,
		CreatedAt:                          time.Now(),
		IsJsonRequest:                      order.IsJsonRequest,
		AmountInMerchantAccountingCurrency: FormatAmount(mACAmount),
	}

	if nOrder.PayerData.CountryCodeA2 == "" && order.Region != nil {
		nOrder.PayerData.CountryCodeA2 = *order.Region
	}

	if order.Description != nil {
		nOrder.Description = *order.Description
	}

	if order.PaymentMethod != nil {
		nOrder.PaymentMethod = &model.OrderPaymentMethod{
			Id:            pm.Id,
			Name:          pm.Name,
			Params:        pm.Params,
			PaymentSystem: pm.PaymentSystem,
			GroupAlias:    pm.GroupAlias,
		}

		nOrder.PaymentMethodOutcomeAmount = FormatAmount(pmOutcomeData.amount)
		nOrder.PaymentMethodOutcomeCurrency = pmOutcomeData.currency

		if nOrder, err = om.ProcessOrderCommissions(nOrder); err != nil {
			return nil, err
		}
	}

	if order.UrlSuccess != nil {
		nOrder.UrlSuccess = *order.UrlSuccess
	}

	if order.UrlFail != nil {
		nOrder.UrlFail = *order.UrlFail
	}

	if err = om.Database.Repository(TableOrder).InsertOrder(nOrder); err != nil {
		om.Logger.Errorf("Query from table \"%s\" ended with error: %s", TableOrder, err)

		return nil, errors.New(orderErrorCanNotCreate)
	}

	return nOrder, nil
}

// Post action after process order to form data to json order create response
func (om *OrderManager) JsonOrderCreatePostProcess(o *model.Order, oh *OrderHttp) (*model.JsonOrderCreateResponse, error) {
	pmPrepData := &model.OrderFormRendering{}
	err := errors.New("fuck") //om.GetOrderByIdWithPaymentMethods(o, oh.Host)

	if err != nil {
		return nil, err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": o.Id.Hex(),
		"exp": time.Now().Add(time.Minute * 30).Unix(),
	})
	tokenString, err := token.SignedString([]byte(om.centrifugoSecret))

	if err != nil {
		return nil, err
	}

	jo := &model.JsonOrderCreateResponse{
		Id:                o.Id.Hex(),
		HasVat:            o.Project.Merchant.IsVatEnabled,
		HasUserCommission: o.Project.Merchant.IsCommissionToUserEnabled,
		Project: &model.ProjectJsonOrderResponse{
			Name:       o.Project.Name,
			UrlSuccess: o.Project.UrlSuccess,
			UrlFail:    o.Project.UrlFail,
		},
		PaymentMethods:        pmPrepData.SlicePaymentMethodJsonOrderResponse,
		InlineFormRedirectUrl: fmt.Sprintf(model.OrderInlineFormUrlMask, oh.Scheme, oh.Host, o.Id.Hex()),
		Token:                 tokenString,
	}

	if o.ProjectAccount != "" {
		jo.Account = &o.ProjectAccount
	}

	return jo, nil
}

// Calculate all possible commissions for order, i.e. payment system fee amount, PSP (P1) fee amount,
// commission shifted from project to user and VAT
func (om *OrderManager) ProcessOrderCommissions(o *model.Order) (*model.Order, error) {
	pmOutAmount := o.PaymentMethodOutcomeAmount

	// if merchant enable VAT calculation then we're need to calculate VAT for payer
	if o.Project.Merchant.IsVatEnabled == true {
		vatAmount, err := om.vatManager.CalculateVat(o.PayerData.CountryCodeA2, o.PayerData.Subdivision, o.PaymentMethodOutcomeAmount)

		if err != nil {
			return nil, err
		}

		o.VatAmount = FormatAmount(vatAmount)

		// add VAT amount to payment amount
		pmOutAmount += vatAmount
	}

	// calculate commissions to selected payment method
	commissions, err := om.commissionManager.CalculateCommission(o.Project.Id, o.PaymentMethod.Id, o.PaymentMethodOutcomeAmount)

	if err != nil {
		return nil, err
	}

	var cCom float64

	mAccCur := o.Project.Merchant.Banking.Currency.CodeInt
	pmOutCur := o.PaymentMethodOutcomeCurrency.CodeInt

	totalCommission := commissions.PMCommission + commissions.PspCommission

	// if merchant enable to shift commissions form project to payer then we're need to calculate commissions shifting
	if o.Project.Merchant.IsCommissionToUserEnabled == true {
		// subtract commission to user from project's commission
		totalCommission -= commissions.ToUserCommission

		// add commission to user to payment amount
		pmOutAmount += commissions.ToUserCommission

		o.ToPayerFeeAmount = &model.OrderFee{
			AmountPaymentMethodCurrency: FormatAmount(commissions.ToUserCommission),
		}

		// convert amount of fee shifted to user to accounting currency of merchant
		if cCom, err = om.currencyRateManager.convert(pmOutCur, int(mAccCur), commissions.ToUserCommission); err != nil {
			return nil, err
		}

		o.ToPayerFeeAmount.AmountMerchantCurrency = FormatAmount(cCom)
	}

	o.ProjectFeeAmount = &model.OrderFee{
		AmountPaymentMethodCurrency: FormatAmount(totalCommission),
	}

	// convert amount of fee to project to accounting currency of merchant
	if cCom, err = om.currencyRateManager.convert(pmOutCur, int(mAccCur), totalCommission); err != nil {
		return nil, err
	}

	o.ProjectFeeAmount.AmountMerchantCurrency = FormatAmount(cCom)

	o.PspFeeAmount = &model.OrderFeePsp{
		AmountPaymentMethodCurrency: commissions.PspCommission,
	}

	// convert PSP amount of fee to accounting currency of merchant
	if cCom, err = om.currencyRateManager.convert(pmOutCur, int(mAccCur), commissions.PspCommission); err != nil {
		return nil, err
	}

	o.PspFeeAmount.AmountMerchantCurrency = FormatAmount(cCom)

	// convert PSP amount of fee to accounting currency of PSP
	if cCom, err = om.currencyRateManager.convert(pmOutCur, om.pspAccountingCurrency.CodeInt, commissions.PspCommission); err != nil {
		return nil, err
	}

	o.PspFeeAmount.AmountPspCurrency = FormatAmount(cCom)

	// save information about payment system commission
	o.PaymentSystemFeeAmount = &model.OrderFeePaymentSystem{
		AmountPaymentMethodCurrency: FormatAmount(commissions.PMCommission),
	}

	// convert payment system amount of fee to accounting currency of payment system
	cCom, err = om.currencyRateManager.convert(pmOutCur, o.PaymentMethod.PaymentSystem.AccountingCurrency.CodeInt, commissions.PMCommission)

	if err != nil {
		return nil, err
	}

	o.PaymentSystemFeeAmount.AmountPaymentSystemCurrency = FormatAmount(cCom)

	// convert payment system amount of fee to accounting currency of merchant
	if cCom, err = om.currencyRateManager.convert(pmOutCur, int(mAccCur), commissions.PMCommission); err != nil {
		return nil, err
	}

	o.PaymentSystemFeeAmount.AmountMerchantCurrency = FormatAmount(cCom)
	o.PaymentMethodOutcomeAmount = FormatAmount(pmOutAmount)

	return o, nil
}

func (om *OrderManager) FindById(id string) *model.Order {
	o, err := om.Database.Repository(TableOrder).FindOrderById(bson.ObjectIdHex(id))

	if err != nil {
		om.Logger.Errorf("Query from table \"%s\" ended with error: %s", TableOrder, err)
	}

	return o
}

/*func (om *OrderManager) GetOrderByIdWithPaymentMethods(o *model.Order, host string) (*model.OrderFormRendering, error) {
	projectPms, err := om.rep.FindPaymentMethodsByCurrency(om.ctx, &repository.FindByIntValue{Value: int32(o.ProjectIncomeCurrency.CodeInt)})

	if err != nil {
		return nil, err
	}

	ofr := &model.OrderFormRendering{}
	amount := o.ProjectIncomeAmount

	for _, pm := range projectPms.PaymentMethods {
		pmPrepData := &model.PaymentMethodJsonOrderResponse{
			Id:                       tools.ByteToObjectId(pm.Id).Hex(),
			Name:                     pm.Name,
			Icon:                     fmt.Sprintf(model.OrderInlineFormImagesUrlMask, host, pm.Icon),
			Type:                     pm.Type,
			GroupAlias:               pm.Group,
			AccountRegexp:            pm.AccountRegexp,
			AmountWithoutCommissions: tools.FormatAmount(amount),
			Currency:                 o.ProjectIncomeCurrency.CodeA3,
		}

		// if commission to user enabled for merchant then calculate commissions to user
		// for every allowed payment methods
		if o.Project.Merchant.IsCommissionToUserEnabled == true {
			commissions, err := om.commissionManager.CalculateCommission(o.Project.Id, tools.ByteToObjectId(pm.Id), pmPrepData.AmountWithoutCommissions)

			if err != nil {
				return nil, err
			}

			amount += commissions.ToUserCommission
			pmPrepData.UserCommissionAmount = FormatAmount(commissions.ToUserCommission)
		}

		// if merchant enable VAT calculation then we're calculate VAT for payer
		if o.Project.Merchant.IsVatEnabled == true {
			vat, err := om.vatManager.CalculateVat(o.PayerData.CountryCodeA2, o.PayerData.Subdivision, pmPrepData.AmountWithoutCommissions)

			if err != nil {
				return nil, err
			}

			amount += vat
			pmPrepData.VatAmount = FormatAmount(vat)
		}

		pmPrepData.AmountWithCommissions = FormatAmount(amount)

		if pm.IsBankCard() == true {
			req := &repository.SavedCardRequest{Account: o.ProjectAccount, ProjectId: tools.ObjectIdToByte(o.Project.Id)}

			if rsp, err := om.rep.FindSavedCards(om.ctx, req); err == nil {
				has := len(rsp.SavedCards) > 0
				pmPrepData.HasSavedCards = &has

				if has == true {
					pmPrepData.SavedCards = []*model.SavedCardResponse{}

					for _, v := range rsp.SavedCards {
						d := &model.SavedCardResponse{
							Id:     tools.ByteToObjectId(v.Id).Hex(),
							Pan:    v.MaskedPan,
							Expire: v.Expire,
						}

						pmPrepData.SavedCards = append(pmPrepData.SavedCards, d)
					}
				}
			}
		}

		tOfr := &model.PaymentMethodJsonOrderResponseOrderFormRendering{
			GroupAlias:                     pm.Group,
			PaymentMethodJsonOrderResponse: pmPrepData,
		}

		ofr.SlicePaymentMethodJsonOrderResponse = append(ofr.SlicePaymentMethodJsonOrderResponse, pmPrepData)
		ofr.MapPaymentMethodJsonOrderResponse = append(ofr.MapPaymentMethodJsonOrderResponse, tOfr)
	}

	return ofr, nil
}*/

func (om *OrderManager) getPaymentMethod(order *model.OrderScalar, pms map[string][]*model.ProjectPaymentModes) (*model.ProjectPaymentModes, error) {
	cpms, ok := pms[*order.PaymentMethod]

	if !ok || len(cpms) <= 0 {
		return nil, errors.New(orderErrorPaymentMethodNotAllowed)
	}

	var opm *model.ProjectPaymentModes

	for _, ppm := range cpms {
		if opm == nil || opm.AddedAt.Before(ppm.AddedAt) == true {
			opm = ppm
		}
	}

	return opm, nil
}

func (om *OrderManager) checkProjectOrderIdUnique(order *model.OrderScalar) error {
	if order.OrderId == "" {
		return nil
	}

	o, err := om.Database.Repository(TableOrder).FindOrderByProjectOrderId(order.OrderId)

	if err != nil {
		om.Logger.Errorf("Query from table \"%s\" ended with error: %s", TableOrder, err)
	}

	if o == nil {
		return nil
	}

	return errors.New(orderErrorProjectOrderIdIsDuplicate)
}

func (om *OrderManager) checkProjectLimits(c *check) error {
	var err error
	cAmount := c.order.Amount

	if c.oCurrency != nil && c.oCurrency.CodeA3 != c.project.LimitsCurrency.CodeA3 {
		cAmount, err = om.currencyRateManager.convert(c.oCurrency.CodeInt, int(c.project.LimitsCurrency.CodeInt), c.order.Amount)

		if err != nil {
			return err
		}
	}

	if cAmount < c.project.MinPaymentAmount {
		return errors.New(orderErrorAmountLowerThanMinAllowed)
	}

	if cAmount > c.project.MaxPaymentAmount {
		return errors.New(orderErrorAmountGreaterThanMaxAllowed)
	}

	return nil
}

func (om *OrderManager) checkPaymentMethodLimits(c *check) (*pmOutcomeData, error) {
	var err error
	cAmount := c.order.Amount

	if c.oCurrency != nil && c.oCurrency.CodeA3 != c.paymentMethod.Currency.CodeA3 {
		cAmount, err = om.currencyRateManager.convert(c.oCurrency.CodeInt, c.paymentMethod.Currency.CodeInt, c.order.Amount)

		if err != nil {
			return nil, err
		}
	}

	if cAmount < c.paymentMethod.MinPaymentAmount {
		return nil, errors.New(orderErrorAmountLowerThanMinAllowedPaymentMethod)
	}

	if cAmount > c.paymentMethod.MaxPaymentAmount {
		return nil, errors.New(orderErrorAmountGreaterThanMaxAllowedPaymentMethod)
	}

	pmOutcomeData := &pmOutcomeData{
		amount:   cAmount,
		currency: c.paymentMethod.Currency,
	}

	return pmOutcomeData, nil
}

func (om *OrderManager) checkPaymentMethod(c *check) (*model.PaymentMethod, error) {
	opm, err := om.getPaymentMethod(c.order, c.project.PaymentMethods)

	if err != nil {
		return nil, err
	}

	pm := om.paymentMethodManager.FindById(opm.Id)

	if pm == nil {
		return nil, errors.New(orderErrorPaymentMethodNotFound)
	}

	if pm.IsActive == false {
		return nil, errors.New(orderErrorPaymentMethodInactive)
	}

	ps := pm.PaymentSystem

	if ps == nil {
		return nil, errors.New(orderErrorPaymentSystemNotFound)
	}

	if ps.IsActive == false {
		return nil, errors.New(orderErrorPaymentSystemInactive)
	}

	return pm, nil
}

func (om *OrderManager) getOrderFixedPackage(c *check) (*proto.GeoIpDataResponse, *model.OrderFixedPackage, error) {
	var region string

	if c.order.Region != nil {
		region = *c.order.Region
	}

	data, err := om.geo.GetIpData(om.ctx, &proto.GeoIpDataRequest{IP: c.order.CreateOrderIp})

	if err != nil {
		return nil, nil, errors.New(orderErrorPayerRegionUnknown + " ====> " + err.Error())
	}

	if region == "" {
		region = data.Country.IsoCode
	}

	fps, ok := c.project.FixedPackage[region]

	if !ok || len(fps) <= 0 {
		return nil, nil, errors.New(orderErrorFixedPackageForRegionNotFound)
	}

	var ofp *model.FixedPackage

	for _, fp := range fps {
		if fp.Price != c.order.Amount || (c.order.Currency != nil && fp.Currency.CodeA3 != *c.order.Currency) {
			continue
		}

		ofp = fp
	}

	if ofp == nil {
		return nil, nil, errors.New(orderErrorFixedPackageNotFound)
	}

	orderFp := &model.OrderFixedPackage{
		Id:          ofp.Id,
		Region:      region,
		Name:        ofp.Name,
		CurrencyInt: ofp.CurrencyInt,
		Price:       ofp.Price,
		Currency:    ofp.Currency,
	}

	return data, orderFp, nil
}

func (om *OrderManager) checkSignature(check *check) error {
	keys := make([]string, 0, len(check.order.RawRequestParams))
	gs := make([]string, 0, len(check.order.RawRequestParams))

	for k := range check.order.RawRequestParams {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		value := k + "=" + check.order.RawRequestParams[k]
		gs = append(gs, value)
	}

	h := sha256.New()
	h.Write([]byte(strings.Join(gs, orderSignatureElementsGlue) + orderSignatureElementsGlue + check.project.SecretKey))

	if string(h.Sum(nil)) != *check.order.Signature {
		return errors.New(orderErrorSignatureInvalid)
	}

	return nil
}

func (om *OrderManager) FindAll(params *FindAll) (*model.OrderPaginate, error) {
	var f bson.M
	var pFilter []bson.ObjectId

	for k := range params.Projects {
		pFilter = append(pFilter, k)
	}

	filter := bson.M{"project.id": bson.M{"$in": pFilter}}

	if quickFilter, ok := params.Values[model.OrderFilterFieldQuickFilter]; ok {
		r := bson.RegEx{Pattern: ".*" + quickFilter[0] + ".*", Options: "i"}

		filter["$or"] = []bson.M{
			{"project.name": bson.M{"$regex": r}},
			{"project_account": bson.M{"$regex": r}},
			{"project_order_id": bson.M{"$regex": r, "$exists": true}},
			{"fixed_package.name": bson.M{"$regex": r, "$exists": true}},
			{"payment_method.name": bson.M{"$regex": r, "$exists": true}},
			{"id_string": bson.M{"$regex": r, "$exists": true}},
		}

		f = filter
	} else {
		f = om.ProcessFilters(params.Values, filter)
	}

	co, err := om.Database.Repository(TableOrder).GetOrdersCountByConditions(f)

	if err != nil {
		om.Logger.Errorf("Query from table \"%s\" ended with error: %s", TableOrder, err)
	}

	o, err := om.Database.Repository(TableOrder).FindAllOrders(f, params.SortBy, params.Limit, params.Offset)

	if err != nil {
		om.Logger.Errorf("Query from table \"%s\" ended with error: %s", TableOrder, err)
	}

	var ot []*model.OrderSimple

	if o != nil && len(o) > 0 {
		ot, err = om.transformOrders(o, params)

		if err != nil {
			return nil, err
		}
	}

	return &model.OrderPaginate{Count: co, Items: ot}, nil
}

func (om *OrderManager) transformOrders(orders []*model.Order, params *FindAll) ([]*model.OrderSimple, error) {
	var tOrders []*model.OrderSimple

	for _, oValue := range orders {
		tOrder := &model.OrderSimple{
			Id: oValue.Id,
			Project: &model.SimpleItem{
				Id:   oValue.Project.Id,
				Name: oValue.Project.Name,
			},
			Account:        oValue.ProjectAccount,
			ProjectOrderId: oValue.ProjectOrderId,
			PayerData:      oValue.PayerData,
			ProjectAmountIncome: &model.OrderSimpleAmountObject{
				Amount: oValue.ProjectIncomeAmount,
				Currency: &model.SimpleCurrency{
					CodeInt: oValue.ProjectIncomeCurrency.CodeInt,
					CodeA3:  oValue.ProjectIncomeCurrency.CodeA3,
					Name:    oValue.ProjectIncomeCurrency.Name,
				},
			},
			Status: &model.Status{
				Status:      oValue.Status,
				Name:        model.OrderStatusesNames[oValue.Status],
				Description: model.OrderStatusesDescription[oValue.Status],
			},
			VatAmount: oValue.VatAmount,
			CreatedAt: oValue.CreatedAt.Unix(),
		}

		if oValue.PaymentMethodIncomeAmount > 0 {
			tOrder.PaymentMethodAmountIncome = &model.OrderSimpleAmountObject{
				Amount: oValue.PaymentMethodIncomeAmount,
				Currency: &model.SimpleCurrency{
					CodeInt: oValue.PaymentMethodIncomeCurrency.CodeInt,
					CodeA3:  oValue.PaymentMethodIncomeCurrency.CodeA3,
					Name:    oValue.PaymentMethodIncomeCurrency.Name,
				},
			}
		}

		if oValue.AmountOutMerchantAccountingCurrency > 0 {
			tOrder.ProjectAmountOutcome = &model.OrderSimpleAmountObject{
				Amount: oValue.AmountOutMerchantAccountingCurrency,
				Currency: &model.SimpleCurrency{
					CodeInt: int(params.Merchant.Banking.Currency.CodeInt),
					CodeA3:  params.Merchant.Banking.Currency.CodeA3,
					Name:    &model.Name{EN: params.Merchant.Banking.Currency.Name.En},
				},
			}
		}

		if oValue.PaymentMethod != nil {
			tOrder.PaymentMethod = &model.SimpleItem{
				Id:   oValue.PaymentMethod.Id,
				Name: oValue.PaymentMethod.Name,
			}

			tOrder.PspFeeAmount = oValue.PspFeeAmount
			tOrder.PaymentSystemFeeAmount = oValue.PaymentSystemFeeAmount
			tOrder.ProjectFeeAmount = oValue.ProjectFeeAmount
			tOrder.ToPayerFeeAmount = oValue.ToPayerFeeAmount

			tOrder.PaymentRequisites = om.preparePaymentRequisites(oValue)
		}

		if oValue.PaymentMethodOrderClosedAt != nil {
			tOrder.ConfirmedAt = oValue.PaymentMethodOrderClosedAt.Unix()
		}

		if oValue.ProjectLastRequestedAt != nil {
			tOrder.ClosedAt = oValue.ProjectLastRequestedAt.Unix()
		}

		tOrders = append(tOrders, tOrder)
	}

	return tOrders, nil
}

// Prepare payer payment requisites for frontend
func (om *OrderManager) preparePaymentRequisites(o *model.Order) map[string]string {
	requisites := make(map[string]string)

	for k, v := range o.PaymentRequisites {
		n, ok := entity.PaymentRequisitesNames[k]

		if !ok {
			continue
		}

		requisites[n] = v
	}

	if len(o.PaymentMethodTxnParams) <= 0 {
		return requisites
	}

	for k, v := range o.PaymentMethodTxnParams {
		n, ok := entity.PaymentRequisitesNames[k]

		if !ok {
			continue
		}

		requisites[n] = v.(string)
	}

	return requisites
}

func (om *OrderManager) ProcessFilters(values url.Values, filter bson.M) bson.M {
	if id, ok := values[model.OrderFilterFieldId]; ok {
		if bson.IsObjectIdHex(id[0]) {
			filter["_id"] = bson.ObjectIdHex(id[0])
		} else {
			filter["_id"] = id[0]
		}
	}

	if pms, ok := values[model.OrderFilterFieldPaymentMethods]; ok {
		var fPms []bson.ObjectId

		for _, pm := range pms {
			fPms = append(fPms, bson.ObjectIdHex(pm))
		}

		filter["payment_method.id"] = bson.M{"$in": fPms}
	}

	if cs, ok := values[model.OrderFilterFieldCountries]; ok {
		filter["payer_data.country_code_a2"] = bson.M{"$in": cs}
	}

	if ss, ok := values[model.OrderFilterFieldStatuses]; ok {
		var ssi []int

		for _, s := range ss {
			si, err := strconv.Atoi(s)

			if err != nil {
				continue
			}

			ssi = append(ssi, si)
		}

		if len(ssi) > 0 {
			filter["status"] = bson.M{"$in": ssi}
		}
	}

	if a, ok := values[model.OrderFilterFieldAccount]; ok {
		ar := bson.RegEx{Pattern: ".*" + a[0] + ".*", Options: "i"}
		filter["$or"] = bson.M{"project_account": ar, "pm_account": ar, "payer_data.phone": ar, "payer_data.email": ar}
	}

	pmDates := make(bson.M)

	if pmDateFrom, ok := values[model.OrderFilterFieldPMDateFrom]; ok {
		if ts, err := strconv.ParseInt(pmDateFrom[0], 10, 64); err == nil {
			pmDates["$gte"] = time.Unix(ts, 0)
		}
	}

	if pmDateTo, ok := values[model.OrderFilterFieldPMDateTo]; ok {
		if ts, err := strconv.ParseInt(pmDateTo[0], 10, 64); err == nil {
			pmDates["$lte"] = time.Unix(ts, 0)
		}
	}

	if len(pmDates) > 0 {
		filter["pm_order_close_date"] = pmDates
	}

	prjDates := make(bson.M)

	if prjDateFrom, ok := values[model.OrderFilterFieldProjectDateFrom]; ok {
		if ts, err := strconv.ParseInt(prjDateFrom[0], 10, 64); err == nil {
			prjDates["$gte"] = time.Unix(ts, 0)
		}
	}

	if prjDateTo, ok := values[model.OrderFilterFieldProjectDateTo]; ok {
		if ts, err := strconv.ParseInt(prjDateTo[0], 10, 64); err == nil {
			prjDates["$lte"] = time.Unix(ts, 0)
		}
	}

	if len(prjDates) > 0 {
		filter["created_at"] = prjDates
	}

	return filter
}

func (om *OrderManager) ProcessCreatePayment(iData map[string]interface{}, psSettings map[string]interface{}) *payment_system.PaymentResponse {
	return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorSystem, "some error")

	/*var err error

	  if err = om.validateCreatePaymentData(iData); err != nil {
	      return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorValidation, err.Error())
	  }

	  data := om.convertCreatePaymentData(iData)
	  o := om.FindById(data[model.OrderPaymentCreateRequestFieldOrderId])

	  if o == nil {
	      return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorValidation, orderErrorNotFound)
	  }

	  if o.IsComplete() == true {
	      return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorValidation, orderErrorOrderAlreadyComplete)
	  }

	  pm := om.paymentMethodManager.FindById(bson.ObjectIdHex(data[model.OrderPaymentCreateRequestFieldOPaymentMethodId]))

	  if pm == nil {
	      return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorValidation, orderErrorPaymentMethodNotFound)
	  }

	  if o, err = om.modifyOrderAfterOrderFormSubmit(o, pm); err != nil {
	      return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorValidation, err.Error())
	  }

	  email, _ := data[model.OrderPaymentCreateRequestFieldEmail]
	  o.PayerData.Email = &email

	  delete(data, model.OrderPaymentCreateRequestFieldOrderId)
	  delete(data, model.OrderPaymentCreateRequestFieldOPaymentMethodId)
	  delete(data, model.OrderPaymentCreateRequestFieldEmail)

	  if val, ok := data[requestFieldOrderStoredCardId]; ok {
	      rsp, err := om.rep.FindSavedCardById(om.ctx, &repository.FindByStringValue{Value: val})

	      if err == nil && len(rsp.Pan) > 0 {
	          data[entity.BankCardFieldPan] = rsp.Pan
	          data[entity.BankCardFieldHolder] = rsp.CardHolder
	          data[entity.BankCardFieldMonth] = rsp.Expire.Month
	          data[entity.BankCardFieldYear] = rsp.Expire.Year
	      }
	  }

	  // if it bank card payment, then get data about bank issuer
	  val, ok := data[entity.BankCardFieldPan]

	  if ok && len(val) > 0 {
	      binData, err := om.rep.FindBinData(om.ctx, &repository.FindByStringValue{Value: val})

	      if err == nil && binData != nil {
	          data[model.BankCardFieldBrand] = binData.GetCardBrand()
	          data[model.BankCardFieldType] = binData.GetCardType()
	          data[model.BankCardFieldCategory] = binData.GetCardCategory()
	          data[model.BankCardFieldIssuerName] = binData.GetBankName()
	          data[model.BankCardFieldIssuerCountry] = binData.GetBankCountryName()
	      }

	      if v, ok := iData["store_data"]; ok && v.(bool) == true {
	          req := &repository.SavedCardRequest{
	              Account:    o.ProjectAccount,
	              ProjectId:  o.Project.Id.Hex(),
	              Pan:        data[entity.BankCardFieldPan],
	              CardHolder: data[entity.BankCardFieldHolder],
	              Expire: &billing.CardExpire{
	                  Month: data[entity.BankCardFieldMonth],
	                  Year:  data[entity.BankCardFieldYear],
	              },
	          }

	          if _, err := om.rep.InsertSavedCard(om.ctx, req); err != nil {
	              log.Println(err)
	          }
	      }
	  }

	  o.PaymentRequisites = data
	  o.PaymentMethodTerminalId = pm.Params.Terminal

	  if o.ProjectAccount == "" {
	      o.ProjectAccount = email
	  }

	  if o, err = om.updateOrderAccountingAmounts(o); err != nil {
	      return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorSystem, err.Error())
	  }

	  handler, err := om.paymentSystemsSettings.GetPaymentHandler(o, psSettings)

	  if err != nil {
	      return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorSystem, err.Error())
	  }

	  res := handler.CreatePayment()

	  if res.Status == payment_system.PaymentStatusOK {
	      o.Status = model.OrderStatusPaymentSystemCreate
	  }

	  if res.Status == payment_system.PaymentStatusErrorSystem {
	      o.Status = model.OrderStatusPaymentSystemRejectOnCreate
	  }

	  if err = om.Database.Repository(TableOrder).UpdateOrder(o); err != nil {
	      return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorSystem, err.Error())
	  }

	  return res*/
}

func (om *OrderManager) ProcessNotifyPayment(opn *model.OrderPaymentNotification, psSettings map[string]interface{}) *payment_system.PaymentResponse {
	o := om.FindById(opn.Id)

	if o == nil {
		return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorValidation, orderErrorNotFound)
	}

	if !o.CanProcessNotify() {
		if o.Status == model.OrderStatusNew {
			return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorValidation, orderErrorCanNotCreate)
		}

		if o.Status == model.OrderStatusPaymentSystemDeclined {
			return payment_system.NewPaymentResponse(payment_system.PaymentStatusOK, orderErrorOrderDeclined)
		}

		if o.Status == model.OrderStatusPaymentSystemCanceled {
			return payment_system.NewPaymentResponse(payment_system.PaymentStatusOK, orderErrorOrderCanceled)
		}

		return payment_system.NewPaymentResponse(payment_system.PaymentStatusOK, orderErrorOrderAlreadyComplete)
	}

	handler, err := om.paymentSystemsSettings.GetPaymentHandler(o, psSettings)

	if err != nil {
		return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorSystem, err.Error())
	}

	o.UpdatedAt = time.Now()

	res := handler.ProcessPayment(o, opn)

	if res.Status == payment_system.PaymentStatusTemporary {
		return res
	}

	if res.Status == payment_system.PaymentStatusOK {
		o = res.Order

		if o, err = om.processNotifyPaymentAmounts(o); err != nil {
			return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorSystem, err.Error())
		}

		err = om.pub.Publish(
			constant.PayOneTopicNotifyPaymentName,
			om.getPublisherOrder(o),
			amqp.Table{"x-retry-count": int32(0)},
		)

		if err != nil {
			return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorSystem, err.Error())
		}
	}

	if _, err = om.UpdateOrder(o); err != nil {
		return payment_system.NewPaymentResponse(payment_system.PaymentStatusErrorSystem, model.ResponseMessageUnknownDbError)
	}

	return res
}

func (om *OrderManager) processNotifyPaymentAmounts(o *model.Order) (*model.Order, error) {
	var err error

	o.PaymentMethodIncomeCurrency = om.currencyManager.FindByCodeA3(o.PaymentMethodIncomeCurrencyA3)

	if o.PaymentMethodIncomeCurrency == nil {
		return nil, errors.New(orderErrorOrderPaymentMethodIncomeCurrencyNotFound)
	}

	o.ProjectOutcomeAmount, err = om.currencyRateManager.convert(
		o.PaymentMethodIncomeCurrency.CodeInt,
		int(o.ProjectOutcomeCurrency.CodeInt),
		o.PaymentMethodIncomeAmount,
	)

	if err != nil {
		return nil, err
	}

	if om.pspAccountingCurrency == nil {
		return nil, errors.New(orderErrorOrderPSPAccountingCurrencyNotFound)
	}

	o.AmountInPSPAccountingCurrency, err = om.currencyRateManager.convert(
		o.PaymentMethodIncomeCurrency.CodeInt,
		om.pspAccountingCurrency.CodeInt,
		o.PaymentMethodIncomeAmount,
	)

	if err != nil {
		return nil, err
	}

	o.AmountOutMerchantAccountingCurrency, err = om.currencyRateManager.convert(
		o.PaymentMethodIncomeCurrency.CodeInt,
		int(o.Project.Merchant.Banking.Currency.CodeInt),
		o.PaymentMethodIncomeAmount,
	)

	if err != nil {
		return nil, err
	}

	o.AmountInPaymentSystemAccountingCurrency, err = om.currencyRateManager.convert(
		o.PaymentMethodIncomeCurrency.CodeInt,
		o.PaymentMethod.PaymentSystem.AccountingCurrency.CodeInt,
		o.PaymentMethodIncomeAmount,
	)

	if err != nil {
		return nil, err
	}

	return o, nil
}

func (om *OrderManager) validateCreatePaymentData(data map[string]interface{}) error {
	if _, ok := data[model.OrderPaymentCreateRequestFieldOrderId]; !ok {
		return errors.New(orderErrorCreatePaymentRequiredFieldIdNotFound)
	}

	if _, ok := data[model.OrderPaymentCreateRequestFieldOPaymentMethodId]; !ok {
		return errors.New(orderErrorCreatePaymentRequiredFieldPaymentMethodNotFound)
	}

	if _, ok := data[model.OrderPaymentCreateRequestFieldEmail]; !ok {
		return errors.New(orderErrorCreatePaymentRequiredFieldEmailNotFound)
	}

	return nil
}

func (om *OrderManager) convertCreatePaymentData(iData map[string]interface{}) map[string]string {
	data := make(map[string]string)

	for k, v := range iData {
		switch sv := v.(type) {
		case bool:
			continue
		default:
			data[k] = fmt.Sprintf("%v", sv)
		}
	}

	return data
}

func (om *OrderManager) modifyOrderAfterOrderFormSubmit(o *model.Order, pm *model.PaymentMethod) (*model.Order, error) {
	if o.PaymentMethod != nil && o.PaymentMethod.Id == pm.Id {
		return o, nil
	}

	p := om.projectManager.FindProjectById(o.Project.Id.Hex())

	if p == nil {
		return nil, errors.New(orderErrorProjectNotFound)
	}

	check := &check{
		order: &model.OrderScalar{
			Amount: o.ProjectIncomeAmount,
		},
		project:       p,
		oCurrency:     o.ProjectIncomeCurrency,
		paymentMethod: pm,
	}

	pmOutData, err := om.checkPaymentMethodLimits(check)

	if err != nil {
		return nil, err
	}

	o.PaymentMethod = &model.OrderPaymentMethod{
		Id:            pm.Id,
		Name:          pm.Name,
		Params:        pm.Params,
		PaymentSystem: pm.PaymentSystem,
		GroupAlias:    pm.GroupAlias,
	}

	o.PaymentMethodOutcomeAmount = FormatAmount(pmOutData.amount)
	o.PaymentMethodOutcomeCurrency = pmOutData.currency

	if o, err = om.ProcessOrderCommissions(o); err != nil {
		return nil, err
	}

	return o, nil
}

func (om *OrderManager) GetRevenueDynamic(rdr *model.RevenueDynamicRequest) (*model.RevenueDynamicResult, error) {
	rdr.From = utils.GetTimeRangeFrom(rdr.From)
	rdr.To = utils.GetTimeRangeFrom(rdr.To)

	res, err := om.Database.Repository(TableOrder).GetRevenueDynamic(rdr)

	if err != nil {
		return nil, err
	}

	refPoints := make(map[string]float64)

	var revPoints []*model.RevenueDynamicPoint

	pRefund := res[0][model.RevenueDynamicFacetFieldPointsRefund].([]interface{})
	pRevenue := res[0][model.RevenueDynamicFacetFieldPointsRevenue].([]interface{})

	for _, v := range pRefund {
		vm := v.(map[string]interface{})

		vmId := vm[model.RevenueDynamicFacetFieldId].(map[string]interface{})
		vmTotal := vm[model.RevenueDynamicFacetFieldTotal].(float64)

		refPoints[om.getRevenueDynamicPointsKey(vmId).String()] = FormatAmount(vmTotal)
	}

	for _, v := range pRevenue {
		vm := v.(map[string]interface{})

		vmId := vm[model.RevenueDynamicFacetFieldId].(map[string]interface{})
		vmTotal := vm[model.RevenueDynamicFacetFieldTotal].(float64)

		revPointDate := om.getRevenueDynamicPointsKey(vmId)
		refVal, ok := refPoints[revPointDate.String()]

		revPoint := &model.RevenueDynamicPoint{
			Date: revPointDate,
		}

		if ok {
			revPoint.Amount = FormatAmount(vmTotal - refVal)
		} else {
			revPoint.Amount = FormatAmount(vmTotal)
		}

		revPoints = append(revPoints, revPoint)
	}

	rd := &model.RevenueDynamicResult{
		Points:  revPoints,
		Revenue: &model.RevenueDynamicMainData{Count: 0, Total: 0, Avg: 0},
		Refund:  &model.RevenueDynamicMainData{Count: 0, Total: 0, Avg: 0},
	}

	rev := res[0][model.RevenueDynamicFacetFieldRevenue].([]interface{})

	if len(rev) > 0 {
		mRev := rev[0].(map[string]interface{})

		if v, ok := mRev[model.RevenueDynamicFacetFieldCount]; ok {
			rd.Revenue.Count = v.(int)
		}

		if v, ok := mRev[model.RevenueDynamicFacetFieldTotal]; ok {
			rd.Revenue.Total = FormatAmount(v.(float64))
		}

		if v, ok := mRev[model.RevenueDynamicFacetFieldAvg]; ok {
			rd.Revenue.Avg = FormatAmount(v.(float64))
		}
	}

	ref := res[0][model.RevenueDynamicFacetFieldRefund].([]interface{})

	if len(ref) > 0 {
		mRef := ref[0].(map[string]interface{})

		if v, ok := mRef[model.RevenueDynamicFacetFieldCount]; ok {
			rd.Refund.Count = v.(int)
		}

		if v, ok := mRef[model.RevenueDynamicFacetFieldTotal]; ok {
			rd.Refund.Total = FormatAmount(v.(float64))
		}

		if v, ok := mRef[model.RevenueDynamicFacetFieldAvg]; ok {
			rd.Refund.Avg = FormatAmount(v.(float64))
		}
	}

	return rd, err
}

func (om *OrderManager) getRevenueDynamicPointsKey(pointId map[string]interface{}) *model.RevenueDynamicPointDate {
	revPointDate := &model.RevenueDynamicPointDate{
		Year: pointId[model.RevenueDynamicRequestPeriodYear].(int),
	}

	if val, ok := pointId[model.RevenueDynamicRequestPeriodMonth]; ok {
		revPointDate.Month = val.(int)
	}

	if val, ok := pointId[model.RevenueDynamicRequestPeriodWeek]; ok {
		revPointDate.Week = val.(int)
	}

	if val, ok := pointId[model.RevenueDynamicRequestPeriodDay]; ok {
		revPointDate.Day = val.(int)
	}

	if val, ok := pointId[model.RevenueDynamicRequestPeriodHour]; ok {
		revPointDate.Hour = val.(int)
	}

	return revPointDate
}

// Calculate order accounting amounts and update this amounts in order struct
func (om *OrderManager) updateOrderAccountingAmounts(o *model.Order) (*model.Order, error) {
	var cAmount float64
	var err error

	pmCodeInt := o.PaymentMethodOutcomeCurrency.CodeInt

	// calculate and save order amount in PSP accounting currency
	cAmount, err = om.currencyRateManager.convert(pmCodeInt, om.pspAccountingCurrency.CodeInt, o.PaymentMethodOutcomeAmount)

	if err != nil {
		return nil, err
	}

	o.AmountInPSPAccountingCurrency = FormatAmount(cAmount)

	// calculate and save order amount in merchant accounting currency
	cAmount, err = om.currencyRateManager.convert(o.ProjectIncomeCurrency.CodeInt, int(o.Project.Merchant.Banking.Currency.CodeInt), o.ProjectIncomeAmount)

	if err != nil {
		return nil, err
	}

	o.AmountOutMerchantAccountingCurrency = FormatAmount(cAmount)

	// calculate and save order amount in payment system accounting currency
	cAmount, err = om.currencyRateManager.convert(pmCodeInt, o.PaymentMethod.PaymentSystem.AccountingCurrency.CodeInt, o.PaymentMethodOutcomeAmount)

	if err != nil {
		return nil, err
	}

	o.AmountInPaymentSystemAccountingCurrency = FormatAmount(cAmount)

	return o, nil
}

func (om *OrderManager) UpdateOrder(o *model.Order) (*model.Order, error) {
	err := om.Database.Repository(TableOrder).UpdateOrder(o)

	if err != nil {
		om.Logger.Errorf("Query from table \"%s\" ended with error: %s", TableOrder, err)
		return nil, err
	}

	return o, nil
}

// Get data about accounting payment by accounting period of merchant
func (om *OrderManager) GetAccountingPayment(rdr *model.RevenueDynamicRequest, mId string) (*model.AccountingPayment, error) {
	rdr.From = utils.GetTimeRangeFrom(rdr.From)
	rdr.To = utils.GetTimeRangeTo(rdr.To)

	res, err := om.Database.Repository(TableOrder).GetAccountingPayment(rdr, mId)

	if err != nil {
		return nil, err
	}

	apData := res[0]
	ap := &model.AccountingPayment{}

	if v, ok := apData["total_success"]; ok {
		sV := v.([]interface{})

		if len(sV) > 0 {
			mV := sV[0].(map[string]interface{})

			if vv, ok := mV["total"]; ok {
				ap.SuccessWithCommissions = vv.(float64)
			}
		}
	}

	if v, ok := apData["success"]; ok {
		ap.SuccessWithoutCommissions = om.getValueFromAccountingPaymentReport(v)
	}

	if v, ok := apData["refund"]; ok {
		ap.TotalRefund = om.getValueFromAccountingPaymentReport(v)
	}

	if v, ok := apData["chargeback"]; ok {
		ap.TotalChargeback = om.getValueFromAccountingPaymentReport(v)
	}

	if v, ok := apData["commission"]; ok {
		ap.TotalCommission = om.getValueFromAccountingPaymentReport(v)
	}

	return ap, nil
}

func (om *OrderManager) getValueFromAccountingPaymentReport(v interface{}) float64 {
	var fV float64

	sV := v.([]interface{})

	if len(sV) > 0 {
		mV := sV[0].(map[string]interface{})

		if vv, ok := mV["total"]; ok {
			fV = FormatAmount(vv.(float64))
		}
	}

	return fV
}

// temporary method helper to convert model.Order to proto.Order
func (om *OrderManager) getPublisherOrder(o *model.Order) *billing.Order {
	sProjectParams := make(map[string]string)
	sPaymentMethodTxnParams := make(map[string]string)

	for k, v := range o.ProjectParams {
		sProjectParams[k] = fmt.Sprintf("%s", v)
	}

	for k, v := range o.PaymentMethodTxnParams {
		sPaymentMethodTxnParams[k] = fmt.Sprintf("%s", v)
	}

	pOrder := &billing.Order{
		Id: o.Id.Hex(),
		Project: &billing.ProjectOrder{
			Id:               o.Project.Id.Hex(),
			NotifyEmails:     o.Project.NotifyEmails,
			SendNotifyEmail:  o.Project.SendNotifyEmail,
			SecretKey:        o.Project.SecretKey,
			CallbackProtocol: o.Project.CallbackProtocol,
		},
		ProjectAccount:      o.ProjectAccount,
		Description:         o.Description,
		ProjectIncomeAmount: o.ProjectIncomeAmount,
		ProjectIncomeCurrency: &billing.Currency{
			CodeInt:  int32(o.ProjectIncomeCurrency.CodeInt),
			CodeA3:   o.ProjectIncomeCurrency.CodeA3,
			Name:     &billing.Name{En: o.ProjectIncomeCurrency.Name.EN, Ru: o.ProjectIncomeCurrency.Name.RU},
			IsActive: o.ProjectIncomeCurrency.IsActive,
		},
		ProjectParams: sProjectParams,
		PaymentMethod: &billing.PaymentMethodOrder{
			Id:   o.PaymentMethod.Id.Hex(),
			Name: o.PaymentMethod.Name,
			Params: &billing.PaymentMethodParams{
				Handler:    o.PaymentMethod.Params.Handler,
				Terminal:   o.PaymentMethod.Params.Terminal,
				ExternalId: o.PaymentMethod.Params.ExternalId,
				Other:      o.PaymentMethod.Params.Other,
			},
			PaymentSystem: &billing.PaymentSystem{
				Id:   o.PaymentMethod.PaymentSystem.Id.Hex(),
				Name: o.PaymentMethod.PaymentSystem.Name,
				Country: &billing.Country{
					CodeInt:  int32(o.PaymentMethod.PaymentSystem.Country.CodeInt),
					CodeA2:   o.PaymentMethod.PaymentSystem.Country.CodeA2,
					CodeA3:   o.PaymentMethod.PaymentSystem.Country.CodeA3,
					Name:     &billing.Name{En: o.PaymentMethod.PaymentSystem.Country.Name.EN, Ru: o.PaymentMethod.PaymentSystem.Country.Name.RU},
					IsActive: o.PaymentMethod.PaymentSystem.Country.IsActive,
				},
				AccountingCurrency: &billing.Currency{
					CodeInt:  int32(o.PaymentMethod.PaymentSystem.AccountingCurrency.CodeInt),
					CodeA3:   o.PaymentMethod.PaymentSystem.AccountingCurrency.CodeA3,
					Name:     &billing.Name{En: o.PaymentMethod.PaymentSystem.AccountingCurrency.Name.EN, Ru: o.PaymentMethod.PaymentSystem.AccountingCurrency.Name.RU},
					IsActive: o.PaymentMethod.PaymentSystem.AccountingCurrency.IsActive,
				},
				AccountingPeriod: o.PaymentMethod.PaymentSystem.AccountingPeriod,
				IsActive:         o.PaymentMethod.PaymentSystem.IsActive,
			},
			Group: o.PaymentMethod.GroupAlias,
		},
		PaymentMethodOrderId:       o.PaymentMethodOrderId,
		PaymentMethodOutcomeAmount: o.PaymentMethodOutcomeAmount,
		PaymentMethodOutcomeCurrency: &billing.Currency{
			CodeInt:  int32(o.PaymentMethodOutcomeCurrency.CodeInt),
			CodeA3:   o.PaymentMethodOutcomeCurrency.CodeA3,
			Name:     &billing.Name{En: o.PaymentMethodOutcomeCurrency.Name.EN, Ru: o.PaymentMethodOutcomeCurrency.Name.RU},
			IsActive: o.PaymentMethodOutcomeCurrency.IsActive,
		},
		PaymentMethodIncomeAmount: o.PaymentMethodIncomeAmount,
		PaymentMethodIncomeCurrency: &billing.Currency{
			CodeInt:  int32(o.PaymentMethodIncomeCurrency.CodeInt),
			CodeA3:   o.PaymentMethodIncomeCurrency.CodeA3,
			Name:     &billing.Name{En: o.PaymentMethodIncomeCurrency.Name.EN, Ru: o.PaymentMethodIncomeCurrency.Name.RU},
			IsActive: o.PaymentMethodIncomeCurrency.IsActive,
		},
		Status:                                  int32(o.Status),
		IsJsonRequest:                           o.IsJsonRequest,
		AmountInPspAccountingCurrency:           o.AmountInPSPAccountingCurrency,
		AmountInMerchantAccountingCurrency:      o.AmountInMerchantAccountingCurrency,
		AmountOutMerchantAccountingCurrency:     o.AmountOutMerchantAccountingCurrency,
		AmountInPaymentSystemAccountingCurrency: o.AmountInPaymentSystemAccountingCurrency,
		PaymentMethodPayerAccount:               o.PaymentMethodPayerAccount,
		PaymentMethodTxnParams:                  sPaymentMethodTxnParams,
		Items:                                   o.Items,
		PaymentRequisites:                       o.PaymentRequisites,
		PspFeeAmount: &billing.OrderFeePsp{
			AmountPaymentMethodCurrency: o.PspFeeAmount.AmountPaymentMethodCurrency,
			AmountMerchantCurrency:      o.PspFeeAmount.AmountMerchantCurrency,
			AmountPspCurrency:           o.PspFeeAmount.AmountPspCurrency,
		},
		ProjectFeeAmount: &billing.OrderFee{
			AmountPaymentMethodCurrency: o.ProjectFeeAmount.AmountPaymentMethodCurrency,
			AmountMerchantCurrency:      o.ProjectFeeAmount.AmountMerchantCurrency,
		},
		ToPayerFeeAmount: &billing.OrderFee{
			AmountPaymentMethodCurrency: o.ToPayerFeeAmount.AmountPaymentMethodCurrency,
			AmountMerchantCurrency:      o.ToPayerFeeAmount.AmountMerchantCurrency,
		},
		PaymentSystemFeeAmount: &billing.OrderFeePaymentSystem{
			AmountPaymentMethodCurrency: o.PaymentSystemFeeAmount.AmountPaymentMethodCurrency,
			AmountMerchantCurrency:      o.PaymentSystemFeeAmount.AmountMerchantCurrency,
			AmountPaymentSystemCurrency: o.PaymentSystemFeeAmount.AmountMerchantCurrency,
		},
	}

	if o.ProjectLastRequestedAt != nil {
		if v, err := ptypes.TimestampProto(*o.ProjectLastRequestedAt); err == nil {
			pOrder.ProjectLastRequestedAt = v
		}
	}

	if v, err := ptypes.TimestampProto(o.CreatedAt); err == nil {
		pOrder.CreatedAt = v
	}

	if v, err := ptypes.TimestampProto(o.UpdatedAt); err == nil {
		pOrder.UpdatedAt = v
	}

	if o.PaymentMethodOrderClosedAt != nil {
		if v, err := ptypes.TimestampProto(*o.PaymentMethodOrderClosedAt); err == nil {
			pOrder.PaymentMethodOrderClosedAt = v
		}
	}

	if v, err := ptypes.TimestampProto(o.PaymentMethodOutcomeCurrency.CreatedAt); err == nil {
		pOrder.PaymentMethodOutcomeCurrency.CreatedAt = v
	}

	if v, err := ptypes.TimestampProto(o.PaymentMethodOutcomeCurrency.UpdatedAt); err == nil {
		pOrder.PaymentMethodOutcomeCurrency.UpdatedAt = v
	}

	if v, err := ptypes.TimestampProto(o.PaymentMethodIncomeCurrency.CreatedAt); err == nil {
		pOrder.PaymentMethodIncomeCurrency.CreatedAt = v
	}

	if v, err := ptypes.TimestampProto(o.PaymentMethodIncomeCurrency.UpdatedAt); err == nil {
		pOrder.PaymentMethodIncomeCurrency.UpdatedAt = v
	}

	if v, err := ptypes.TimestampProto(o.PaymentMethod.PaymentSystem.AccountingCurrency.CreatedAt); err == nil {
		pOrder.PaymentMethod.PaymentSystem.AccountingCurrency.CreatedAt = v
	}

	if v, err := ptypes.TimestampProto(o.PaymentMethod.PaymentSystem.AccountingCurrency.UpdatedAt); err == nil {
		pOrder.PaymentMethod.PaymentSystem.AccountingCurrency.UpdatedAt = v
	}

	if v, err := ptypes.TimestampProto(o.PaymentMethod.PaymentSystem.Country.CreatedAt); err == nil {
		pOrder.PaymentMethod.PaymentSystem.Country.CreatedAt = v
	}

	if v, err := ptypes.TimestampProto(o.PaymentMethod.PaymentSystem.Country.UpdatedAt); err == nil {
		pOrder.PaymentMethod.PaymentSystem.Country.UpdatedAt = v
	}

	if v, err := ptypes.TimestampProto(o.PaymentMethod.PaymentSystem.CreatedAt); err == nil {
		pOrder.PaymentMethod.PaymentSystem.CreatedAt = v
	}

	if v, err := ptypes.TimestampProto(o.PaymentMethod.PaymentSystem.UpdatedAt); err == nil {
		pOrder.PaymentMethod.PaymentSystem.UpdatedAt = v
	}

	if v, err := ptypes.TimestampProto(o.ProjectIncomeCurrency.CreatedAt); err == nil {
		pOrder.ProjectIncomeCurrency.CreatedAt = v
	}

	if v, err := ptypes.TimestampProto(o.ProjectIncomeCurrency.UpdatedAt); err == nil {
		pOrder.ProjectIncomeCurrency.UpdatedAt = v
	}

	if o.Project.UrlSuccess != "" {
		pOrder.Project.UrlSuccess = o.Project.UrlSuccess
	}

	if o.Project.UrlFail != "" {
		pOrder.Project.UrlFail = o.Project.UrlFail
	}

	if o.Project.URLCheckAccount != "" {
		pOrder.Project.UrlCheckAccount = o.Project.URLCheckAccount
	}

	if o.Project.URLProcessPayment != "" {
		pOrder.Project.UrlProcessPayment = o.Project.URLProcessPayment
	}

	return pOrder
}
