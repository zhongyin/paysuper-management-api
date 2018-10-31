package api

import (
	"github.com/ProtocolONE/p1pay.api/database/model"
	"gopkg.in/go-playground/validator.v9"
)

func ProjectStructValidator(sl validator.StructLevel) {
	p := sl.Current().Interface().(model.ProjectScalar)

	if p.SendNotifyEmail == true && len(p.NotifyEmails) <= 0 {
		sl.ReportError(p.NotifyEmails, "NotifyEmails", "notify_emails", "notify_emails", "")
	}

	if p.OnlyFixedAmounts != true && p.FixedPackage != nil {
		var counter int

		for _, packages := range *p.FixedPackage {
			counter += len(packages)
		}

		if counter > 0 {
			sl.ReportError(p.FixedPackage, "FixedPackage", "fixed_package", "fixed_package", "")
		}
	}
}
