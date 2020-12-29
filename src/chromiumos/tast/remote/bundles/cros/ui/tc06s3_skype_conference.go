// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"chromiumos/tast/remote/bundles/cros/ui/conference"
	"chromiumos/tast/remote/bundles/cros/ui/pre"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TC06S3SkypeConference,
		Desc:         "Create Skype video conference, and present slide to another user",
		Contacts:     []string{"aaboagye@chromium.org"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		ServiceDeps: []string{
			"tast.cros.ui.SkypeService",
			"tast.cros.cuj.LocalStoreService",
		},
		Pre:  pre.LocalStore(),
		Vars: []string{"ui.cuj_username", "ui.cuj_password", "ui.cuj_username_2", "ui.cuj_password_2"},
	})
}

func TC06S3SkypeConference(ctx context.Context, s *testing.State) {
	conference.Run(ctx, s, conference.Skype)
}
