package workload

import "io"

type Workload interface {
	Encode(dest io.Writer) error
	Decode(src io.Reader) error
}

type PaymentWorkload struct {
	Users []User
}
