// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"

	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SWDeps,
		Desc:         "Demonstration of software deps feature",
		Contacts:     []string{"seewaifu@chromium.org", "tast-owners@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"brya"},
	})
}

func SWDeps(ctx context.Context, s *testing.State) {
	// No errors means the test passed.
	// This test should run only on eve models.
}
