// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/perappdensity"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerAppDensitySurfaceView,
		Desc:         "Checks that density can be changed with an Android application that uses SurfaceView",
		Contacts:     []string{"sarakato@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"android_p", "chrome"},
		Data:         []string{densitySurfaceViewApk},
		Timeout:      4 * time.Minute,
		Pre:          arc.Booted(),
	})
}

const densitySurfaceViewApk = "ArcPerAppDensitySurfaceViewTest.apk"

func PerAppDensitySurfaceView(ctx context.Context, s *testing.State) {
	const packageName = "org.chromium.arc.testapp.perappdensitysurfaceviewtest"
	perappdensity.RunTest(ctx, s, perappdensity.DensityApk{Name: densitySurfaceViewApk, Package: packageName}, func(ctx context.Context, a *arc.ARC, chromeVoxConn *chrome.Conn) error {
		return nil
	})
}
