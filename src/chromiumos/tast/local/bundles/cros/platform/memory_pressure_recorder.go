// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"time"

	"chromiumos/tast/local/bundles/cros/platform/chromewpr"
	"chromiumos/tast/local/bundles/cros/platform/mempressure"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MemoryPressureRecorder,
		Desc:         "Record a WPR archive for platform.MemoryPressure",
		Contacts:     []string{"semenzato@chromium.org", "sonnyrao@chromium.org", "chromeos-memory@google.com"},
		Attr:         []string{"disabled", "informational"},
		Timeout:      60 * time.Minute,
		SoftwareDeps: []string{"chrome"},
	})
}

// MemoryPressureRecorder runs WPR in recording mode.
func MemoryPressureRecorder(ctx context.Context, s *testing.State) {
	p := &mempressure.RunParameters{
		Mode: chromewpr.Record,
	}

	cp := &chromewpr.Params{
		WPRArchivePath: "/tmp/archive.wprgo",
		Mode:           chromewpr.Record,
	}
	w, err := chromewpr.New(ctx, cp)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer w.Close(ctx)

	mempressure.Run(ctx, s, w.Chrome, p)
}
