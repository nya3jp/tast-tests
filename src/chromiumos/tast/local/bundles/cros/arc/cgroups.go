// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Cgroups,
		Desc:         "Checks that foreground/background status of ARC applications reflects properly in cgroup limits",
		Contacts:     []string{"sonnyrao@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      4 * time.Minute,
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p"},
			Val:               "/sys/fs/cgroup/cpu/session_manager_containers/cpu.shares",
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val:               "/sys/fs/cgroup/cpu/vms/arc/cpu.shares",
		}},
	})
}

// See platform2/login_manager/session_manager_impl.cc for defintion of these constants.
const (
	// cpuSharesARCBackground is the value for cpu.shares when ARC has nothing in the foreground.
	cpuSharesARCBackground = 64
	// cpuSharesARCForeground is the value of cpu.shares when ARC is in the foreground.
	cpuSharesARCForeground = 1024
)

// cpuCgroupShares retrieves the current value for cpu.shares for the container.
func cpuCgroupShares(path string) (int, error) {
	// Read from a file that indicates the relative amount of CPU this cgroup gets when there's contention.
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}
	shares, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, errors.Wrapf(err, "bad integer: %q", b)
	}
	return shares, nil
}

func Cgroups(ctx context.Context, s *testing.State) {
	// Path to cpu.shares.
	path := s.Param().(string)

	const pkgName = "com.android.settings"
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.RestrictARCCPU())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer tconn.Close()

	// TODO(sonnyrao): Try to figure out how to use the app launcher to do this.
	act, err := arc.NewActivity(a, pkgName, ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}
	defer act.Stop(ctx, tconn)

	if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventMaximize); err != nil {
		s.Fatal("Failed to maximize the activity: ", err)
	}

	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMaximized); err != nil {
		s.Fatal("Failed to wait for activity to enter Maximized state: ", err)
	}

	// Check shares after ARC window is up and in the foreground.
	share, err := cpuCgroupShares(path)
	if err != nil {
		s.Fatal("Failed to get ARC CPU shares value: ", err)
	}

	// TODO(sonnyrao): try to show and then hide the apps shelf and verify the shares value stays as expected.
	if share != cpuSharesARCForeground {
		s.Fatal("Unexpected ARC CPU shares value foreground: ", share)
	}
	// Minimize ARC window and ensure we go back to background shares.

	if _, err := ash.SetARCAppWindowState(ctx, tconn, act.PackageName(), ash.WMEventMinimize); err != nil {
		s.Fatal("Failed to set window state to Minimized: ", err)
	}
	if err := ash.WaitForARCAppWindowState(ctx, tconn, act.PackageName(), ash.WindowStateMinimized); err != nil {
		s.Fatal("Failed to wait for activity to become Minimized: ", err)
	}

	// TODO(b/152733335): Fix and remove this in ARCVM.
	if path == "/sys/fs/cgroup/cpu/session_manager_containers/cpu.shares" {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			share, err = cpuCgroupShares(path)
			if err != nil {
				return testing.PollBreak(err)
			}
			if share != cpuSharesARCBackground {
				return errors.Errorf("unexpected ARC CPU shares: got %d, want %d", share, cpuSharesARCBackground)
			}
			return nil
		}, &testing.PollOptions{Timeout: 15 * time.Second}); err != nil {
			s.Error("Failed to verify cpu.shares after minimize: ", err)
		}
	}
}
