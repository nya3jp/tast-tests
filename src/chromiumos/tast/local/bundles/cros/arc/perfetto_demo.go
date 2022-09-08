// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PerfettoDemo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Perfetto Demo",
		Contacts:     []string{"sstan@google.com"},
		SoftwareDeps: []string{"chrome", "android_vm"},
		Fixture:      "arcBooted",
		Data:         []string{"perfetto_config_demo.pbtxt"},
		Timeout:      5 * time.Minute,
	})
}

func PerfettoDemo(ctx context.Context, s *testing.State) {
	a := s.FixtValue().(*arc.PreData).ARC
	if err := a.RunPerfettoTrace(ctx, s.DataPath("perfetto_config_demo.pbtxt"), filepath.Join(s.OutDir(), "pulledtrace")); err != nil {
		s.Fatal("Error on run perfetto trace")
	}
}
