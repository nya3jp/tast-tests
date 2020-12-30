// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package peripheraltypes contains common types used in multiple peripherals tests.
package peripheraltypes

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// WaitForAppFn is the waitForApp function from the relevant UI driver.
type WaitForAppFn func(ctx context.Context, tconn *chrome.TestConn) error
