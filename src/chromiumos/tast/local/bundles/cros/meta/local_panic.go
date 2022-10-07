// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LocalPanic,
		Desc:         "Helper test that panics",
		Contacts:     []string{"tast-owners@google.com", "nya@chromium.org"},
		BugComponent: "b:1034625",
		// This test is called by remote tests in the meta package.
	})
}

func LocalPanic(ctx context.Context, s *testing.State) {
	panic("intentionally panicking")
}
