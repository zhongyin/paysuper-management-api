package dao

import (
	"github.com/globalsign/mgo/bson"
	"github.com/paysuper/paysuper-billing-server/pkg/proto/billing"
	"github.com/paysuper/paysuper-management-api/database/model"
)

type Repository interface {
	FindCurrencyById(int) (*model.Currency, error)
	FindCurrenciesByName(string) ([]*model.Currency, error)
	FindAllCurrencies(int32, int32) ([]*model.Currency, error)
	FindCurrencyByCodeA3(string) (*model.Currency, error)

	FindCountryById(int) (*model.Country, error)
	FindCountryByName(string) ([]*model.Country, error)
	FindAllCountries(int32, int32) (*model.CountryItems, error)

	FindMerchantById(id string) (*billing.Merchant, error)
	InsertMerchant(m *model.Merchant) error
	UpdateMerchant(m *billing.Merchant) error

	InsertProject(p *model.Project) error
	UpdateProject(p *model.Project) error
	FindProjectsByMerchantId(string, int32, int32) ([]*billing.Project, error)
	FindProjectByMerchantIdAndName(bson.ObjectId, string) (*model.Project, error)
	FindProjectById(bson.ObjectId) (*model.Project, error)
	FindFixedPackageByFilters(filters *model.FixedPackageFilters) ([]map[string]interface{}, error)

	FindPaymentSystemById(bson.ObjectId) (*model.PaymentSystem, error)
	FindAllPaymentSystem() ([]*model.PaymentSystem, error)

	FindPaymentMethodById(bson.ObjectId) (*model.PaymentMethod, error)
	FindAllPaymentMethods() ([]*model.PaymentMethod, error)
	FindPaymentMethodsByIds([]bson.ObjectId) ([]*model.PaymentMethod, error)

	FindOrderByProjectOrderId(string) (*model.Order, error)
	FindOrderById(bson.ObjectId) (*model.Order, error)
	FindAllOrders(filters bson.M, sort []string, limit int32, offset int32) ([]*model.Order, error)
	GetOrdersCountByConditions(filters bson.M) (int, error)
	GetRevenueDynamic(*model.RevenueDynamicRequest) ([]map[string]interface{}, error)
	GetAccountingPayment(rdr *model.RevenueDynamicRequest, mId string) ([]map[string]interface{}, error)
	InsertOrder(*model.Order) error
	UpdateOrder(*model.Order) error

	FindCurrenciesPair(int, int) (*model.CurrencyRate, error)

	FindCommissionByProjectIdAndPaymentMethodId(projectId bson.ObjectId, pmId bson.ObjectId) (*model.Commission, error)

	FindVatByCountry(string) (*model.Vat, error)
	FindVatByCountryAndSubdivision(string, string) (*model.Vat, error)

	InsertLog(*model.Log) error
}
