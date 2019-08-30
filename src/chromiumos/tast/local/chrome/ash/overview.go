// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
)

// SetOverviewModeAndWait requests Ash to set the overview mode state and wait
// for its animation to complete.
func SetOverviewModeAndWait(ctx context.Context, tconn *chrome.Conn, inOverview bool) error {
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.setOverviewModeState)(%v)`, inOverview)
	return tconn.EvalPromise(ctx, expr, nil)
}
