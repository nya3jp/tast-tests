// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: GameAutoHideShelf,
		Desc: "Tests shelf behavior after launching an ARC game",
		Contacts: []string{
			"yulunwu@chromium.org",
			"tbarzic@chromium.org",
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"com.halfbrick.fruitninja.apk"},
		Timeout:      60 * time.Second,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

// GameAutoHideShelf tests that the shelf auto-hides when an ARC game like fruitninja starts up.
func GameAutoHideShelf(ctx context.Context, s *testing.State) {
	const (
		apk     = "com.halfbrick.fruitninja.apk"
		pkgName = "com.halfbrick.fruitninjafree"
		actName = "com.google.firebase.MessagingUnityPlayerActivity"
	)

	p := s.FixtValue().(*arc.PreData)
	a := p.ARC
	cr := p.Chrome

	s.Log("Creating Test API connection")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Installing app")
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed installing app: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, false)
	if err != nil {
		s.Fatal("Failed to enter clamshell mode: ", err)
	}
	defer cleanup(ctx)

	act, err := arc.NewActivity(a, pkgName, actName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Logf("Starting activity: %s/%s", pkgName, actName)
	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed start activity: ", err)
	}

	if err := ash.WaitForHotseatToUpdateAutoHideState(ctx, tconn, true); err != nil {
		s.Fatal("Shelf should be hidden when launching an ARC game: ", err)
	}
}
