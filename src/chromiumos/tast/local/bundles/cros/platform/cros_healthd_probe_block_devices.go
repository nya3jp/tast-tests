// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"

	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: CrosHealthdProbeBlockDevices,
		Desc: "Check that we can probe cros_healthd for various probe data points",
		Contacts: []string{
			"jschettler@google.com",
			"khegde@google.com",
			"pmoy@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"diagnostics"},
	})
}

func CrosHealthdProbeBlockDevices(ctx context.Context, s *testing.State) {
	records, err := croshealthd.FetchTelemetry(ctx, croshealthd.TelemCategoryStorage, s.OutDir())
	if err != nil {
		s.Fatal("Failed to get storage telemetry info: ", err)
	}

	// Every board should have at least one non-removable block device.
	if len(records) < 2 {
		s.Fatalf("Wrong number of output lines: got %d; want 2+", len(records))
	}
}
