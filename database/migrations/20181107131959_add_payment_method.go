package migrations

import (
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/paysuper/paysuper-management-api/database/model"
	"github.com/paysuper/paysuper-management-api/manager"
	"github.com/xakep666/mongo-migrate"
	"time"
)

func init() {
	err := migrate.Register(
		func(db *mgo.Database) error {
			cr := &model.Currency{}
			if err := db.C(manager.TableCurrency).Find(bson.M{"code_a3": "EUR"}).One(&cr); err != nil {
				return err
			}

			ps := &model.PaymentSystem{}
			if err := db.C(manager.TablePaymentSystem).Find(bson.M{"name": "CardPay"}).One(ps); err != nil {
				return err
			}

			pm := &model.PaymentMethod{
				Id:               bson.NewObjectId(),
				Name:             "Bank card",
				PaymentSystem:    ps,
				GroupAlias:       "bank_card",
				MinPaymentAmount: 0.01,
				MaxPaymentAmount: 15000.00,
				IsActive:         true,
				CreatedAt:        time.Now(),
				UpdatedAt:        time.Now(),
			}

			return db.C(manager.TablePaymentMethod).Insert(pm)
		},
		func(db *mgo.Database) error {
			return db.C(manager.TablePaymentSystem).Remove(bson.M{"group_alias": "bank_card"})
		},
	)

	if err != nil {
		return
	}
}
