// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppSanity,
		Desc:         "Sanity check to start a simple app",
		Contacts:     []string{"oka@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{"app_sanity_hello_world.apk"},
		Pre:          arc.Booted(),
	})
}

func AppSanity(ctx context.Context, s *testing.State) {
	const (
		// This is a plain hello world app:
		// https://googleplex-android.googlesource.com/platform/vendor/google_arc/+/refs/heads/pi-arc/packages/development/ArcAppSanityTastTest
		apk = "app_sanity_hello_world.apk"
		pkg = "org.chromium.arc.testapp.appsanitytast"
		cls = ".MainActivity"
	)

	a := s.PreValue().(arc.PreData).ARC
	if err := a.Install(ctx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting app")
	if err = act.Start(ctx); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	err = testing.Poll(ctx, func(ctx context.Context) error {
		bounds, err := act.SurfaceBounds(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get surface bounds")
		}
		if bounds.Width <= 0 || bounds.Height <= 0 {
			return errors.Errorf("bounds should be positive but were %dx%d", bounds.Width, bounds.Height)
		}
		return nil
	}, nil)
	if err != nil {
		s.Error("Failed waiting for app window: ", err)
	}
}
