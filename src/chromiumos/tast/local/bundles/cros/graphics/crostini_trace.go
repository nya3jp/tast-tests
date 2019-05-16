// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniTrace,
		Desc:         "Sanity test for graphics trace replay in Crostini VM",
		Contacts:     []string{"chromeos-gfx@google.com"},
		Attr:         []string{"informational"},
		Data:         []string{"crostini_trace_glxgears.trace"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_hosts"},
	})
}

func CrostiniTrace(ctx context.Context, s *testing.State) {
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
			s.Error("Failure dumping container log: ", err)
		}
		vm.StopConcierge(ctx)
	}()

	s.Log("Verifying pwd command works")
	cmd := cont.Command(ctx, "pwd")
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run pwd: ", err)
	}
	// Log glxinfo so that we can give some insight on how the crostini is set up.
	cmd = cont.Command(ctx, "glxinfo")
	cmd.Run()
	cmd.DumpLog(ctx)

	// Stop the apt-daily systemd timers since they may end up running while we
	// are executing the tests and cause failures due to resource contention.
	for _, t := range []string{"apt-daily", "apt-daily-upgrade"} {
		cmd := cont.Command(ctx, "sudo", "systemctl", "stop", t+".timer")
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			s.Logf("Failed to stop %s timer: %v", t, err)
		}
	}

	// TODO(pwang): Install it in container image.
	s.Log("Installing apitrace")
	cmd = cont.Command(ctx, "sudo", "apt", "-y", "install", "apitrace")
	if err := cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to install apitrace: ", err)
	}

	s.Log("Copying existing trace file to container")
	const traceFileName = "crostini_trace_glxgears.trace"
	containerPath := filepath.Join("/home/testuser", traceFileName)
	if err := cont.PushFile(ctx, s.DataPath(traceFileName), containerPath); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed copying trace file to container: ", err)
	}

	s.Log("Start Replaying trace file")
	cmd = cont.Command(ctx, "apitrace", "replay", containerPath)
	traceOut, err := cmd.CombinedOutput()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to replay apitrace: ", err)
	}
	s.Log(string(traceOut))
	_, _, fps, err := parseResult(traceOut)
	if err != nil {
		s.Fatal("Failed to parse the result: ", err)
	}
	perfValues := perf.NewValues()
	perfValues.Set(perf.Metric{
		Name:      "glxgears",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}, fps)
	if err := perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

func parseResult(output []byte) (frames uint64, duration float64, fps float64, err error) {
	re := regexp.MustCompile(`Rendered (\d+) frames in (\d*\.?\d*) secs, average of (\d*\.?\d*) fps`)
	match := re.FindSubmatch(output)
	if match == nil || len(match) != 4 {
		err = errors.New("result line can't be located")
		return
	}
	_, err = fmt.Sscanf(string(match[1]), "%d", &frames)
	_, err = fmt.Sscanf(string(match[2]), "%f", &duration)
	_, err = fmt.Sscanf(string(match[3]), "%f", &fps)
	return
}
