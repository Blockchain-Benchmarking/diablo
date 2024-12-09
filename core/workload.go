package core

import (
	"diablo/workload/behavior"
	"diablo/workload/payment"
	"diablo/workload/store"
)

var Users = map[string]behavior.User{
	payment.STUBBORN_PAYMENT_USER: &payment.StubbornPaymentUser{},
	store.STUBBORN_STORE_USER:     &store.StubbornStoreUser{},
}

var Results = map[string]behavior.Result{
	behavior.StubbornResult: &behavior.StubbornAction{},
}
