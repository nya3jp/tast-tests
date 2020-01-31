// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Density,
		Desc:         "Checks that density can be charged with Android applications",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_both", "chrome"},
		Data:         []string{densityApk},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

const (
	densityApk     = "ArcDensityTest.apk"
	densitySetting = "persist.sys.enable_application_zoom"
	packageName    = "org.chromium.arc.testapp.densitytest"
	cls            = ".DensityActivity"
	setprop        = "/system/bin/setprop"
)

type densityChange struct {
	action      string
	keySequence string
	scaleFactor int
}

func performAndConfirmDensityChange(ctx context.Context, ew *input.KeyboardEventWriter, act *arc.Activity, test densityChange) error {
	testing.ContextLog(ctx, test.action+" density using key "+test.keySequence)
	if err := ew.Accel(ctx, test.keySequence); err != nil {
		return errors.Wrapf(err, "could not %s scale factor", test.keySequence)
	}
	if gotScaleFactor, err := act.ScaleFactor(ctx); err != nil {
		return errors.Wrap(err, "failed to get scale factor")
	} else if gotScaleFactor != test.scaleFactor {
		return errors.Errorf("scale factor incorrect, expected: %d; got: %d", test.scaleFactor, gotScaleFactor)
	}
	return nil
}

func Density(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	if err := arc.BootstrapCommand(ctx, setprop, densitySetting, "true").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set developer option: ", err)
	}
	defer arc.BootstrapCommand(ctx, setprop, densitySetting, "false").Run(testexec.DumpLogOnError)

	testing.ContextLog(ctx, "Installing app")
	if err := a.Install(ctx, s.DataPath(densityApk)); err != nil {
		s.Fatal("Failed to install app: ", "app sanity")
	}
	act, err := arc.NewActivity(a, packageName, ".MainActivity")
	if err != nil {
		s.Fatal("Failed to create new activity")
	}
	defer act.Close()

	testing.ContextLog(ctx, "Starting activity")
	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed to start the  activity: ", err)
	}

	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Error creating keyboard: ", err)
	}
	defer ew.Close()

	if scaleFactor, err := act.ScaleFactor(ctx); err != nil {
		s.Fatal("Failed to get scale factor: ", err)
	} else if scaleFactor != 0 {
		s.Fatalf("Incorrect initial scale factor, expected: 0, got: %d", scaleFactor)
	}

	for _, test := range []densityChange{
		{
			"increase",
			"ctrl+=",
			1,
		},
		{
			"reset",
			"ctrl+0",
			0,
		},
		{
			"decrease",
			"ctrl+-",
			-1,
		},
	} {
		if err := performAndConfirmDensityChange(ctx, ew, act, test); err != nil {
			s.Fatal("Error with performing action: ", err)
		}
	}
}
