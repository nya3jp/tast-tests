// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"path/filepath"
	"time"

	"chromiumos/tast/local/bundles/cros/vm/subtest"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniStartEverything,
		Desc:         "Tests Termina VM startup, container startup and other Crostini functionality",
		Attr:         []string{"informational"},
		Data:         []string{"cros-tast-tests-deb.deb"},
		Timeout:      7 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func CrostiniStartEverything(s *testing.State) {
	defer faillog.SaveIfError(s)

	ctx := s.Context()
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Set the preference for Crostini being enabled as this is required for some
	// of the Chrome integration tests to function properly.
	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	if err = tconn.EvalPromise(ctx,
		`new Promise((resolve, reject) => {
		   chrome.autotestPrivate.setCrostiniEnabled(true, () => {
		     if (chrome.runtime.lastError === undefined) {
		       resolve();
		     } else {
		       reject(chrome.runtime.lastError.message);
		     }
		   });
		 })`, nil); err != nil {
		s.Fatal("Running autotestPrivate.setCrostiniEnabled failed: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	s.Log("Creating default container")
	cont, err := vm.CreateDefaultContainer(ctx, cr.User(), vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer vm.StopConcierge(ctx)

	s.Log("Verifying pwd command works")
	cmd := cont.Command(ctx, "pwd")
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run pwd: ", err)
	}

	// The VM and container have started up so we can now execute all of the other
	// Crostini tests. We need to be careful about this because we are going to be
	// testing multiple things in one test. This should be done so that no tests
	// have any known dependency on prior tests. If we hit a conflict at some
	// point then we will need to add functionality to save the VM/container image
	// at this point and then stop the VM/container and restore that image so we
	// can have a clean VM/container to start from again. Failures should not be
	// fatal so that all tests can get executed.
	subtest.Webserver(s, cr, cont)
	subtest.LaunchTerminal(s, cr, cont)
	subtest.LaunchBrowser(s, cr, cont)
	subtest.VerifyAppFromTerminal(s, cont, "x11", "/opt/google/cros-containers/bin/x11_demo",
		screenshot.Color{R: 0x9999, G: 0xeeee, B: 0x4444})
	subtest.VerifyAppFromTerminal(s, cont, "wayland", "/opt/google/cros-containers/bin/wayland_demo",
		screenshot.Color{R: 0x3333, G: 0x8888, B: 0xdddd})

	// Copy a test Debian package file to the container which will be used by
	// subsequent tests.
	const debianFilename = "cros-tast-tests-deb.deb"
	containerDebPath := filepath.Join("/home/testuser", debianFilename)
	err = cont.PushFile(ctx, s.DataPath(debianFilename), containerDebPath)
	if err != nil {
		s.Fatal("Failed copying test Debian package to container:", err)
	}

	subtest.LinuxPackageInfo(s, cont, containerDebPath)
	err = subtest.InstallPackage(ctx, cont, containerDebPath)
	if err != nil {
		s.Error("Failure in performing Linux package install", err)
	} else {
		// TODO(jkardatzke): Verify apps in Chrome launcher exist and that we can
		// launch them properly from the Chrome launcher.
	}
}
