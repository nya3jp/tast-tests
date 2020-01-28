// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/local/chrome"
)

// Refresh takes a running Chrome API connection and refreshes policies.
// If the policices served have changed between now and the last time policies
// were fetched, this function will ensure that Chrome uses the changes.
// Note that this will not work for policies which require a reboot before a
// change is applied.
func Refresh(ctx context.Context, tconn *chrome.Conn) error {
	const cmd = `tast.promisify(chrome.autotestPrivate.refreshEnterprisePolicies)();`
	return tconn.EvalPromise(ctx, cmd, nil)
}
