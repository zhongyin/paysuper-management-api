package model

import (
	"github.com/globalsign/mgo/bson"
	"time"
)

const (
	orderFieldProjectId     = "PP_PROJECT_ID"
	orderFieldSignature     = "PP_SIGNATURE"
	orderFieldAmount        = "PP_AMOUNT"
	orderFieldCurrency      = "PP_CURRENCY"
	orderFieldAccount       = "PP_ACCOUNT"
	orderFieldOrderId       = "PP_ORDER_ID"
	orderFieldPaymentMethod = "PP_PAYMENT_METHOD"
	orderFieldUrlVerify     = "PP_URL_VERIFY"
	orderFieldUrlNotify     = "PP_URL_NOTIFY"
	orderFieldUrlSuccess    = "PP_URL_SUCCESS"
	orderFieldUrlFail       = "PP_URL_FAIL"
	orderFieldPayerEmail    = "PP_PAYER_EMAIL"
	orderFieldPayerPhone    = "PP_PAYER_PHONE"
	orderFieldDescription   = "PP_DESCRIPTION"
	orderFieldRegion        = "PP_REGION"

	OrderStatusCreated  = 0
	OrderStatusComplete = 10

	OrderFilterFieldProjects        = "projects"
	OrderFilterFieldId              = "id"
	OrderFilterFieldPaymentMethods  = "payment_methods"
	OrderFilterFieldCountries       = "countries"
	OrderFilterFieldStatuses        = "statuses"
	OrderFilterFieldAccount         = "account"
	OrderFilterFieldPMDateFrom      = "pm_date_from"
	OrderFilterFieldPMDateTo        = "pm_date_to"
	OrderFilterFieldProjectDateFrom = "project_date_from"
	OrderFilterFieldProjectDateTo   = "project_date_to"
)

var OrderReservedWords = map[string]bool{
	orderFieldProjectId:     true,
	orderFieldSignature:     true,
	orderFieldAmount:        true,
	orderFieldCurrency:      true,
	orderFieldAccount:       true,
	orderFieldOrderId:       true,
	orderFieldDescription:   true,
	orderFieldPaymentMethod: true,
	orderFieldUrlVerify:     true,
	orderFieldUrlNotify:     true,
	orderFieldUrlSuccess:    true,
	orderFieldUrlFail:       true,
	orderFieldPayerEmail:    true,
	orderFieldPayerPhone:    true,
	orderFieldRegion:        true,
}

var OrderStatusesDescription = map[int]string{
	OrderStatusCreated:  "Order created",
	OrderStatusComplete: "Order successfully complete. Notification successfully send to project",
}

type PayerData struct {
	// payer ip from create order request
	Ip string `bson:"ip" json:"ip"`
	// payer country code by ISO 3166-1 from create order request
	CountryCodeA2 string `bson:"country_code_a2" json:"country_code_a2"`
	// payer country names
	CountryName *Name `bson:"country_name" json:"country_name"`
	// payer city names, get from ip geo location
	City *Name `bson:"city" json:"city"`
	// payer timezone name, get from ip geo location
	Timezone string `bson:"timezone" json:"timezone"`
	// payer phone from create order request
	Phone *string `bson:"phone,omitempty" json:"phone,omitempty"`
	// payer email from create order request
	Email *string `bson:"email,omitempty" json:"email,omitempty"`
}

type OrderScalar struct {
	// project unique identifier in Protocol One payment solution
	ProjectId string `query:"PP_PROJECT_ID" form:"PP_PROJECT_ID" json:"project" validate:"required,hexadecimal" swaggertype:"string"`
	// signature of request to verify that the data has not been changed. This field not required, BUT we're recommend send this field always
	Signature *string `query:"PP_SIGNATURE" form:"PP_SIGNATURE" json:"signature" validate:"omitempty,alphanum" swaggertype:"string"`
	// order amount
	Amount float64 `query:"PP_AMOUNT" form:"PP_AMOUNT" json:"amount" validate:"required,numeric" swaggertype:"number"`
	// order currency by ISO 4217 (3 chars). If this field send, then we're process amount in this currency
	Currency *string `query:"PP_CURRENCY" form:"PP_CURRENCY" json:"currency" validate:"omitempty,alpha,len=3" swaggertype:"string"`
	// user unique account in project
	Account string `query:"PP_ACCOUNT" form:"PP_ACCOUNT" json:"account" validate:"required"`
	// unique order identifier in project. This field not required, BUT we're recommend send this field always
	OrderId *string `query:"PP_ORDER_ID" form:"PP_ORDER_ID" json:"order_id"`
	// order description. If this field not send in request, then we're create standard order description
	Description *string `query:"PP_DESCRIPTION" form:"PP_DESCRIPTION" json:"description"`
	// payment method identifier in Protocol One payment solution
	PaymentMethod *string `query:"PP_PAYMENT_METHOD" form:"PP_PAYMENT_METHOD" json:"payment_method"`
	// URL for payment data verification request to project. This field can be send if it allowed in project admin panel
	UrlVerify *string `query:"PP_URL_VERIFY" form:"PP_URL_VERIFY" json:"url_verify" validate:"omitempty,url"`
	// URL for payment notification request to project. This field can be send if it allowed in project admin panel
	UrlNotify *string `query:"PP_URL_NOTIFY" form:"PP_URL_NOTIFY" json:"url_notify" validate:"omitempty,url"`
	// URL for redirect user after successfully completed payment. This field can be send if it allowed in project admin panel
	UrlSuccess *string `query:"PP_URL_SUCCESS" form:"PP_URL_SUCCESS" json:"url_success" validate:"omitempty,url"`
	// URL for redirect user after failed payment. This field can be send if it allowed in project admin panel
	UrlFail *string `query:"PP_URL_FAIL" form:"PP_URL_FAIL" json:"url_fail" validate:"omitempty,url"`
	// user (payer) email
	PayerEmail *string `query:"PP_PAYER_EMAIL" form:"PP_PAYER_EMAIL" json:"payer_email" validate:"omitempty,email"`
	// user (payer) phone
	PayerPhone *string `query:"PP_PAYER_PHONE" form:"PP_PAYER_PHONE" json:"payer_phone"`
	// user (payer) region code by ISO 3166-1 (2 chars) for check project packages. If this field not send, then user region will be get from user ip
	Region *string `query:"PP_REGION" form:"PP_REGION" json:"region" validate:"omitempty,alpha,len=2"`
	// user (payer) ip
	CreateOrderIp string `json:"payer_ip"`
	// object with any fields on the project side that do not match the names of the reserved fields
	Other            map[string]interface{} `json:"other"`
	RawRequestParams map[string]string      `json:"-"`
	RawRequestBody   string                 `json:"-"`
	IsJsonRequest    bool                   `json:"-"`
}

type Order struct {
	// unique order identifier in Protocol One
	Id bson.ObjectId `bson:"_id" json:"id"`
	// project unique identifier in Protocol One payment solution
	ProjectId bson.ObjectId `bson:"project_id" json:"project_id"`
	// unique order identifier in project. if was send in create order process
	ProjectOrderId *string `bson:"project_order_id" json:"project_order_id"`
	// user unique account in project
	ProjectAccount string `bson:"project_account" json:"project_account"`
	// order amount received from project
	ProjectIncomeAmount float64 `bson:"project_income_amount" json:"project_income_amount"`
	// order currency received from project
	ProjectIncomeCurrency *Currency `bson:"project_income_currency" json:"project_income_currency"`
	// order amount send to project in notification request
	ProjectOutcomeAmount float64 `bson:"project_outcome_amount" json:"project_outcome_amount"`
	// order currency send to project in notification request
	ProjectOutcomeCurrency *Currency `bson:"project_outcome_currency" json:"project_outcome_currency"`
	// fee is charged with the project for the operation
	ProjectFee float64 `bson:"project_fee" json:"project_fee"`
	// date of last notification request to project
	ProjectLastRequestedAt *time.Time `bson:"project_last_requested_at" json:"project_last_requested_at,omitempty"`
	// any project params which received from project in request of create of order
	ProjectParams map[string]interface{} `bson:"project_params" json:"project_params"`
	// information about payer, for example: ip, email,phone and etc
	PayerData *PayerData `bson:"payer_data" json:"payer_data"`
	// payment method unique identifier
	PaymentMethodId bson.ObjectId `bson:"pm_id" json:"pm_id"`
	// identifier of terminal for process payment in payment system side
	PaymentMethodTerminalId string `bson:"pm_terminal_id" json:"pm_terminal_id"`
	// unique order id in payment system
	PaymentMethodOrderId string `bson:"pm_order_id" json:"pm_order_id"`
	// order amount send to payment system
	PaymentMethodOutcomeAmount float64 `bson:"pm_outcome_amount" json:"pm_outcome_amount"`
	// order currency send to payment system
	PaymentMethodOutcomeCurrency *Currency `bson:"pm_outcome_currency" json:"pm_outcome_currency"`
	// order amount received from payment system in notification request
	PaymentMethodIncomeAmount float64 `bson:"pm_income_amount" json:"pm_income_amount"`
	// order currency received from payment system in notification request
	PaymentMethodIncomeCurrency *Currency `bson:"pm_income_currency" json:"pm_income_currency"`
	// payment system fee for payment operation
	PaymentMethodFee float64 `bson:"pm_fee" json:"pm_fee"`
	// date of ended payment operation in payment system
	PaymentMethodOrderClosedAt *time.Time `bson:"pm_order_close_date" json:"pm_order_close_date,omitempty"`
	// order status
	Status int `bson:"status" json:"status"`
	// date of create order
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	// date of last update order data
	UpdatedAt *time.Time `bson:"updated_at" json:"created_at,omitempty"`
	// is order create by json request
	IsJsonRequest bool `bson:"created_by_json" json:"created_by_json"`
	// operation amount in accounting currency of PSP
	AmountInPSPAccountingCurrency float64 `bson:"amount_psp_ac" json:"amount_psp_accounting_currency"`
	// received from project operation amount in project owner (merchant) accounting currency
	AmountInMerchantAccountingCurrency float64 `bson:"amount_in_merchant_ac" json:"amount_in_merchant_accounting_currency"`
	// received from payment system operation amount in project owner (merchant) accounting currency
	AmountOutMerchantAccountingCurrency float64 `bson:"amount_out_merchant_ac" json:"amount_out_merchant_accounting_currency"`
	// operation amount in payment system accounting currency
	AmountInPaymentSystemAccountingCurrency float64 `bson:"amount_ps_ac" json:"amount_ps_accounting_currency"`
	// account of payer in payment system
	PaymentMethodPayerAccount string `bson:"pm_account" json:"pm_account"`
	// any params received in request of payment system about payment
	PaymentMethodTxnParams map[string]interface{} `bson:"pm_txn_params" json:"pm_txn_params"`
	// fixed package which buy payer
	FixedPackage *OrderFixedPackage `bson:"fixed_package" json:"fixed_package"`

	ProjectOutcomeAmountPrintable string   `bson:"-" json:"-"`
	OrderIdPrintable              string   `bson:"-" json:"-"`
	ProjectData                   *Project `bson:"-" json:"-"`
}

type OrderUrl struct {
	// link for user to payment confirmation form
	OrderUrl string `json:"order_url"`
}

type OrderSimple struct {
	// unique order identifier in Protocol One
	Id bson.ObjectId `json:"id"`
	// object which contains main information about project
	Project *SimpleItem `json:"project"`
	// user account in project
	Account string `json:"account"`
	// unique order identifier in project
	ProjectOrderId *string `json:"order_id,omitempty"`
	// data about payer, for example: country, city, ip and etc
	PayerData *PayerData `json:"payer_data"`
	// object which contains main information about payment method
	PaymentMethod *SimpleItem `json:"payment_method"`
	// object which contains main information about technical finances of project which received of project
	ProjectTechnicalIncome *OrderSimpleAmountObject `json:"project_technical_income,omitempty"`
	// object which contains main information about technical finances of project which will send to project
	ProjectTechnicalOutcome *OrderSimpleAmountObject `json:"project_technical_outcome,omitempty"`
	// object which contains main information about technical finances of payment system which received of payment system
	PaymentSystemTechnicalIncome *OrderSimpleAmountObject `json:"ps_technical_income,omitempty"`
	// object which contains main information about accounting finances of project which received of project
	ProjectAccountingIncome *OrderSimpleAmountObject `json:"project_accounting_income,omitempty"`
	// object which contains main information about accounting finances of project which will send to project
	ProjectAccountingOutcome *OrderSimpleAmountObject `json:"project_accounting_outcome,omitempty"`
	// object which contains main information about fixed package which was buy
	FixedPackage *OrderFixedPackage `json:"fixed_package"`
	// object which contains main information about payment status
	Status *Status `json:"status"`
	// date when payment created
	CreatedAt time.Time `json:"created_at"`
	// date when payment was confirmed from payment system side
	ConfirmedAt *time.Time `json:"confirmed_at"`
	// date when project was notification about payment
	ClosedAt *time.Time `json:"confirmed_at"`
}

type OrderPaginate struct {
	// total count of selected orders
	Count int `json:"count"`
	// array of selected orders
	Items []*OrderSimple `json:"items"`
}
