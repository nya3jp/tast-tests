// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/adb/ui"
)

// Hard-coded IP addresses of ARC.
const ipAddr = "100.115.92.2"

// NewUIDevice creates a Device object by starting and connecting to UI Automator server.
// Close must be called to clean up resources when a test is over.
func (a *ARC) NewUIDevice(ctx context.Context) (*ui.Device, error) {
	return ui.NewDevice(ctx, a.device, ipAddr)
}
