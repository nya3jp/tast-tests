// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         AppValidity,
		Desc:         "Validity check of starting a simple app",
		Contacts:     []string{"oka@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBooted",
		Timeout:      3 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
	})
}

func AppValidity(ctx context.Context, s *testing.State) {
	const (
		// This is a plain hello world app.
		apk = "ArcAppValidityTest.apk"
		pkg = "org.chromium.arc.testapp.appvaliditytast"
		cls = ".MainActivity"
	)

	a := s.FixtValue().(*arc.PreData).ARC
	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	cr := s.FixtValue().(*arc.PreData).Chrome
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting app")
	if err = act.Start(ctx, tconn); err != nil {
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
