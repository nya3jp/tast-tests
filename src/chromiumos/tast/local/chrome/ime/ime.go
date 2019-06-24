// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// The IME package
package ime

import (
	"context"
	"fmt"

	"chromiumos/tast/local/chrome"
)

// SetCurrentInputMethod sets the current input method used by both the virtual
// and physical keyboard.
// TODO(keithlee) - chrome.autotestPrivate.setWhitelistedPref seems to
// not be synchronous. The promise returns before the input engine has been set
// Here, we evaluate promises and get a result
func SetCurrentInputMethod(ctx context.Context, tconn *chrome.Conn, inputMethod string) error {
	return tconn.EvalPromise(ctx, fmt.Sprintf(`
new Promise((resolve, reject) => {
	chrome.autotestPrivate.setWhitelistedPref(
		'settings.language.preload_engines', %[1]q, () => {
			chrome.inputMethodPrivate.setCurrentInputMethod(%[1]q, () => {
				if (chrome.runtime.lastError) {
					reject(chrome.runtime.lastError.message);
				} else {
					resolve();
				}
			});
		}
	);
})
`, inputMethod), nil)
}
