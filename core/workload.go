package core

import (
	"diablo/workload/behavior"
	"diablo/workload/payment"
)

var Users = map[string]behavior.User{
	payment.STUBBORN_PAYMENT_USER: &payment.StubbornPaymentUser{},
}

var Schedules = map[string]behavior.Schedule{
	behavior.ScheduleInteractionsName: &behavior.ScheduleInteractions{},
	behavior.ScheduleRatesName:        &behavior.ScheduleRates{},
}

var Results = map[string]behavior.Result{
	behavior.StubbornResult: &behavior.StubbornAction{},
}
