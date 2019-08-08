// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     VarsSkip,
		Desc:     "Test skipped if a var is not defined",
		Contacts: []string{"tast-owners@google.com"},
		Attr:     []string{"informational"},
		Vars:     []string{"example.VarsSkip.nonexistent"},
	})
}

func VarsSkip(ctx context.Context, s *testing.State) {
	s.Error("Test unexpectedly run")
}
