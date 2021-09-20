// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package serial

import (
	"context"
)

// PortOpener opens the serial port.
type PortOpener interface {
	OpenPort(context.Context) (Port, error)
}
