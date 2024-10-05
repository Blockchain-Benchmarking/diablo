package workload

const PaymentAppName = "payment"

var Applications = []Application{
	{
		name: PaymentAppName,
		workloads: map[string][]string{
			"simple": {"payment"}, // Use shorthand syntax for slice initialization
		},
	},
}

type Application struct {
	name      string
	workloads map[string][]string
}
