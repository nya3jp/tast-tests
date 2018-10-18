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
		Func:         MashLogin,
		Desc:         "Checks that chrome --enable-features=Mash starts",
		SoftwareDeps: []string{"chrome_login"},
	})
}

// MashLogin checks that chrome --enable-features=Mash starts and at least one mash service is running.
func MashLogin(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs([]string{"--enable-features=Mash"}))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	// TODO(jamescook): Check that a mash process is running. The test used
	// to do this (see git history) but we had to stop due to flake from
	// chrome's command line sometimes being truncated.
	// https://crbug.com/891470
}
