// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SingleProcessMashLogin,
		Desc:         "Checks that chrome --enable-features=SingleProcessMash starts",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

// SingleProcessMashLogin checks that chrome --enable-features=SingleProcessMash starts normally.
func SingleProcessMashLogin(s *testing.State) {
	cr, err := chrome.New(s.Context(), chrome.ExtraArgs([]string{"--enable-features=SingleProcessMash"}))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	cr.Close(s.Context())
}
