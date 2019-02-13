// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MashLogin,
		Desc:     "Checks that chrome --enable-features=Mash starts",
		Contacts: []string{"jamescook@chromium.org"},
		Attr:     []string{"informational"},
		// Skipped on nyan due to flaky crashes. https://crbug.com/717275
		SoftwareDeps: []string{"chrome_login", "stable_egl"},
	})
}

// MashLogin checks that chrome --enable-features=Mash starts and at least one mash service is running.
func MashLogin(ctx context.Context, s *testing.State) {
	// Mash and SingleProcessMash are mutually exclusive. Ensure SingleProcessMash is disabled,
	// even if it is on-by-default.
	cr, err := chrome.New(ctx,
		chrome.ExtraArgs("--enable-features=Mash", "--disable-features=SingleProcessMash"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// TODO(jamescook): Check that a mash process is running. The test used
	// to do this (see git history) but we had to stop due to flake from
	// chrome's command line sometimes being truncated.
	// https://crbug.com/891470
}
