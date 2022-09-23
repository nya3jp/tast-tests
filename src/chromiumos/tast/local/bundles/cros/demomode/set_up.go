// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package demomode

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/demomode"
	"chromiumos/tast/local/demomode/fixture"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SetUp,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Basic test that clicks through Demo Mode setup from OOBE",
		Contacts:     []string{"cros-demo-mode-eng@google.com"},
		// If DUT ran other tests before current one, it could have logged into a managedchrome.com account.
		// This would place a domain lock on the device and prevent it from entering demo mode (cros-demo-mode.com).
		// The solution is to reset TPM before trying to enter demo mode.
		Fixture: fixture.TPMReset,
		Attr:    []string{"group:mainline", "informational"},
		// Demo Mode uses Zero Touch Enrollment for enterprise enrollment, which
		// requires a real TPM.
		// We require "arc" and "chrome_internal" because the ARC TOS screen
		// is only shown for chrome-branded builds when the device is ARC-capable.
		SoftwareDeps: []string{"chrome", "chrome_internal", "arc", "tpm"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

// SetUp runs through the basic flow of entering Demo Mode from OOBE
//
// TODO(b/231472901): Deduplicate the shared code between Demo Mode and normal
// OOBE Tast tests
func SetUp(ctx context.Context, s *testing.State) {
	demoCtx := s.FixtValue().(*demomode.Context)
	tconn := demoCtx.Tconn

	ui := uiauto.New(tconn).WithTimeout(50 * time.Second)

	s.Log("Waiting for Highlights App")
	highlightsWindow := nodewith.Name("Google Retail Chromebook").First()
	if err := ui.WaitUntilExists(highlightsWindow)(ctx); err != nil {
		s.Fatal("Failed to wait until Highlights App exists: ", err)
	}
}
