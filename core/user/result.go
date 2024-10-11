package user

import "io"

type Results interface {
	Encode(dest io.Writer) error
	Decode(src io.Reader) error
	PrintResult(dest io.Writer) error
}
