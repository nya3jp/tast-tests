// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	cui "chromiumos/tast/local/crostini/ui"
	"chromiumos/tast/local/crostini/ui/terminalapp"
	dlcutil "chromiumos/tast/local/dlc"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	installationTimeout   = 7 * time.Minute
	checkContainerTimeout = time.Minute
	postTestTimeout       = 30 * time.Second
	cninstallationTimeout = time.Minute
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBuster",
		Desc:            "Install Crostini with Buster",
		Contacts:        []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Impl:            &crostiniFixture{preData: preTestDataBuster},
		SetUpTimeout:    chrome.LoginTimeout + installationTimeout,
		ResetTimeout:    chrome.ResetTimeout + checkContainerTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: chrome.ResetTimeout + cninstallationTimeout,

		// TODO (jinrongwu): switch to Global RunTime Variable when deprecating pre.go.
		Vars: []string{"keepState"},
		Data: []string{"crostini_test_container_metadata_buster_amd64.tar.xz", "crostini_test_container_rootfs_buster_amd64.tar.xz"},
	})

}

// preTestData contains the data to set up the fixture.
type preTestData struct {
	loginType     loginType
	container     containerType
	debianVersion vm.ContainerDebianVersion
	startedOK     bool
}

// crostiniFixture holds the runtime state of the fixture.
type crostiniFixture struct {
	cr       *chrome.Chrome
	tconn    *chrome.TestConn
	cont     *vm.Container
	kb       *input.KeyboardEventWriter
	preData  *preTestData
	postData *PostTestData
}

// FixtureData is the data returned by SetUp and passed to tests.
type FixtureData struct {
	Chrome   *chrome.Chrome
	Tconn    *chrome.TestConn
	Cont     *vm.Container
	KB       *input.KeyboardEventWriter
	PostData *PostTestData
}

var preTestDataBuster = &preTestData{
	container:     normal,
	debianVersion: vm.DebianBuster,
}

func (f *crostiniFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	f.postData = &PostTestData{}

	// If initialization fails, this defer is used to clean-up the partially-initialized pre
	// and copies over lxc + container boot logs.
	// Stolen verbatim from arc/pre.go
	shouldClose := true
	defer func() {
		if shouldClose {
			// TODO (jinrongwu): use FixtureData instead of PreData and modify RunCrostiniPostTest when deprecating pre.go.
			RunCrostiniPostTest(ctx, PreData{f.cr, f.tconn, f.cont, f.kb, f.postData})
			f.cleanUp(ctx, s)
		}
	}()

	opts := []chrome.Option{chrome.ARCDisabled()}

	// Enable ARC++ if it is supported. We do this on every
	// supported device because some tests rely on it and this
	// lets us reduce the number of distinct fixture. If
	// your test relies on ARC++ you should add an appropriate
	// software dependency.
	if arc.Supported() {
		if f.preData.loginType == loginGaia {
			opts = []chrome.Option{chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...)}
		} else {
			opts = []chrome.Option{chrome.ARCEnabled()}
		}
	}
	opts = append(opts, chrome.ExtraArgs("--vmodule=crostini*=1"))

	opts = append(opts, chrome.EnableFeatures("KernelnextVMs"))

	// To help identify sources of flake, we report disk usage before the test.
	if err := reportDiskUsage(ctx); err != nil {
		s.Log("Failed to gather disk usage: ", err)
	}

	if f.preData.loginType == loginGaia {
		opts = append(opts, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")))
	}

	useLocalImage := checkKeepState(s) && terminaDLCAvailable()
	if useLocalImage {
		// Retain the user's cryptohome directory and previously installed VM.
		opts = append(opts, chrome.KeepState())
	}
	var err error
	if f.cr, err = chrome.New(ctx, opts...); err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}

	if f.tconn, err = f.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, f.tconn)

	if f.kb, err = input.Keyboard(ctx); err != nil {
		s.Fatal("Failed to create keyboard device: ", err)
	}

	if useLocalImage {
		s.Log("keepState attempting to start the existing VM and container by launching Terminal")
		if err = f.launchExitTerminal(ctx); err != nil {
			s.Fatal("KeepState error: ", err)
		}
	} else {
		// Install Crostini.
		iOptions := GetInstallerOptions(s, f.preData.debianVersion, f.preData.container == largeContainer, f.cr.NormalizedUser())
		if _, err := cui.InstallCrostini(ctx, f.tconn, f.cr, iOptions); err != nil {
			s.Fatal("Failed to install Crostini: ", err)
		}
	}

	f.cont, err = vm.DefaultContainer(ctx, f.cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to connect to running container: ", err)
	}

	// Report disk size again after successful install.
	if err := reportDiskUsage(ctx); err != nil {
		s.Log("Failed to gather disk usage: ", err)
	}

	f.preData.startedOK = true

	chrome.Lock()
	vm.Lock()
	shouldClose = false
	if err := f.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return FixtureData{f.cr, f.tconn, f.cont, f.kb, f.postData}
}

func (f *crostiniFixture) TearDown(ctx context.Context, s *testing.FixtState) {
	f.close(ctx, s)
}

func (f *crostiniFixture) Reset(ctx context.Context) error {
	// Check container.
	// It returns error in the following situations:
	// 1. no container
	// 2. container does not work
	// 3. chrome is not responsive
	// 4. fail to close any open window
	// 5. fail to reset chrome.
	// Otherwise, return nil.
	if f.cont == nil {
		return errors.New("There is no container")
	}
	if err := BasicCommandWorks(ctx, f.cont); err != nil {
		return errors.Wrap(err, "failed to check basic commands in the existing container")
	} else if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "chrome is unresponsive")
	}

	if err := ash.CloseAllWindow(ctx, f.tconn); err != nil {
		return errors.Wrap(err, "failed to close all windows")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed to reset chrome's state")
	}

	return nil
}

func (f *crostiniFixture) PreTest(ctx context.Context, s *testing.FixtTestState) {
}

func (f *crostiniFixture) PostTest(ctx context.Context, s *testing.FixtTestState) {
	// TODO (jinrongwu): use FixtureData instead of PreData and modify RunCrostiniPostTest when deprecating pre.go.
	RunCrostiniPostTest(ctx, PreData{f.cr, f.tconn, f.cont, f.kb, f.postData})
}

func (f *crostiniFixture) close(ctx context.Context, s *testing.FixtState) {
	vm.Unlock()
	chrome.Unlock()
	f.cleanUp(ctx, s)
}

// cleanUp de-initializes the fixture by closing/cleaning-up the relevant
// fields and resetting the struct's fields.
func (f *crostiniFixture) cleanUp(ctx context.Context, s *testing.FixtState) {
	if f.kb != nil {
		if err := f.kb.Close(); err != nil {
			s.Log("Failure closing keyboard: ", err)
		}
		f.kb = nil
	}

	if f.postData.vmLogReader != nil {
		if err := f.postData.vmLogReader.Close(); err != nil {
			s.Log("Failed to close VM log reader: ", err)
		}
	}

	// Don't uninstall crostini or delete the image for keepState so that
	// crostini is still running after the test, and the image can be reused.
	if checkKeepState(s) && f.preData.startedOK {
		s.Log("keepState not uninstalling Crostini and deleting image in cleanUp")
	} else {
		if f.cont != nil {
			if err := uninstallLinuxFromUI(ctx, f.tconn, f.cr); err != nil {
				s.Log("Failed to uninstall Linux: ", err)
			}
			f.cont = nil
		}

		// Unmount the VM image to prevent later tests from
		// using it by accident. Otherwise we may have a dlc
		// test use the component or vice versa.
		if err := dlcutil.Uninstall(ctx, "termina-dlc"); err != nil {
			s.Error("Failed to unmount termina-dlc: ", err)
		}

		if err := vm.DeleteImages(); err != nil {
			s.Log("Error deleting images: ", err)
		}
	}
	f.preData.startedOK = false

	// Nothing special needs to be done to close the test API connection.
	f.tconn = nil

	if f.cr != nil {
		if err := f.cr.Close(ctx); err != nil {
			s.Log("Failure closing chrome: ", err)
		}
		f.cr = nil
	}
}

func (f *crostiniFixture) launchExitTerminal(ctx context.Context) error {
	terminalApp, err := terminalapp.Launch(ctx, f.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to launch Terminal")
	}
	if err = terminalApp.Exit(f.kb)(ctx); err != nil {
		return errors.Wrap(err, "failed to exit Terminal window")
	}
	return nil
}

// checkKeepState returns whether the fixture should keep state from the
// previous test execution and try to recycle the VM.
func checkKeepState(s *testing.FixtState) bool {
	if str, ok := s.Var("keepState"); ok {
		b, err := strconv.ParseBool(str)
		if err != nil {
			s.Fatalf("Cannot parse argument %q to keepState: %v", str, err)
		}
		return b
	}
	return false
}
