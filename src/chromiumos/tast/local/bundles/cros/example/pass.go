// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Pass,
		Desc:     "Always passes",
		Contacts: []string{"derat@chromium.org", "tast-users@chromium.org"},
	})
}

func Pass(ctx context.Context, s *testing.State) {
	// No errors means the test passed.
}
