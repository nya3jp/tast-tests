// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"
)

// Port represends a serial port and its basic operations.
type Port interface {
	// Read bytes into buffer and return number of bytes read.
	// Bytes already written to the port shall be moved into buf, up to its size.
	Read(ctx context.Context, buf []byte) (n int, err error)
	// Write bytes from buffer and return number of bytes written.
	// It returns a non-nil error when n != len(b), nil otherwise.
	Write(ctx context.Context, buf []byte) (n int, err error)
	// Flush un-read/written bytes.
	Flush(ctx context.Context) error
	// Close closes the port.
	Close(ctx context.Context) error
}
