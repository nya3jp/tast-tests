// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ImagePaste,
		Desc:         "",
		Contacts:     []string{"tetsui@chromium.org", "arc-framework+tast@google.com"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Attr:         []string{"group:mainline", "informational"},
		Timeout:      4 * time.Minute,
	})
}

func ImagePaste(ctx context.Context, s *testing.State) {
	// TODO(tetsui): Remove the flag once it's enabled by default.
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.EnableFeatures("ArcImageCopyPasteCompat"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	d, err := a.NewUIDevice(ctx)
	if err != nil {
		s.Fatal("Failed initializing UI Automator: ", err)
	}
	defer d.Close(ctx)

	const (
		apk = "ArcImagePasteTest.apk"
		pkg = "org.chromium.arc.testapp.imagepaste"
		activityName = ".MainActivity"
	)

	if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
		s.Fatal("Failed to install the app: ", err)
	}

	act, err := arc.NewActivity(a, pkg, activityName)
	if err != nil {
		s.Fatalf("Failed to create a new activity %q", activityName)
	}
	// defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatalf("Failed to start the activity %q", activityName)
	}
	// defer act.Stop(ctx, tconn)
}
