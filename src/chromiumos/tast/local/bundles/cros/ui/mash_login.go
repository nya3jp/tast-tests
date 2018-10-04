// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	// Switch that identifies the ash mojo service. Keep in sync with chromium
	// switches::kMashServiceName in src/chrome/common/chrome_switches.cc
	// TODO(crbug.com/891470): Sometimes the chrome command line has truncated
	// arguments. Change this to "mash-service-name" when that is fixed.
	mashServiceName = "mash-s"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MashLogin,
		Desc:         "Checks that chrome --enable-features=Mash starts",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login"},
	})
}

// MashLogin checks that chrome --enable-features=Mash starts and at least one mash service is running.
func MashLogin(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.ExtraArgs([]string{"--enable-features=Mash"}))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	pids, err := chrome.GetPIDs()
	if err != nil {
		s.Fatal("Could not get chrome PIDs: ", err)
	}

	found := false
	cmds := make([]string, 0)
	for _, pid := range pids {
		// If we see errors, assume the process exited.
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			continue
		}
		cmd, err := proc.Cmdline()
		if err != nil {
			continue
		}
		if strings.Contains(cmd, mashServiceName) {
			found = true
			break
		}
		cmds = append(cmds, cmd)
	}
	if !found {
		s.Errorf("No chrome process containing %q among %v", mashServiceName, cmds)
	}
}
