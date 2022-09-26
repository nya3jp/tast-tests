// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cellular

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/cellular"
	"chromiumos/tast/local/dlc"
	"chromiumos/tast/local/modemfwd"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ModemfwdFallbackToRootfsFailedToInstallDlc,
		Desc:         "Verifies that modemfwd can fallback to the rootfs FW images when it fails to install the DLC",
		Contacts:     []string{"andrewlassalle@google.com", "chromeos-cellular-team@google.com"},
		Attr:         []string{"group:cellular", "cellular_sim_active", "cellular_unstable"},
		Fixture:      "cellular",
		SoftwareDeps: []string{"modemfwd", "cellular_modem_dlcs_present"},
		Timeout:      1 * time.Minute,
	})
}

// ModemfwdFallbackToRootfsFailedToInstallDlc Test
func ModemfwdFallbackToRootfsFailedToInstallDlc(ctx context.Context, s *testing.State) {
	dlcID, err := cellular.GetDlcIDForVariant(ctx)
	if err != nil {
		s.Fatalf("Failed to get DLC ID: %s", err)
	}

	if err := dlc.Purge(ctx, dlcID); err != nil {
		s.Fatalf("Failed to purge dlc %q: %s", dlcID, err)
	}

	if err := upstart.StopJob(ctx, dlc.JobName); err != nil {
		s.Fatalf("Failed to stop %q: %s", dlc.JobName, err)
	}
	s.Log("dlcservice was stopped successfully")

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()
	// Ensure the test restores the dlcservice state.
	defer func(ctx context.Context) {
		ctx, st := timing.Start(ctx, "cleanUp")
		defer st.End()
		if err := upstart.EnsureJobRunning(ctx, dlc.JobName); err != nil {
			s.Fatal("Failed to start dlcservice: ", err)
		}
		s.Log("dlcservice has started successfully")
	}(cleanupCtx)

	// Force a DLC Install failure by moving the PRELOAD directory to another place
	extDirBase, err := ioutil.TempDir("", "")
	if err != nil {
		s.Fatal("Failed to create a tempdir: ", err)
	}
	preloadPath := filepath.Join(dlc.PreloadDir, dlcID)
	tempDlcPath := filepath.Join(extDirBase, dlcID)
	err = copyDir(preloadPath, tempDlcPath)
	if err != nil {
		s.Fatal("Failed to move dlc preload to tempdir: ", err)
	}
	os.RemoveAll(preloadPath)
	defer func() {
		err = copyDir(tempDlcPath, preloadPath)
		if err != nil {
			s.Logf("Failed to restore dlc to preload location:%s", err)
		}
	}()

	if err := upstart.StartJob(ctx, dlc.JobName); err != nil {
		s.Fatal("Failed to start dlcservice: ", err)
	}
	s.Log("dlcservice has started successfully")

	// Shorten deadline to leave time for cleanup.
	cleanupCtxModemFwd := ctx
	ctx, cancelModemFwd := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancelModemFwd()
	defer func(ctx context.Context) {
		if err := upstart.StopJob(ctx, modemfwd.JobName); err != nil {
			s.Fatalf("Failed to stop %q: %s", modemfwd.JobName, err)
		}
		s.Log("modemfwd has stopped successfully")
	}(cleanupCtxModemFwd)

	// modemfwd is initially stopped in the fixture SetUp
	if err := modemfwd.StartAndWaitForQuiescence(ctx); err != nil {
		s.Fatal("modemfwd failed during initialization: ", err)
	}
}

// copyDir copies a directory recursively.
func copyDir(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		src := filepath.Join(srcDir, rel)
		dst := filepath.Join(dstDir, rel)
		if info.IsDir() {
			return os.Mkdir(dst, 0755)
		}
		return fsutil.CopyFile(src, dst)
	})
}
