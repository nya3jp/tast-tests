// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/saml"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChromeSAML,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that Chrome can make real SAML logins",
		Contacts: []string{
			"lmasopust@google.com",
			"cros-3pidp@google.com",
		},
		SoftwareDeps: []string{
			"chrome",
			"chrome_internal",
		},
		Attr: []string{
			"group:mainline", "informational",
		},
		VarDeps: []string{
			"accountmanager.samlusername",
			"accountmanager.samlpassword",
			"ui.signinProfileTestExtensionManifestKey",
		},
		Timeout: chrome.GAIALoginTimeout + time.Minute,
	})
}

func ChromeSAML(ctx context.Context, s *testing.State) {
	username := s.RequiredVar("accountmanager.samlusername")
	password := s.RequiredVar("accountmanager.samlpassword")

	cr, err := saml.LoginWithSAMLAccount(
		ctx,
		username,
		saml.HandleMicrosoftLogin(username, password),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Chrome SAML login failed: ", err)
	}
	defer cr.Close(ctx)
}
