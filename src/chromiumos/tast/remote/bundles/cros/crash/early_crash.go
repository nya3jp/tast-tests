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
	"chromiumos/tast/rpc"
	crash_service "chromiumos/tast/services/cros/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EarlyCrash,
		Desc:         "Verify artificial early crash creates crash files",
		Contacts:     []string{"mutexlox@chromium.org", "cros-monitoring-forensics@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "metrics_consent", "reboot"},
		ServiceDeps:  []string{"tast.cros.crash.FixtureService"},
	})
}

func EarlyCrash(ctx context.Context, s *testing.State) {
	const systemCrashDir = "/var/spool/crash"

	d := s.DUT()

	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	fs := crash_service.NewFixtureServiceClient(cl.Conn)

	if _, err := fs.SetUp(ctx, &empty.Empty{}); err != nil {
		cl.Close(ctx)
		s.Fatal("Failed to set up: ", err)
	}

	// Shorten deadline to leave time for cleanup
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

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

	// init/upstart/test-init/early-falure.conf will crash early in boot.
	s.Log("Rebooting")
	if err := d.Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot DUT: ", err)
	}

	// TODO(mutexlox): After the reboot, when crash runs with --boot_collect,
	// it sometimes fails consent. (~3-4/10 times). Figure out why.

	// When we lost the connection, these connections broke.
	s.Log("Re-dialing")
	cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	fs = crash_service.NewFixtureServiceClient(cl.Conn)

	base := `coreutils\.\d{8}\.\d{6}\.\d{1,4}`
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
		s.Fatal("Failed to find crash files: ", err.Error())
	}

	// Verify that expected metadata tag for early crashes is present
	for _, match := range res.Matches {
		if !strings.HasSuffix(match.Regex, ".meta") {
			continue
		}
		if err := d.Command("/bin/grep", "-q", "upload_var_is_early_boot=true", match.Files[0]).Run(ctx); err != nil {
			s.Error("Couldn't find expected string in meta file: ", err)
			if err := d.GetFile(cleanupCtx, match.Files[0],
				filepath.Join(s.OutDir(), path.Base(match.Files[0]))); err != nil {
				s.Log("Failed to get messages log")
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
