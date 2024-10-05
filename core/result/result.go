package result

import "io"

type Result interface {
	PrintResult(dest io.Writer)
	PrintStat()
}
