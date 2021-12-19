package terminal

import "io"

type Terminal struct {
	io.ReadWriter

	Width, Height uint32
}
