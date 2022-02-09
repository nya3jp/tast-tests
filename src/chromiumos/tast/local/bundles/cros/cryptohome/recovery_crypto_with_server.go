// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cryptohome

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RecoveryCryptoWithServer,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test signout from the lock screen",
		Contacts:     []string{""},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
	})
}

type CryptohomeRecoveryData struct {
	ReauthProofToken string `json:"reauthProofToken"`
	AccessToken      string `json:"accessToken"`
}

func RecoveryCryptoWithServer(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.DontSkipOOBEAfterLogin(),
		chrome.EnableFeatures("CryptohomeRecoveryFlow"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	var data CryptohomeRecoveryData
	if err := tconn.Call(ctx, &data, "tast.promisify(chrome.autotestPrivate.getCryptohomeRecoveryData)"); err != nil {
		s.Fatal("error: ", err)
	}
	s.Log("ReAuthProofToken: ", data.ReauthProofToken)
	s.Log("AccessToken: ", data.AccessToken)
}
