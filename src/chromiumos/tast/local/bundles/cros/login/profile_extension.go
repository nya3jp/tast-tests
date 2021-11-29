// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProfileExtension,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check private signin profile extension loads",
		Contacts:     []string{"cros-oac@google.com", "chromeos-sw-engprod@google.com"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		Attr:         []string{"group:mainline"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func ProfileExtension(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	if _, err := cr.SigninProfileTestAPIConn(ctx); err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
}
