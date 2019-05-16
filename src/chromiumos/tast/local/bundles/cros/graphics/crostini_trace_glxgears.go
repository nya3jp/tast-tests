// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/graphics/trace"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

var crostiniTraceGlxgearsTraceNameMap = make(map[string]string)
var crostiniTraceGlxgearsTraceNames = [...]string{"glxgears"}

func init() {
	var data = make([]string, 0)
	var filePrefix = "crostini_trace_glxgears"
	for _, traceName := range crostiniTraceGlxgearsTraceNames {
		fileName := filePrefix + "_" + traceName + ".trace"
		crostiniTraceGlxgearsTraceNameMap[fileName] = traceName
		data = append(data, fileName)
	}
	testing.AddTest(&testing.Test{
		Func:         CrostiniTraceGlxgears,
		Desc:         "Replay graphics trace in Crostini VM",
		Contacts:     []string{"chromeos-gfx@google.com"},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		Data:         data,
		Pre:          chrome.LoggedIn(),
		Timeout:      3 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func CrostiniTraceGlxgears(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}
	defer vm.UnmountComponent(ctx)

	s.Log("Creating default container")
	cont, err := vm.CreateDefaultContainer(ctx, s.OutDir(), cr.User(), vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer func() {
		if err := cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failed to dump container log: ", err)
		}
		vm.StopConcierge(ctx)
	}()

	s.Log("Verifying pwd command works")
	cmd := cont.Command(ctx, "pwd")
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run pwd: ", err)
	}

	shortCtx, shortCancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer shortCancel()
	if err := trace.InstallApitrace(shortCtx, s, cont); err != nil {
		s.Fatal("Failed to get Apitrace: ", err)
	}
	for traceFileName, traceName := range crostiniTraceGlxgearsTraceNameMap {
		result := trace.RunAPITrace(shortCtx, s, cont, traceFileName)
		perfValues := perf.NewValues()
		perfValues.Set(perf.Metric{
			Name:      traceName,
			Unit:      "fps",
			Direction: perf.BiggerIsBetter,
		}, result.Fps)
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Fatal("Failed saving perf data: ", err)
		}
	}
}
