// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"chromiumos/tast/local/profiler"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     Profiler,
		Desc:     "Demonstrates how to use profiler package",
		Contacts: []string{"tast-owners@google.com"},
		Attr:     []string{"informational"},
	})
}

func Profiler(ctx context.Context, s *testing.State) {
	profs := []profiler.Profiler{
		profiler.Top(&profiler.TopOpts{
			Interval: 2 * time.Second,
		}),
		profiler.VMStat(nil)}

	// TODO(crbug.com/996728): aarch64 is disabled before the kernel crash is fixed.
	if u, err := sysutil.Uname(); err == nil && u.Machine != "aarch64" {
		profs = append(profs, profiler.Perf(&profiler.PerfOpts{Type: profiler.PerfStat}))
	}

	p, err := profiler.Start(ctx, s.OutDir(), profs...)
	if err != nil {
		s.Fatal("Failure in starting the profiler: ", err)
	}

	defer func() {
		if err := p.End(); err != nil {
			s.Fatal("Failure in ending the profiler: ", err)
		}
	}()

	// Wait for 2 seconds to gather perf.data.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failure in sleeping: ", err)
	}

}
