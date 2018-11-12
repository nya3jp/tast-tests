// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/bundles/cros/security/userfiles"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UserFilesGuest,
		Desc:         "Checks ownership and permissions of files for guest users",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

func UserFilesGuest(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.GuestLogin())
	if err != nil {
		s.Fatal("Login failed: ", err)
	}
	defer cr.Close(ctx)

	userfiles.Check(ctx, s, cr.User())
}
