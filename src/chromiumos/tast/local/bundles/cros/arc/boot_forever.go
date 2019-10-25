// Copyright 2018 The Chromium OS Authors. All rights reserved.
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
	// TODO(hidehiko): Migrate this into arc.Boot, when crbug.com/1018045 is fixed.
	testing.AddTest(&testing.Test{
		Func: BootForever,
		Desc: "Checks that Android boots repeatedly",
		Contacts: []string{
			"ereth@chromium.org",
			"arc-core@google.com",
			"nya@chromium.org", // Tast port author.
		},
		Attr:         []string{"disabled"},
		SoftwareDeps: []string{"android", "chrome"},
		Timeout:      365 * 24 * time.Hour,
	})
}

func BootForever(ctx context.Context, s *testing.State) {
	iter := func() {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()

		cr, err := chrome.New(ctx, chrome.ARCEnabled())
		if err != nil {
			s.Fatal("Failed to connect to Chrome: ", err)
		}
		defer cr.Close(ctx)

		a, err := arc.New(ctx, s.OutDir())
		if err != nil {
			s.Fatal("Failed to start ARC: ", err)
		}
		defer a.Close()

		// Skip further sanity checks to speed up iterations.
	}

	for i := 0; ; i++ {
		s.Log("======= Iteration ", i+1)
		iter()
	}
}
