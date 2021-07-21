// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/rpc"
	crash_service "chromiumos/tast/services/cros/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         KernelCrash,
		Desc:         "Verify artificial kernel crash creates crash files",
		Contacts:     []string{"mutexlox@chromium.org", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"device_crash", "pstore", "reboot"},
		ServiceDeps:  []string{"tast.cros.crash.FixtureService"},
		Params: []testing.Param{{
			Name:              "real_consent",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"chrome", "metrics_consent"},
			Val:               crash_service.SetUpCrashTestRequest_REAL_CONSENT,
		}, {
			Name: "mock_consent",
			Val:  crash_service.SetUpCrashTestRequest_MOCK_CONSENT,
		}},
		Timeout: 10 * time.Minute,
	})
}

func KernelCrash(ctx context.Context, s *testing.State) {
	const systemCrashDir = "/var/spool/crash"

	d := s.DUT()

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	fs := crash_service.NewFixtureServiceClient(cl.Conn)

	req := crash_service.SetUpCrashTestRequest{
		Consent: s.Param().(crash_service.SetUpCrashTestRequest_ConsentType),
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

	if out, err := d.Conn().CommandContext(ctx, "logger", "Running KernelCrash").CombinedOutput(); err != nil {
		s.Logf("WARNING: Failed to log info message: %s", out)
	}

	// Sync filesystem to minimize impact of the panic on other tests
	if out, err := d.Conn().CommandContext(ctx, "sync").CombinedOutput(); err != nil {
		s.Fatalf("Failed to sync filesystems: %s", out)
	}

	// Trigger a panic
	// Run the triggering command in the background to avoid the DUT potentially going down before
	// success is reported over the SSH connection. Redirect all I/O streams to ensure that the
	// SSH exec request doesn't hang (see https://en.wikipedia.org/wiki/Nohup#Overcoming_hanging).
	cmd := `nohup sh -c 'sleep 2
	if [ -f /sys/kernel/debug/provoke-crash/DIRECT ]; then
		echo PANIC > /sys/kernel/debug/provoke-crash/DIRECT
	else
		echo panic > /proc/breakme
	fi' >/dev/null 2>&1 </dev/null &`
	if err := d.Conn().CommandContext(ctx, "sh", "-c", cmd).Run(); err != nil {
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
	waitReq := &crash_service.WaitForCrashFilesRequest{
		Dirs:    []string{systemCrashDir},
		Regexes: []string{base + `\.kcrash`, base + `\.meta`, base + `\.log`},
	}
	s.Log("Waiting for files to become present")
	res, err := fs.WaitForCrashFiles(ctx, waitReq)
	if err != nil {
		if err := d.GetFile(cleanupCtx, "/var/log/messages",
			filepath.Join(s.OutDir(), "messages")); err != nil {
			s.Log("Failed to save messages log")
		}
		s.Fatal("Failed to find crash files: " + err.Error())
	}

	// Verify that expected signature for kernel crashes is non-zero
	for _, match := range res.Matches {
		if !strings.HasSuffix(match.Regex, ".meta") {
			continue
		}
		s.Log("Checking signature line for non-zero")
		if err := d.GetFile(cleanupCtx, match.Files[0],
			filepath.Join(s.OutDir(), path.Base(match.Files[0]))); err != nil {
			s.Error("Failed to save meta file")
			continue
		}
		f, err := ioutil.ReadFile(filepath.Join(s.OutDir(), path.Base(match.Files[0])))
		if err != nil {
			s.Error("Failed to read meta file", match.Files[0])
			continue
		}
		badSigRegexp := regexp.MustCompile("sig=kernel-.+-00000000")
		goodSigRegexp := regexp.MustCompile("sig=kernel-.+-[[:xdigit:]]{8}")
		if badSigRegexp.Match(f) {
			s.Error("Found all zero signature in meta file ", match.Files[0])
		} else if !goodSigRegexp.Match(f) {
			s.Error("Couldn't find unique signature in meta file ", match.Files[0])
		}
	}

	// Also remove the bios log if it was created.
	biosLogMatches := &crash_service.RegexMatch{
		Regex: base + `\.bios_log`,
		Files: nil,
	}
	for _, f := range res.Matches[0].Files {
		biosLogMatches.Files = append(biosLogMatches.Files, strings.TrimSuffix(f, filepath.Ext(f))+".bios_log")
	}
	removeReq := &crash_service.RemoveAllFilesRequest{
		Matches: append(res.Matches, biosLogMatches),
	}

	if _, err := fs.RemoveAllFiles(ctx, removeReq); err != nil {
		s.Error("Error removing files: ", err)
	}
}
