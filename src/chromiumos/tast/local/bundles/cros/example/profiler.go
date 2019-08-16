// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"chromiumos/tast/local/profiler"
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
	p, err := profiler.Start(ctx, s.OutDir(), profiler.Perf, profiler.Top, profiler.VMStat)
	if err != nil {
		s.Fatal("Failure in starting the profiler: ", err)
	}

	vmstatOpts := profiler.VMStatOpts{
		Interval: 3,
	}
	pOpts, err := profiler.StartWithOpts(ctx, s.OutDir(), profiler.VMStatWithOpts, vmstatOpts)
	if err != nil {
		s.Fatal("Failure in starting profiler with options: ", err)
	}

	defer func() {
		if err := p.End(); err != nil {
			s.Fatal("Failure in ending the profiler: ", err)
		}
		if err := pOpts.End(); err != nil {
			s.Fatal("Failure in ending profiler with options: ", err)
		}
	}()

	// Wait for 2 seconds to gather perf.data.
	if err := testing.Sleep(ctx, 2*time.Second); err != nil {
		s.Fatal("Failure in sleeping: ", err)
	}

}
