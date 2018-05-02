// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ARCStartup,
		Desc:         "Checks that ARC starts",
		Attr:         []string{"arc", "bvt", "chrome"},
		SoftwareDeps: []string{"android", "chrome_login"},
	})
}

func ARCStartup(s *testing.State) {
	cr, err := chrome.New(s.Context(), chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(s.Context())

	// TODO(derat): Do more to test that ARC is working.
}
