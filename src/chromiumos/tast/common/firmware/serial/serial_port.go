package serial

import (
	"context"
)

type Port interface {
	Read(ctx context.Context, b []byte) (n int, err error)
	Write(ctx context.Context, b []byte) (n int, err error)
	Flush(ctx context.Context) error
	Close(ctx context.Context) error
}
