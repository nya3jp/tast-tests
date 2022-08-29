// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WindowAnimationJank,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Sample test to run ArcWindowAnimationJankTest.apk",
		Contacts:     []string{"khmel@chromium.org", "skuhne@chromium.org", "arc-performance@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Data:         []string{"ArcWindowAnimationJankTest.apk", "config.pbtxt"},
		Timeout:      30 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func WindowAnimationJank(ctx context.Context, s *testing.State) {

	const (
		apkName      = "ArcWindowAnimationJankTest.apk"
		pkgName      = "org.chromium.arc.testapp.windowanimationjank"
		activityName = "ElementLayoutActivity"
	)
	a := s.FixtValue().(*arc.PreData).ARC
	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Installing: ", apkName)
	if err := a.Install(ctx, s.DataPath(apkName)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	s.Log("Running test")

	s.Log("Starting app")

	act, err := arc.NewActivity(a, pkgName, "."+activityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}

	if err := act.StartWithDefaultOptions(ctx, tconn); err != nil {
		s.Fatal("Failed to start the BlackFlashTest activity: ", err)
	}

}
