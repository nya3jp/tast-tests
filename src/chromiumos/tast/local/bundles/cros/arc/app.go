// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         App,
		Desc:         "Sanity check to starts a simple app",
		Contacts:     []string{"oka@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome"},
		Data:         []string{"app_hello-world.apk"},
		Pre:          arc.Booted(),
	})
}

func App(ctx context.Context, s *testing.State) {
	const (
		// This is a plain hello world app:
		// https://codelabs.developers.google.com/codelabs/android-training-hello-world
		apk = "app_hello-world.apk"
		pkg = "android.example.com"
		cls = ".MainActivity"
	)

	shortCtx, shortChancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer shortChancel()

	a := s.PreValue().(arc.PreData).ARC

	if err := a.Install(shortCtx, s.DataPath(apk)); err != nil {
		s.Fatal("Failed to install app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, cls)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	s.Log("Starting app")
	if err = act.Start(shortCtx); err != nil {
		s.Fatal("Failed to start app: ", err)
	}

	bounds, err := act.SurfaceBounds(shortCtx)
	if err != nil {
		s.Fatal("Failed to get window bounds: ", err)
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		s.Errorf("Bounds should be positive but was %dx%d", bounds.Width, bounds.Height)
	}
}
