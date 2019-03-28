// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/vm/subtest"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CrostiniStartEverything,
		Desc:         "Tests Termina VM startup, container startup and other Crostini functionality",
		Contacts:     []string{"jkardatzke@chromium.org", "smbarber@chromium.org", "cros-containers-dev@google.com"},
		Attr:         []string{"informational"},
		Data:         []string{"cros-tast-tests-deb.deb"},
		Timeout:      10 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

func CrostiniStartEverything(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Close the initial new tab to set up a clean initial state for later tests
	// that want to take screenshots.
	s.Log("Trying to close initial new tab")
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://newtab/"))
	if err == nil {
		conn.CloseTarget(ctx)
		conn.Close()
	}

	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}
	defer vm.UnmountComponent(ctx)

	s.Log("Creating default container")
	cont, err := vm.CreateDefaultContainer(ctx, s.OutDir(), cr.User(), vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer vm.StopConcierge(ctx)
	defer func() {
		if err := cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failure dumping container log: ", err)
		}
	}()

	s.Log("Verifying pwd command works")
	cmd := cont.Command(ctx, "pwd")
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run pwd: ", err)
	}

	// Stop the apt-daily systemd timers since they may end up running while we
	// are executing the tests and cause failures due to resource contention.
	for _, t := range []string{"apt-daily", "apt-daily-upgrade"} {
		cmd := cont.Command(ctx, "sudo", "systemctl", "stop", t+".timer")
		if err = cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			s.Fatalf("Failed to stop %s timer: %v", t, err)
		}
	}

	// The VM and container have started up so we can now execute all of the other
	// Crostini tests. We need to be careful about this because we are going to be
	// testing multiple things in one test. This should be done so that no tests
	// have any known dependency on prior tests. If we hit a conflict at some
	// point then we will need to add functionality to save the VM/container image
	// at this point and then stop the VM/container and restore that image so we
	// can have a clean VM/container to start from again. Failures should not be
	// fatal so that all tests can get executed.
	const x11DemoAppPath = "/opt/google/cros-containers/bin/x11_demo"
	const waylandDemoAppPath = "/opt/google/cros-containers/bin/wayland_demo"

	subtestCtx, subtestCancel := ctxutil.Shorten(ctx, 15*time.Second)
	defer subtestCancel()

	// TODO(timzheng): Pass the keyboard to all other subtests that use the keyboard.
	keyboard, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard device: ", err)
	}
	defer keyboard.Close()

	subtest.Webserver(subtestCtx, s, cr, cont)

	// Opening a new browser window seems to sometimes also launch the Chromebook
	// landing page in a new tab.  Clean it up if it exists to prepare for
	// the screenshot tests.
	// TODO(dverkamp): Investigate if we can fix this in a more reliable way (https://crbug.com/938091)
	s.Log("Trying to close Chromebook landing page tab")
	findTabCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err = cr.NewConnForTarget(findTabCtx, chrome.MatchTargetURL("https://www.google.com/chromebook/"))
	if err == nil {
		conn.CloseTarget(ctx)
		conn.Close()
	}

	subtest.LaunchTerminal(subtestCtx, s, cr, cont)
	subtest.LaunchBrowser(subtestCtx, s, cr, cont)
	subtest.VerifyAppFromTerminal(subtestCtx, s, cr, cont, keyboard, "x11", x11DemoAppPath,
		colorcmp.RGB(0x99, 0xee, 0x44))
	subtest.VerifyAppFromTerminal(subtestCtx, s, cr, cont, keyboard, "wayland", waylandDemoAppPath,
		colorcmp.RGB(0x33, 0x88, 0xdd))
	subtest.AppDisplayDensity(subtestCtx, s, tconn, cont, keyboard, "x11_demo", x11DemoAppPath)
	subtest.AppDisplayDensity(subtestCtx, s, tconn, cont, keyboard, "wayland", waylandDemoAppPath)

	subtest.SyncTime(subtestCtx, s, cont)

	// Copy a test Debian package file to the container which will be used by
	// subsequent tests.
	const debianFilename = "cros-tast-tests-deb.deb"
	containerDebPath := filepath.Join("/home/testuser", debianFilename)
	err = cont.PushFile(subtestCtx, s.DataPath(debianFilename), containerDebPath)
	if err != nil {
		s.Fatal("Failed copying test Debian package to container: ", err)
	}

	subtest.LinuxPackageInfo(subtestCtx, s, cont, containerDebPath)
	err = subtest.InstallPackage(subtestCtx, cont, containerDebPath)
	if err != nil {
		s.Error("Failure in performing Linux package install: ", err)
	} else {
		// The application IDs below are generated by the code here:
		// https://cs.chromium.org/chromium/src/chrome/browser/chromeos/crostini/crostini_registry_service.cc?g=0&l=75
		// It's a modified SHA256 hash output of a concatentation of a constant,
		// the VM name, the container name and the identifier for the .desktop file
		// the app is associated with.
		const (
			x11DemoName            = "x11_demo"
			x11DemoID              = "glkpdbkfmomgogbfppaajjcgbcgaicmi"
			x11DemoFixedSizeID     = "mddfmcdnhpnhoefmmiochnnjofmfhanb"
			waylandDemoID          = "nodabfiipdopnjihbfpiengllkohmfkl"
			waylandDemoFixedSizeID = "ddlengdehbebnlegdnllbdhpjofodekl"
		)
		subtest.VerifyLauncherApp(subtestCtx, s, cr, tconn, cont.VM.Concierge.GetOwnerID(),
			x11DemoName, x11DemoID, colorcmp.RGB(0x99, 0xee, 0x44))
		subtest.AppDisplayDensityThroughLauncher(subtestCtx, s, tconn, keyboard, cont.VM.Concierge.GetOwnerID(),
			"x11_demo_fixed_size", x11DemoFixedSizeID)
		subtest.VerifyLauncherApp(subtestCtx, s, cr, tconn, cont.VM.Concierge.GetOwnerID(),
			"wayland_demo", waylandDemoID, colorcmp.RGB(0x33, 0x88, 0xdd))
		subtest.AppDisplayDensityThroughLauncher(subtestCtx, s, tconn, keyboard, cont.VM.Concierge.GetOwnerID(),
			"wayland_demo_fixed_size", waylandDemoFixedSizeID)

		subtest.UninstallApplication(subtestCtx, s, cont, cont.VM.Concierge.GetOwnerID(),
			x11DemoName, x11DemoID)
	}

	subtest.UninstallInvalidApplication(subtestCtx, s, cont)
}
