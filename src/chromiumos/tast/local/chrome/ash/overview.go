// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// SetOverviewModeAndWait requests Ash to set the overview mode state and waits
// for its animation to complete.
func SetOverviewModeAndWait(ctx context.Context, tconn *chrome.TestConn, inOverview bool) error {
	expr := fmt.Sprintf(`tast.promisify(chrome.autotestPrivate.setOverviewModeState)(%v)`, inOverview)
	finished := false
	if err := tconn.EvalPromise(ctx, expr, &finished); err != nil {
		return err
	}
	if !finished {
		return errors.New("the overview mode animation is canceled")
	}
	return nil
}
