package core

import (
	"diablo/workload/behavior"
	"diablo/workload/payment"
)

var Users = map[string]behavior.User{
	payment.STUBBORN_PAYMENT_USER: &payment.StubbornPaymentUser{},
}

var Results = map[string]behavior.Result{
	behavior.StubbornResult: &behavior.StubbornAction{},
}
