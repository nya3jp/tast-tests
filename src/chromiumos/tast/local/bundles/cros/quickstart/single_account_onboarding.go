// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package quickstart contains tests for the Quick Start feature in ChromeOS.
package quickstart

import (
	"context"

	"chromiumos/tast/local/chrome/crossdevice"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SingleAccountOnboarding,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Prototype test description",
		Contacts: []string{
			"jasonrhee@chromium.org",
		},
		Attr:         []string{"group:cross-device"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "crossdeviceNoSignIn",
	})
}

func SingleAccountOnboarding(ctx context.Context, s *testing.State) {
	tconn := s.FixtValue().(*crossdevice.FixtData).TestConn
	if tconn == nil {
		s.Fatal("test connection is empty")
	}
	androidDevice := s.FixtValue().(*crossdevice.FixtData).AndroidDevice
	if androidDevice == nil {
		s.Fatal("fixture not associated with an android device")
	}
}
