// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/rpc"
	crash_service "chromiumos/tast/services/cros/crash"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WatchdogCrash,
		Desc:         "Verify artificial watchdog crash creates crash files",
		Contacts:     []string{"mutexlox@chromium.org", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"device_crash", "pstore", "reboot", "watchdog"},
		ServiceDeps:  []string{"tast.cros.crash.FixtureService"},
		HardwareDeps: hwdep.D(hwdep.SkipOnPlatform(
			// See https://crbug.com/1069618 for discussion of bob, scarlet, kevin issues.
			"bob",
			"scarlet",
			"kevin")),
		Timeout: 10 * time.Minute,
	})
}

func saveAllFiles(ctx context.Context, d *dut.DUT, matches []*crash_service.RegexMatch, dir string) error {
	var firstErr error
	for _, m := range matches {
		for _, f := range m.Files {
			if err := linuxssh.GetFile(ctx, d.Conn(), f, filepath.Join(dir, path.Base(f)), linuxssh.PreserveSymlinks); err != nil {
				testing.ContextLogf(ctx, "Failed to save file %s: %s", f, err)
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	return firstErr
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

	base := `kernel\.\d{8}\.\d{6}\.\d+\.0`
	biosLogMatches := &crash_service.RegexMatch{
		Regex: base + `\.bios_log`,
		Files: nil,
	}
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
			// Also remove the bios log if it was created.
			for _, f := range m.Files {
				biosLogMatches.Files = append(biosLogMatches.Files, strings.TrimSuffix(f, filepath.Ext(f))+".bios_log")
			}
			if len(m.Files) != 1 {
				s.Errorf("Unexpected number of kernel crashes: %d, want 1", len(m.Files))
				continue
			}
			if err := d.Command("/bin/grep", "-q", "sig=kernel-(WATCHDOG)", m.Files[0]).Run(ctx); err != nil {
				// get all files to help debug test failures
				if err := saveAllFiles(cleanupCtx, d, append(res.Matches, biosLogMatches), s.OutDir()); err != nil {
					s.Log("Failed to get meta file: ", err)
				}
				s.Error("Did not find correct pattern in meta file: ", err)
			}
		}
	}

	removeReq := &crash_service.RemoveAllFilesRequest{
		Matches: append(res.Matches, biosLogMatches),
	}
	if _, err := fs.RemoveAllFiles(ctx, removeReq); err != nil {
		s.Error("Error removing files: ", err)
	}
}
