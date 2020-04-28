// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ash

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// Accelerator key used to trigger system tray opening.
const (
	AccelShowTray Accelerator = "{keyCode: 's', shift: true, control: false, alt: true, search: false, pressed: true}"
)

// ShowSystemTray will cause the system tray bubble to open via accelerator.
func ShowSystemTray(ctx context.Context, tconn *chrome.TestConn) error {
	expr := fmt.Sprintf(
		`(async () => {
                   var acceleratorKey=%s;
                   // Send the press event to store it in the history. It'll not be handled, so ignore the result.
                   chrome.autotestPrivate.activateAccelerator(acceleratorKey, () => {});
                   acceleratorKey.pressed = false;
                   await tast.promisify(chrome.autotestPrivate.activateAccelerator)(acceleratorKey);
                 })()`, AccelShowTray)

	if err := tconn.EvalPromise(ctx, expr, nil); err != nil {
		return errors.Wrap(err, "failed to execute accelerator")
	}
	return nil
}
