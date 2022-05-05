// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lurs

import (
	"context"
	"strings"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/local/chrome"
	hwseclocal "chromiumos/tast/local/hwsec"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: KeyEscrow, LacrosStatus: testing.LacrosVariantUnneeded, Desc: "KeyEscrow checks if key escrow is enabled",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"chromeos-commercial-identity@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.FakeDMS,
	})
}

// KeyEscrow checks if key escrow is enabled.
func KeyEscrow(ctx context.Context, s *testing.State) {
	const escrowKeyLabel = "test" // ToDo: Get correct label for the escrow key, maybe store it as secret variable.

	// ToDo: activate key escrow.

	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
	)
	if err != nil {
		s.Fatal("Creating Chrome with deferred login failed: ", err)
	}
	defer cr.Close(ctx)

	cmdRunner := hwseclocal.NewLoglessCmdRunner()
	cryptohome := hwsec.NewCryptohomeClient(cmdRunner)
	keys, err := cryptohome.ListVaultKeys(ctx, fixtures.Username)
	if err != nil {
		s.Fatalf("Failed to list vault keys for %s", fixtures.Username)
	}

	foundKey := false
	for _, key := range keys {
		if strings.Contains(key, escrow_key_label) {
			foundKey = true
			break
		}
	}

	if !foundKey {
		s.Fatal("Escrow key not found in cryptohome")
	}
}
