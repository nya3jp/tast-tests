// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/bundles/cros/ui/filesapp"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FilesApp,
		Desc:         "Basic smoke test for Files app",
		Contacts:     []string{"bhansknecht@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
	})
}

func FilesApp(ctx context.Context, s *testing.State) {
	filesapp.RunTest(ctx, s, s.PreValue().(*chrome.Chrome), false)
}
