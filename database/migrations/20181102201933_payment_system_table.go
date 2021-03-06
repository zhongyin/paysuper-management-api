package migrations

import (
	"github.com/globalsign/mgo"
	"github.com/paysuper/paysuper-management-api/manager"
	"github.com/xakep666/mongo-migrate"
)

func init() {
	err := migrate.Register(
		func(db *mgo.Database) error {
			c := db.C(manager.TablePaymentSystem)

			return c.EnsureIndex(mgo.Index{Name: "county_is_active_idx", Key: []string{"is_active"}})
		},
		func(db *mgo.Database) error {
			return db.C(manager.TablePaymentSystem).DropCollection()
		},
	)

	if err != nil {
		return
	}
}
