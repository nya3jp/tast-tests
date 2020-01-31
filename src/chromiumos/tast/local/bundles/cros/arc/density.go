// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"regexp"
	"strconv"
	"strings"
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
	densityApk = "ArcDensityTest.apk"
)

type densityChange struct {
	action      string
	keySequence string
	scaleFactor int
}

// getScaleFactor runs `dumpsys activity activities` and parses the output to obtain the current scale factor from
// TaskRecordArc.
func getScaleFactor(ctx context.Context, a *arc.ARC) (int, error) {
	var scaleFactor int
	if output, err := a.DumpsysActivity(ctx); err != nil {
		return scaleFactor, err
	} else if matchForScaleFactor := regexp.MustCompile(`scaled-x(-)?\d`).FindStringSubmatch(output); matchForScaleFactor != nil {
		scaleFactor, err = strconv.Atoi(strings.Replace(matchForScaleFactor[0], "scaled-x", "", 1))
		if err != nil {
			return scaleFactor, errors.Wrap(err, "could not parse scaleFactor")
		}
		return scaleFactor, nil
	}
	// If scaled-x is not found, then its value is 0.
	// TODO(sarakato): add logging for when scale factor is 0 to TaskRecordArc.
	return 0, nil
}

// performAndConfirmDensityChange changes the density of the activity, and confirms that the density was changed by checking
// the scale factor.
func performAndConfirmDensityChange(ctx context.Context, ew *input.KeyboardEventWriter, a *arc.ARC, test densityChange) error {
	testing.ContextLog(ctx, test.action+" density using key "+test.keySequence)
	if err := ew.Accel(ctx, test.keySequence); err != nil {
		return errors.Wrapf(err, "could not %s scale factor", test.keySequence)
	}

	// If key press was successful, confirm the current scale factor.
	if gotScaleFactor, err := getScaleFactor(ctx, a); err != nil {
		return err
	} else if gotScaleFactor != test.scaleFactor {
		return errors.Errorf("scale factor incorrect, got: %d, want: %d", gotScaleFactor, test.scaleFactor)
	}
	return nil
}

func Density(ctx context.Context, s *testing.State) {
	const (
		setprop        = "/system/bin/setprop"
		mainActivity   = ".MainActivity"
		packageName    = "org.chromium.arc.testapp.densitytest"
		densitySetting = "persist.sys.enable_application_zoom"
	)
	a := s.PreValue().(arc.PreData).ARC

	if err := a.Command(ctx, setprop, densitySetting, "true").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to set developer option: ", err)
	}
	defer a.Command(ctx, setprop, densitySetting, "false").Run(testexec.DumpLogOnError)

	testing.ContextLog(ctx, "Installing app")
	if err := a.Install(ctx, s.DataPath(densityApk)); err != nil {
		s.Fatal("Failed to install app: ", densityApk)
	}
	act, err := arc.NewActivity(a, packageName, MainActivity)
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

	if gotScaleFactor, err := getScaleFactor(ctx, a); err != nil {
		s.Fatal("Error obtaining scale factor: ", err)
	} else if gotScaleFactor != 0 {
		s.Fatalf("Scale factor incorrect, got: %d, want: 0", gotScaleFactor)
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
		if err := performAndConfirmDensityChange(ctx, ew, a, test); err != nil {
			s.Fatal("Error with performing action: ", err)
		}
	}
}
