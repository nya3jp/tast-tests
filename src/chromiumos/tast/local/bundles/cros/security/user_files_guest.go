// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"

	"chromiumos/tast/local/bundles/cros/security/userfiles"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mountns"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UserFilesGuest,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks ownership and permissions of files for guest users",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
	})
}

func UserFilesGuest(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.GuestLogin())
	if err != nil {
		s.Fatal("Login failed: ", err)
	}
	defer cr.Close(ctx)

	// Guest sessions can be mounted in a non-root mount namespace
	// so the test needs to perform checks in that same namespace.
	if err := mountns.EnterUserSessionMountNs(ctx); err != nil {
		s.Fatal("Failed to enter user session namespace: ", err)
	}
	defer mountns.EnterInitMountNs(ctx)

	userfiles.Check(ctx, s, cr.NormalizedUser())
}
