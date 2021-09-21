// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ti50

import (
	"context"
	"regexp"
)

// DevBoard is the generic interface for development boards.
type DevBoard interface {
	// ReadSerialSubmatch reads output from port until regex is matched.
	ReadSerialSubmatch(ctx context.Context, re *regexp.Regexp) (output [][]byte, err error)
	// WriteSerial writes to port.
	WriteSerial(ctx context.Context, bytes []byte) error
	// FlushSerial flushes un-read/written chars from port.
	FlushSerial(ctx context.Context) error
	// FlashImage flashes image on DevBoard.
	FlashImage(ctx context.Context, imagePath string) error
	// Reset the DevBoard.
	Reset(ctx context.Context) error
	// Close and cleanup.
	Close(ctx context.Context) error
}
