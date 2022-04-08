// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hostapd

import (
	"context"
)

// Conf is an interface for a specific hostapd configuration.
type Conf interface {
	// Prepare creates all the hostapd files required by the current
	// configuration in dir. Returns the path to the main configuration
	// file or an error.
	Prepare(ctx context.Context, dir, ctrlPath string) (string, error)
}
