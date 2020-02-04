// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	common_crash "chromiumos/tast/common/crash"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/baserpc"
	crash_service "chromiumos/tast/services/cros/crash"
	"chromiumos/tast/testing"
)

const (
	leaveCorePath = "/root/.leave_core"
	corePattern   = "/proc/sys/kernel/core_pattern"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ReporterStartup,
		Desc: "Verifies crash reporter after reboot",
		Contacts: []string{
			"domlaskowski@chromium.org", // Original autotest author
			"yamaguchi@chromium.org",    // Tast port author
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent"},
		ServiceDeps: []string{"tast.cros.crash.FixtureService",
			"tast.cros.baserpc.FileSystem"},
	})
}

// ReporterStartup tests crash reporter is set up correctly after reboot.
// Equivlaent to the local test in testReporterStartup, but without
// re-initializing to catch problems with the default crash reporting setup.
// See src/chromiumos/tast/local/bundles/cros/platform/user_crash.go
func ReporterStartup(ctx context.Context, s *testing.State) {
	d := s.DUT()
	s.Log("Rebooting DUT")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Turn off crash filtering so we see the original setting.
	fs := crash_service.NewFixtureServiceClient(cl.Conn)
	_, err = fs.DisableCrashFilter(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to turn off crash filtering: ", err)
	}

	fc := baserpc.NewFileSystemClient(cl.Conn)
	data, err := fc.ReadFile(ctx, &baserpc.ReadFileRequest{Name: corePattern})
	if err != nil {
		s.Fatal("Failed to read core pattern file: ", corePattern)
	}
	trimmed := strings.TrimSuffix(string(data.Content), "\n")
	expectedCorePattern := fmt.Sprintf("|%s --user=%%P:%%s:%%u:%%g:%%e", common_crash.CrashReporterPath)
	if trimmed != expectedCorePattern {
		s.Errorf("Unexpected core_pattern: got %s, want %s", trimmed, expectedCorePattern)
	}

	// Check that we wrote out the file indicating that crash_reporter is
	// enabled AFTER the system was booted. This replaces the old technique
	// of looking for the log message which was flakey when the logs got
	// flooded.
	// NOTE: This technique doesn't need to be highly accurate, we are only
	// verifying that the flag was written after boot and there are multiple
	// seconds between those steps, and a file from a prior boot will almost
	// always have been written out much further back in time than our
	// current boot time.

	fileinfo, err := fc.Stat(ctx, &baserpc.StatRequest{Name: common_crash.CrashReporterEnabledPath})
	if err != nil || !os.FileMode(fileinfo.Mode).IsRegular() {
		s.Error("Crash reporter enabled file flag is not present at ", common_crash.CrashReporterEnabledPath)
	}

	uptimeInfo, err := fc.Stat(ctx, &baserpc.StatRequest{Name: "/proc/uptime"})
	if err != nil {
		s.Fatal("Failed to read uptime file: ", err)
	}
	flagTime := time.Unix(fileinfo.Modified.Seconds, int64(fileinfo.Modified.Nanos))
	uptime := time.Unix(uptimeInfo.Modified.Seconds, int64(uptimeInfo.Modified.Nanos))
	if flagTime.Before(uptime) {
		s.Error("User space crash handling was not started during last boot")
	}
}
