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
		Func: UserFilesLoggedIn,
		Desc: "Checks ownership and permissions of user files for logged-in users",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"chromeos-security@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoggedIn(),
		Attr:         []string{"group:mainline"},
	})
}

func UserFilesLoggedIn(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)
	userfiles.Check(ctx, s, cr.NormalizedUser())
}
