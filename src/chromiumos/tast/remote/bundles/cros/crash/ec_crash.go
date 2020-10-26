// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/servo"
	"chromiumos/tast/rpc"
	crash_service "chromiumos/tast/services/cros/crash"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     ECCrash,
		Desc:     "Verify artificial EC crash creates crash files",
		Contacts: []string{"mutexlox@chromium.org", "cros-telemetry@google.com"},
		Attr:     []string{"group:mainline", "informational"},
		// no_qemu because the servo is not available in VMs, and tast does
		// not (yet) support skipping tests if required vars are empty.
		// TODO(crbug.com/967901): Remove no_qemu dep once servo var is sufficient.
		SoftwareDeps: []string{"pstore", "reboot", "no_qemu"},
		HardwareDeps: hwdep.D(hwdep.SkipOnModel(
			// The below models have buggy EC firmware and cannot capture
			// crash reports. See https://crbug.com/1123191
			"asuka",
			"banon",
			"caroline",
			"cave",
			// TODO(https://crbug.com/1124372): 'celes' sometimes
			// shows up with model name "r", so it runs despite this
			// SkipOnModel.
			"celes",
			"chell",
			"cyan",
			"edgar",
			"kefka",
			"reks",
			"relm",
			"sabin", // a model name for some kefka DUTs
			"sentry",
			"terra",
			"terra2",
			"ultima",
			"wizpig",
			// The below models do not support the "crash" EC command.
			// See https://crbug.com/1123716
			"arcada",
			"arcada_signed",
			"drallion",
			"drallion_signed",
			"drallion360",
			"drallion360_signed",
			"sarien",
			"sarien_signed",
			// The below models do not have an EC.
			// See https://crbug.com/1123716
			// TODO(https://crbug.com/1124554): When tast has one, use a separate "EC" hwdep
			// rather than listing models.
			"fievel", // veyron_fievel
			"tiger",  // veyron_tiger
			"tidus",
			// The below model sporadically captures panics.
			// TODO(https://crbug.com/1135798): Re-enable this test
			// when bug is fixed.
			"nocturne",
		)),
		ServiceDeps: []string{"tast.cros.crash.FixtureService"},
		Timeout:     10 * time.Minute,
		Vars:        []string{"servo"},
	})
}

// ECCrash verifies that crash files are generated when the EC crashes.
func ECCrash(ctx context.Context, s *testing.State) {
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

	if out, err := d.Command("logger", "Running ECCrash").CombinedOutput(ctx); err != nil {
		s.Logf("WARNING: Failed to log info message: %s", out)
	}

	// Sync filesystem to minimize impact of the panic on other tests
	if out, err := d.Command("sync").CombinedOutput(ctx); err != nil {
		s.Fatalf("Failed to sync filesystems: %s", out)
	}

	// This is expected to fail in VMs, since Servo is unusable there and the "servo" var won't
	// be supplied. https://crbug.com/967901 tracks finding a way to skip tests when needed.
	pxy, err := servo.NewProxy(ctx, s.RequiredVar("servo"), d.KeyFile(), d.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(ctx)

	s.Log("Running crash command")
	// This should reboot the device
	if err := pxy.Servo().RunECCommand(ctx, "crash divzero"); err != nil {
		s.Fatal("Failed to run EC command")
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

	const base = `embedded_controller\.\d{8}\.\d{6}\.0`
	waitReq := &crash_service.WaitForCrashFilesRequest{
		Dirs:    []string{systemCrashDir},
		Regexes: []string{base + `\.eccrash`, base + `\.meta`},
	}
	s.Log("Waiting for files to become present")
	res, err := fs.WaitForCrashFiles(ctx, waitReq)
	if err != nil {
		if err := d.GetFile(cleanupCtx, "/var/log/messages", filepath.Join(s.OutDir(), "messages")); err != nil {
			s.Log("Failed to save messages log")
		}
		s.Fatal("Failed to find crash files: " + err.Error())
	}

	removeReq := &crash_service.RemoveAllFilesRequest{
		Matches: res.Matches,
	}
	if _, err := fs.RemoveAllFiles(ctx, removeReq); err != nil {
		s.Error("Error removing files: ", err)
	}
}
