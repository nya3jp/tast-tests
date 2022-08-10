// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package enterprisecuj

import (
	"context"

	cx "chromiumos/tast/local/bundles/cros/spera/enterprisecuj/citrix"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
)

// CitrixScenario contains actions when a user enters a citrix workplace.
type CitrixScenario interface {
	Run(ctx context.Context, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, citrix *cx.Citrix, p *TestParams) error
}
