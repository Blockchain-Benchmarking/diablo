package coordinator

import "diablo/core/user"

var coordinators = map[string]interface{}{
	"simple": &SimpleCoordinator{},
}

type Coordinator interface {
	SendWorkload() error
	CollectResults() user.Results
}
