package generator

import (
	"diablo/core/user"
)

type Generator interface {
	Start() error
	CollectResults() (user.Results, error)
}
