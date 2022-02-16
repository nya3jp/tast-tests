// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"io/ioutil"
	"path/filepath"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UIFaillog,
		Desc:         "Check if faillog for the UI tree works",
		Contacts:     []string{"hidehiko@chromium.org", "tast-owners@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func UIFaillog(ctx context.Context, s *testing.State) {
	// To make sure brand new Chrome instance, do not use fixture.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to log in ash-chrome: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get tconn: ", err)
	}

	s.Log("Dump the UI tree to ui_dump.txt")
	filePath := filepath.Join(s.OutDir(), "ui_dump.txt")
	if err := uiauto.LogRootDebugInfo(ctx, tconn, filePath); err != nil {
		s.Fatal("Failed to dump: ", err)
	}

	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		s.Fatal("Failed to read ui_dump file: ", err)
	}

	if len(b) == 0 {
		s.Fatal("Dump file is unexpectedly empty")
	}
}
