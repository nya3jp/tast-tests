package serial

import (
	"context"
)

type PortOpener interface {
	OpenPort(context.Context) (Port, error)
}
