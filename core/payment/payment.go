package payment

import (
	"diablo/core/behavior"
	"time"
)

var PaymentApplications = map[string]func(config behavior.Config) (PaymentApplication, error){
	"ethereum": NewEthereumPaymentApplication,
}

type UserTools struct {
	name   string
	create func(duration time.Duration, appName string, config behavior.Config, params map[string]interface{}) (behavior.User, error)
}

type PaymentApplication interface {
	Pay(to string, amount float64, timeout time.Duration) error
	Balance() (float64, error)
	GetOthers() []string
}
