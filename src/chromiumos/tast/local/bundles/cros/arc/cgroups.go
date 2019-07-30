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
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Cgroups,
		Desc:         "Checks that foreground/background status of ARC applications reflects properly in cgroup limits",
		Contacts:     []string{"sonnyrao@chromium.org", "arc-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "android_p", "chrome"},
		Timeout:      4 * time.Minute,
	})
}

// See platform2/login_manager/session_manager_impl.cc for defintion of these constants.
const (
	// cpuShares is a file that indicates the relative amount of CPU this cgroup gets when there's contention.
	cpuShares = "/sys/fs/cgroup/cpu/session_manager_containers/cpu.shares"
	// cpuSharesARCBackground is the value for cpu.shares when ARC has nothing in the foreground.
	cpuSharesARCBackground = 64
	// cpuSharesARCForeground is the value of cpu.shares when ARC is in the foreground.
	cpuSharesARCForeground = 1024
)

// getCPUCgroupShares retrieves the current value for cpu.shares for the container.
func getCPUCgroupShares(ctx context.Context) (int, error) {
	b, err := ioutil.ReadFile(cpuShares)
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
	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.RestrictARCCPU())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close()

	// Check shares after ARC is done booting but nothing is active and poll up to 10 seconds.
	if err := testing.Poll(ctx, func(context.Context) error {
		share, err := getCPUCgroupShares(ctx)
		if err != nil {
			return err
		}
		if share == cpuSharesARCBackground {
			return nil
		}
		return errors.Errorf("unexpected ARC CPU shares value: %d", share)
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed waiting for ARC background shares: ", err)
	}

	// TODO(sonnyrao): Try to figure out how to use the app launcher to do this.
	act, err := arc.NewActivity(a, "com.android.settings", ".Settings")
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.Start(ctx); err != nil {
		s.Fatal("Failed start Settings activity: ", err)
	}

	if err := act.SetWindowState(ctx, arc.WindowStateNormal); err != nil {
		s.Fatal("Failed to set window state to Normal: ", err)
	}

	if err := act.WaitForIdle(ctx, 4*time.Second); err != nil {
		s.Fatal("Failed to wait for idle activity: ", err)
	}

	// Check shares after ARC window is up and in the foreground.
	share, err := getCPUCgroupShares(ctx)
	if err != nil {
		s.Fatal("Failed to get ARC CPU shares value: ", err)
	}

	// TODO(sonnyrao): try to show and then hide the apps shelf and verify the shares value stays as expected.
	if share != cpuSharesARCForeground {
		s.Fatal("Unexpected ARC CPU shares value foreground: ", share)
	}

	// Close ARC window and ensure we go back to background shares.
	if err := act.SetWindowState(ctx, arc.WindowStateMinimized); err != nil {
		s.Fatal("Failed to set window state to Minimized: ", err)
	}
	if err := act.WaitForIdle(ctx, 4*time.Second); err != nil {
		s.Fatal("Failed to wait for idle activity: ", err)
	}

	share, err = getCPUCgroupShares(ctx)
	if err != nil {
		s.Fatal("Failed to get ARC CPU shares value: ", err)
	}
	if share != cpuSharesARCBackground {
		s.Fatal("Unexpected ARC CPU shares value after minimize: ", share)
	}
}
