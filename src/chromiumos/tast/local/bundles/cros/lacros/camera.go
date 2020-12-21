// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"time"

	"chromiumos/tast/local/lacros/launcher"
	"chromiumos/tast/local/webrtc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Camera,
		Desc:         "Tests basic camera functionality on lacros",
		Contacts:     []string{"wtlee@chromium.org", "lacros-team@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "lacros"},
		Pre:          launcher.StartedByData(),
		Timeout:      7 * time.Minute, // A lenient limit for launching Lacros Chrome.
		Data:         append(webrtc.DataFiles(), launcher.DataArtifact, "getusermedia.html"),
	})
}

// Camera runs GetUserMedia test on Lacros-Chrome.
func Camera(ctx context.Context, s *testing.State) {
	l, err := launcher.LaunchLacrosChrome(ctx, s.PreValue().(launcher.PreData))
	if err != nil {
		s.Fatal("Failed to launch lacros-chrome: ", err)
	}
	defer l.Close(ctx)

	if _, err := webrtc.RunGetUserMedia(ctx, s.DataFileSystem(), l, 3*time.Second, webrtc.VerboseLogging); err != nil {
		s.Error("Failed when running get user media: ", err)
	}
}
