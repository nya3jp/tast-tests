// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package lacros implements a library used to setup and launch linux-chrome.
package lacros

import (
	"context"
	"time"

	"chromiumos/tast/local/lacros"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Sanity,
		Desc:         "Tests basic lacros startup",
		Contacts:     []string{"erikchen@chromium.org", "hidehiko@chromium.org", "edcourtney@chromium.org", "lacros-team@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"disabled"},
		// Future versions of this test will have other parameters corresponding to the source of the linux-chrome binary.
		Params: []testing.Param{
			{
				Name:      "artifact",
				Pre:       lacros.StartedByArtifact(),
				Timeout:   7 * time.Minute,
				ExtraData: []string{lacros.ImageArtifact},
			},
		},
	})
}

func Sanity(ctx context.Context, s *testing.State) {
	l, err := lacros.LaunchLinuxChrome(ctx, s.PreValue().(lacros.PreData))
	if err != nil {
		s.Fatal("Failed to launch linux-chrome")
	}
	defer l.Close(ctx)

	_, err = l.Devsess.CreateTarget(ctx, "about:blank")
	if err != nil {
		s.Fatal("Failed to open new tab")
	}
}
