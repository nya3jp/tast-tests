// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/rpc"
	crash_service "chromiumos/tast/services/cros/crash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WatchdogCrash,
		Desc:         "Verify artificial watchdog crash creates crash files",
		Contacts:     []string{"mutexlox@chromium.org", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"pstore", "reboot", "watchdog"},
		ServiceDeps:  []string{"tast.cros.crash.FixtureService"},
		// TODO(https://crbug.com/1045821): Remove this once samus issue is resolved.
		HardwareDeps: hwdep.D(hwdep.SkipOnPlatform("samus"),
			// See https://crbug.com/1069618 for discussion of bob, scarlet, kevin issues.
			hwdep.SkipOnPlatform("bob"), hwdep.SkipOnPlatform("scarlet"), hwdep.SkipOnPlatform("kevin")),
		Timeout: 10 * time.Minute,
	})
}

func WatchdogCrash(ctx context.Context, s *testing.State) {
	const systemCrashDir = "/var/spool/crash"

	d := s.DUT()

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	fs := crash_service.NewFixtureServiceClient(cl.Conn)

	req := crash_service.SetUpCrashTestRequest{
		Consent: crash_service.SetUpCrashTestRequest_MOCK_CONSENT,
	}

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if _, err := fs.SetUp(ctx, &req); err != nil {
		s.Error("Failed to set up: ", err)
		cl.Close(cleanupCtx)
		return
	}

	// This is a bit delicate. If the test fails _before_ we panic the machine,
	// we need to do TearDown then, and on the same connection (so we can close Chrome).
	//
	// If it fails to reconnect, we do not need to clean these up.
	//
	// Otherwise, we need to re-establish a connection to the machine and
	// run TearDown.
	defer func() {
		s.Log("Cleaning up")
		if fs != nil {
			if _, err := fs.TearDown(cleanupCtx, &empty.Empty{}); err != nil {
				s.Error("Couldn't tear down: ", err)
			}
		}
		if cl != nil {
			cl.Close(cleanupCtx)
		}
	}()

	// Sync filesystem to minimize impact of the crash on other tests
	if out, err := d.Command("sync").CombinedOutput(ctx); err != nil {
		s.Fatalf("Failed to sync filesystems: %s. err: %v", out, err)
	}

	// Trigger a watchdog reset
	// Run the triggering command in the background to avoid the DUT potentially going down before
	// success is reported over the SSH connection. Redirect all I/O streams to ensure that the
	// SSH exec request doesn't hang (see https://en.wikipedia.org/wiki/Nohup#Overcoming_hanging).

	// Daisydog is the watchdog service
	cmd := `nohup sh -c 'sleep 2
	stop daisydog
	sleep 60 > /dev/watchdog' >/dev/null 2>&1 </dev/null &`
	if err := d.Command("bash", "-c", cmd).Run(ctx); err != nil {
		s.Fatal("Failed to panic DUT: ", err)
	}

	s.Log("Waiting for DUT to become unreachable")

	if err := d.WaitUnreachable(ctx); err != nil {
		s.Fatal("Failed to wait for DUT to become unreachable: ", err)
	}
	s.Log("DUT became unreachable (as expected)")

	// When we lost the connection, these connections broke.
	cl.Close(ctx)
	cl = nil
	fs = nil

	s.Log("Reconnecting to DUT")
	if err := d.WaitConnect(ctx); err != nil {
		s.Fatal("Failed to reconnect to DUT: ", err)
	}
	s.Log("Reconnected to DUT")

	cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	fs = crash_service.NewFixtureServiceClient(cl.Conn)

	base := `kernel\.\d{8}\.\d{6}\.0`
	waitReq := &crash_service.WaitForCrashFilesRequest{
		Dirs:    []string{systemCrashDir},
		Regexes: []string{base + `\.kcrash`, base + `\.meta`, base + `\.log`},
	}
	s.Log("Waiting for files to become present")
	res, err := fs.WaitForCrashFiles(ctx, waitReq)
	if err != nil {
		if err := d.GetFile(cleanupCtx, "/var/log/messages",
			filepath.Join(s.OutDir(), "messages")); err != nil {
			s.Log("Failed to get messages log")
		}
		s.Fatal("Failed to find crash files: ", err.Error())
	}
	for _, m := range res.Matches {
		if strings.HasSuffix(m.Regex, ".meta") {
			if len(m.Files) != 1 {
				s.Errorf("Unexpected number of kernel crashes: %d, want 1", len(m.Files))
				continue
			}
			if err := d.Command("/bin/grep", "-q", "sig=kernel-(WATCHDOG)", m.Files[0]).Run(ctx); err != nil {
				s.Error("Did not find correct pattern in meta file: ", err)
			}
		}
	}

	removeReq := &crash_service.RemoveAllFilesRequest{
		Matches: res.Matches,
	}
	if _, err := fs.RemoveAllFiles(ctx, removeReq); err != nil {
		s.Error("Error removing files: ", err)
	}
}
