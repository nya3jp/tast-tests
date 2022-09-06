// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package factory

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/factory/toolkit"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Goofy,
		Desc:         "Setup factory toolkit and exercise Goofy with custom TestList",
		Contacts:     []string{"lschyi@google.com", "chromeos-factory-eng@google.com"},
		SoftwareDeps: []string{"factory_flow"},
		Attr:         []string{"group:mainline"},
		Timeout:      3 * time.Minute,
	})
}

func Goofy(fullCtx context.Context, s *testing.State) {
	const finishFlagFilePath = "/tmp/tast_factory_test"

	ctx, cancel := ctxutil.Shorten(fullCtx, 30*time.Second)
	defer cancel()

	setupFactory(ctx, s)
	defer cleanup(fullCtx, s)

	// Make sure the temp file is not there.
	os.Remove(finishFlagFilePath)
	if err := testexec.CommandContext(ctx, "start", "factory").Run(testexec.DumpLogOnError); err != nil {
		s.Fatal("Failed to start factory toolkit: ", err)
	}
	s.Log("factory toolkit started, waiting for TestList finished")

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(finishFlagFilePath); os.IsNotExist(err) {
			return errors.Errorf("finished flag file %s is missing", finishFlagFilePath)
		} else if err != nil {
			return errors.Wrapf(err, "failed to access finished flag file %s", finishFlagFilePath)
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Factory software fail to init and run. This high level and complicate tests is not a good choice to debug. Please check dumpped logs or other failed tests.  Failed to finish Goofy in time: ", err)
	}
}

func setupFactory(ctx context.Context, s *testing.State) {
	installer := toolkit.Installer{
		InstallerPath: toolkit.ToolkitInstallerPath,
		NoEnable:      true,
	}
	ver, err := installer.InstallFactoryToolKit(ctx)
	if err != nil {
		s.Fatal("Install fail: ", err)
	}
	s.Logf("Installed factory toolkit with version: %s", ver)
}

func cleanup(ctx context.Context, s *testing.State) {
	s.Log("Start to backup factory logs under /var/log")

	logFiles := [3]string{"factory-init.log", "factory-session.log", "factory.log"}
	for _, logFile := range logFiles {
		src := filepath.Join("/var/log", logFile)
		dst := filepath.Join(s.OutDir(), logFile)
		if err := fsutil.CopyFile(src, dst); err != nil {
			s.Errorf("Failed to backup %s: %v", logFile, err)
		}
	}

	s.Log("Start to cleanup DUT")

	if err := stopGoofy(ctx); err != nil {
		s.Fatal("Failed to stop goofy when cleanup: ", err)
	}
	s.Log("Stopped Goofy")

	if err := toolkit.UninstallFactoryToolKit(ctx); err != nil {
		s.Fatal("Failed to uninstall factory toolkit when cleanup: ", err)
	}
	s.Log("Uninstalled factory toolkit")
}

// stopGoofy and cleanup all factory configuration.
func stopGoofy(ctx context.Context) error {
	// Cleanup the state files except logs.
	return testexec.CommandContext(ctx, "factory_restart", "-S", "-s", "-t", "-r").Run(testexec.DumpLogOnError)
}
