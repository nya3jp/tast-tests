// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crash

import (
	"context"
	"io/ioutil"
	"path"
	"path/filepath"
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
		Func:         EarlyCrash,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify artificial early crash creates crash files",
		Contacts:     []string{"mutexlox@chromium.org", "cros-telemetry@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"reboot"},
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

func EarlyCrash(ctx context.Context, s *testing.State) {
	const systemCrashDir = "/var/spool/crash"

	d := s.DUT()

	cl, err := rpc.Dial(ctx, d, s.RPCHint())
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

	fs = nil
	cl.Close(ctx)
	cl = nil

	// init/upstart/test-init/early-failure.conf will crash early in boot.
	s.Log("Rebooting")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// When we lost the connection, these connections broke.
	s.Log("Re-dialing")
	cl, err = rpc.Dial(ctx, d, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	fs = crash_service.NewFixtureServiceClient(cl.Conn)

	base := `coreutils\.\d{8}\.\d{6}\.\d+\.\d+`
	waitReq := &crash_service.WaitForCrashFilesRequest{
		Dirs: []string{systemCrashDir},
		Regexes: []string{base + `\.dmp`, base + `\.meta`,
			base + `\.core`, base + `\.proclog`},
	}
	s.Log("Waiting for files to become present")
	res, err := fs.WaitForCrashFiles(ctx, waitReq)
	if err != nil {
		if err := d.GetFile(cleanupCtx, "/var/log/messages",
			filepath.Join(s.OutDir(), "messages")); err != nil {
			s.Log("Failed to get messages log")
		}
		if err := d.GetFile(cleanupCtx, "/run/crash-reporter-early-init.log",
			filepath.Join(s.OutDir(), "crash-reporter-early-init.log")); err != nil {
			s.Log("Failed to get early-init log")
		}
		if out, err := d.Conn().CommandContext(ctx, "/bin/ls", "-l", "/var/spool/crash/", "/run/crash_reporter/").CombinedOutput(); err != nil {
			s.Log("Failed to list crash state dirs: ", err)
		} else if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "crash_state_dirs.txt"), out, 0644); err != nil {
			s.Log("Failed to save crash file listing to outDir: ", err)
		}
		s.Fatal("Failed to find crash files: ", err.Error())
	}

	// Verify that expected metadata tag for early crashes is present
	for _, match := range res.Matches {
		if !strings.HasSuffix(match.Regex, ".meta") {
			continue
		}
		if err := d.Conn().CommandContext(ctx, "/bin/grep", "-q", "upload_var_is_early_boot=true", match.Files[0]).Run(); err != nil {
			s.Error("Couldn't find expected string in meta file: ", err)
			if err := d.GetFile(cleanupCtx, match.Files[0],
				filepath.Join(s.OutDir(), path.Base(match.Files[0]))); err != nil {
				s.Log("Failed to get meta file")
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
