// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniStartTime,
		Desc:         "Performance tests of Termina VM startup and container startup",
		Attr:         []string{"informational"},
		Timeout:      7 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func CrostiniStartTime(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	if err = vm.EnableCrostiniSetting(ctx, tconn); err != nil {
		s.Fatal("Enable Crostini preference setting failed: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	s.Log("Creating default container")

	startTime := time.Now()

	cont, err := vm.CreateDefaultContainer(ctx, cr.User(), vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer func() {
		if err := cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failure dumping container log: ", err)
		}
	}()

	duration := time.Since(startTime)
	s.Log("Elapsed time: ", duration)

	var value perf.Values
	value.Set(perf.Metric{
		Name:      "crostini_start_time",
		Variant:   "total",
		Unit:      "s",
		Direction: perf.SmallerIsBetter,
		Multiple:  false}, duration.Seconds())
	value.Save(s.OutDir())
}
