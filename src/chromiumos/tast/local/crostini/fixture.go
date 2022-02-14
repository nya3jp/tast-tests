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
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
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
	uninstallationTimeout = time.Minute
)

func init() {
	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInForCrostini",
		Desc:     "Logged into a session",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			opts := generateChromeOpts(s)
			// Enable ARC++ if it is supported. We do this on every
			// supported device because some tests rely on it and this
			// lets us reduce the number of distinct fixture. If
			// your test relies on ARC++ you should add an appropriate
			// software dependency.
			if arc.Supported() {
				opts = append(opts, chrome.ARCEnabled())
			} else {
				opts = append(opts, chrome.ARCDisabled())
			}
			return opts, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"keepState"},
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInWithGaiaForCrostini",
		Desc:     "Logged into a session with Gaia user",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			opts := generateChromeOpts(s)
			if arc.Supported() {
				opts = append(opts, chrome.ARCSupported())
				opts = append(opts, chrome.ExtraArgs(arc.DisableSyncFlags()...))

			} else {
				opts = append(opts, chrome.ARCDisabled())
			}
			return append(opts, chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault"))), nil
		}),
		SetUpTimeout:    chrome.GAIALoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"ui.gaiaPoolDefault", "keepState"},
	})

	testing.AddFixture(&testing.Fixture{
		Name:     "chromeLoggedInForCrostiniWithLacros",
		Desc:     "Logged into a session and enable Lacros",
		Contacts: []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Impl: lacrosfixt.NewFixture(lacrosfixt.Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			opts := generateChromeOpts(s)
			if arc.Supported() {
				opts = append(opts, chrome.ARCEnabled())
			} else {
				opts = append(opts, chrome.ARCDisabled())
			}
			opts = append(opts, chrome.EnableFeatures("LacrosPrimary"),
				chrome.ExtraArgs("--disable-lacros-keep-alive"))
			return opts, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{"keepState", lacrosfixt.LacrosDeployedBinary},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBuster",
		Desc:            "Install Crostini with Buster",
		Contacts:        []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Impl:            &crostiniFixture{preData: preTestDataBuster},
		SetUpTimeout:    installationTimeout,
		ResetTimeout:    checkContainerTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: uninstallationTimeout,
		Parent:          "chromeLoggedInForCrostini",

		// TODO (jinrongwu): switch to Global RunTime Variable when deprecating pre.go.
		// The same for the rest keepState var.
		Vars: []string{"keepState"},
		Data: []string{GetContainerMetadataArtifact("buster", false), GetContainerRootfsArtifact("buster", false)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBullseye",
		Desc:            "Install Crostini with Bullseye",
		Contacts:        []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Impl:            &crostiniFixture{preData: preTestDataBullseye},
		SetUpTimeout:    installationTimeout,
		ResetTimeout:    checkContainerTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: uninstallationTimeout,
		Parent:          "chromeLoggedInForCrostini",
		Vars:            []string{"keepState"},
		Data:            []string{GetContainerMetadataArtifact("bullseye", false), GetContainerRootfsArtifact("bullseye", false)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBusterGaia",
		Desc:            "Install Crostini with Buster in Chrome logged in with Gaia",
		Contacts:        []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Impl:            &crostiniFixture{preData: preTestDataBuster},
		SetUpTimeout:    installationTimeout,
		ResetTimeout:    checkContainerTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: uninstallationTimeout,
		Parent:          "chromeLoggedInWithGaiaForCrostini",
		Vars:            []string{"keepState", "ui.gaiaPoolDefault"},
		Data:            []string{GetContainerMetadataArtifact("buster", false), GetContainerRootfsArtifact("buster", false)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBullseyeGaia",
		Desc:            "Install Crostini with Bullseye in Chrome logged in with Gaia",
		Contacts:        []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Impl:            &crostiniFixture{preData: preTestDataBullseye},
		SetUpTimeout:    installationTimeout,
		ResetTimeout:    checkContainerTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: uninstallationTimeout,
		Parent:          "chromeLoggedInWithGaiaForCrostini",
		Vars:            []string{"keepState", "ui.gaiaPoolDefault"},
		Data:            []string{GetContainerMetadataArtifact("bullseye", false), GetContainerRootfsArtifact("bullseye", false)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBullseyeLargeContainer",
		Desc:            "Install Crostini with Bullseye in large container with apps installed",
		Contacts:        []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Impl:            &crostiniFixture{preData: preTestDataBullseyeLC},
		SetUpTimeout:    installationTimeout,
		ResetTimeout:    checkContainerTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: uninstallationTimeout,
		Parent:          "chromeLoggedInForCrostini",
		Vars:            []string{"keepState"},
		Data:            []string{GetContainerMetadataArtifact("bullseye", true), GetContainerRootfsArtifact("bullseye", true)},
	})

	testing.AddFixture(&testing.Fixture{
		Name:            "crostiniBullseyeWithLacros",
		Desc:            "Install Crostini with Bullseye and enable Lacros",
		Contacts:        []string{"jinrongwu@google.com", "cros-containers-dev@google.com"},
		Impl:            &crostiniFixture{preData: preTestDataBullseye},
		SetUpTimeout:    installationTimeout,
		ResetTimeout:    checkContainerTimeout,
		PostTestTimeout: postTestTimeout,
		TearDownTimeout: uninstallationTimeout,
		Parent:          "chromeLoggedInForCrostiniWithLacros",
		Vars:            []string{"keepState"},
		Data:            []string{GetContainerMetadataArtifact("bullseye", false), GetContainerRootfsArtifact("bullseye", false)},
	})

}

// preTestData contains the data to set up the fixture.
type preTestData struct {
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
	ParentFixtV chrome.HasChrome
	Tconn       *chrome.TestConn
	Cont        *vm.Container
	KB          *input.KeyboardEventWriter
	PostData    *PostTestData
}

var preTestDataBuster = &preTestData{
	container:     normal,
	debianVersion: vm.DebianBuster,
}

var preTestDataBullseye = &preTestData{
	container:     normal,
	debianVersion: vm.DebianBullseye,
}

var preTestDataBullseyeLC = &preTestData{
	container:     largeContainer,
	debianVersion: vm.DebianBullseye,
}

func (f *crostiniFixture) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	f.postData = &PostTestData{}
	f.cr = s.ParentValue().(chrome.HasChrome).Chrome()

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

	// To help identify sources of flake, we report disk usage before the test.
	if err := reportDiskUsage(ctx); err != nil {
		s.Log("Failed to gather disk usage: ", err)
	}

	var err error
	if lv, ok := s.ParentValue().(lacrosfixt.FixtValue); ok {
		f.tconn = lv.TestAPIConn()
	} else {
		if f.tconn, err = f.cr.TestAPIConn(ctx); err != nil {
			s.Fatal("Failed to create test API connection: ", err)
		}
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, f.tconn)

	if f.kb, err = input.Keyboard(ctx); err != nil {
		s.Fatal("Failed to create keyboard device: ", err)
	}

	if checkKeepState(s) && terminaDLCAvailable() {
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

	vm.Lock()
	shouldClose = false
	if err := f.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}

	return FixtureData{s.ParentValue().(chrome.HasChrome), f.tconn, f.cont, f.kb, f.postData}
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
	// 4. fail to reset chrome.
	// Note that 3 and 4 is already done by the parent fixture.
	// Otherwise, return nil.
	if f.cont == nil {
		return errors.New("There is no container")
	}
	if err := BasicCommandWorks(ctx, f.cont); err != nil {
		return errors.Wrap(err, "failed to check basic commands in the existing container")
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

	f.cr = nil
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

// generateChromeOpts generates common chrome options for crostini fixtures.
func generateChromeOpts(s *testing.FixtState) []chrome.Option {
	opts := []chrome.Option{chrome.ExtraArgs("--vmodule=crostini*=1", "--disable-login-lacros-opening"), chrome.EnableFeatures("KernelnextVMs")}

	useLocalImage := checkKeepState(s) && terminaDLCAvailable()
	if useLocalImage {
		// Retain the user's cryptohome directory and previously installed VM.
		opts = append(opts, chrome.KeepState())
	}

	return opts
}
