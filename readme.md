[![Build Status](https://travis-ci.org/paysuper/paysuper-management-api.svg?branch=master)](https://travis-ci.org/paysuper/paysuper-management-api.api)
[![codecov](https://codecov.io/gh/paysuper/paysuper-management-api/branch/master/graph/badge.svg)](https://codecov.io/gh/paysuper/paysuper-management-api)
[![Go Report Card](https://goreportcard.com/badge/github.com/paysuper/paysuper-management-api)](https://goreportcard.com/report/github.com/paysuper/paysuper-management-api)

The documentation under construction

### Accounting currency

1. PSP Currency - used to save the amount of the payment transaction in the PSP accounting currency. PSP currency can 
be set using the environment variable named "PSP_ACCOUNTING_CURRENCY"

2. Merchant currency -  used to save the amount of the payment transaction in the merchant (projects owner) accounting 
currency. Merchant currency can be set using merchant settings in PSP control panel.

3. Payment system currency - used to save the amount of the payment transaction in the payment system (payment methods 
owner) accounting currency. Payment system currency can be set using payment system settings in PSP admin panel.
