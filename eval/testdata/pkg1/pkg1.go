package pkg1

import "io"

type Reader struct {
	io.LimitedReader
}
