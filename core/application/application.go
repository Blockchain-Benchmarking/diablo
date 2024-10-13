package application

// Applications keeps track of each application type's initializer
var Applications = map[string]func(appParams map[string]interface{}, blockchainParams map[string]interface{}) (Application, error){
	"payment": CreatePaymentApp,
}

type Application interface {
	Execute(parameters map[string]interface{}) error
}
