package workload

import (
	"diablo/core/user"
	"io"
)

type Workload interface {
	Encode(dest io.Writer) error
	Decode(src io.Reader) error
}

type PaymentWorkload struct {
	Users []user.User
}
