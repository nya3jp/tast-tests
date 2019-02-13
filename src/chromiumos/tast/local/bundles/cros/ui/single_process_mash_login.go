// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
		Func:         SingleProcessMashLogin,
		Desc:         "Checks that chrome --enable-features=SingleProcessMash starts",
		Contacts:     []string{"jamescook@chromium.org"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

// SingleProcessMashLogin checks that chrome --enable-features=SingleProcessMash starts normally.
func SingleProcessMashLogin(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=SingleProcessMash"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	cr.Close(ctx)
}
