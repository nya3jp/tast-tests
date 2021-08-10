// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// LacrosDeployedBinary contains the Fixture Var necessary to run lacros.
// This should be used by any lacros fixtures defined outside this file.
const LacrosDeployedBinary = "lacrosDeployedBinary"

// NewStartedByData creates a new fixture that can launch Lacros chrome with the given setup mode and
// Chrome options.
func NewStartedByData(mode SetupMode, fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return &fixtureImpl{
		mode: mode,
		fOpt: fOpt,
	}
}

func init() {
	// lacrosStartedByData uses a pre-built image downloaded from cloud storage as a
	// data-dependency. This fixture should be used by tests that start lacros from the lacros/launcher package.
	// To use this fixture you must have lacros.DataArtifact as a data dependency.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosStartedByData",
		Desc:     "Lacros Chrome from a pre-built image",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewStartedByData(PreExist, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosStartedByDataBypassPermissions is the same as lacrosStartedByData but
	// camera/microphone permissions are enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosStartedByDataBypassPermissions",
		Desc:     "Lacros Chrome from a pre-built image with camera/microphone permissions",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewStartedByData(PreExist, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--use-fake-ui-for-media-stream"),
				chrome.LacrosExtraArgs("--use-fake-ui-for-media-stream")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosStartedByDataWith100FakeApps is the same as lacrosStartedByData but
	// creates 100 fake apps that are shown in the ash-chrome launcher.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosStartedByDataWith100FakeApps",
		Desc:     "Lacros Chrome from a pre-built image with 100 fake apps installed",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewStartedByData(PreExist, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		Parent:          "install100Apps",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosStartedByDataForceComposition is the same as lacrosStartedByData but
	// forces composition for ash-chrome.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosStartedByDataForceComposition",
		Desc:     "Lacros Chrome from a pre-built image with composition forced on",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewStartedByData(PreExist, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--enable-hardware-overlays=\"\"")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosStartedByDataUI is similar to lacrosStartedByData but should be used
	// by tests that will launch lacros from the ChromeOS UI (e.g shelf) instead
	// of by command line. To use this fixture you must have
	// lacros.DataArtifact as a data dependency.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosStartedByDataUI",
		Desc:     "Lacros Chrome from a pre-built image using the UI",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewStartedByData(PreExist, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.EnableFeatures("LacrosSupport")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosStartedByOmaha is a fixture to enable Lacros by feature flag in Chrome.
	// This does not require downloading a binary from Google Storage before the test.
	// It will use the currently available fishfood release of Lacros from Omaha.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosStartedByOmaha",
		Desc:     "Lacros Chrome from omaha",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewStartedByData(Omaha, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.EnableFeatures("LacrosSupport"),
				chrome.ExtraArgs("--lacros-selection=stateful"),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosStartedFromRootfs is a fixture to bring up Lacros from the rootfs partition.
	// This does not require downloading a binary from Google Storage before the tests.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosStartedFromRootfs",
		Desc:     "Lacros Chrome from rootfs",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: NewStartedByData(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{
				chrome.EnableFeatures("LacrosSupport"),
				chrome.ExtraArgs("--lacros-selection=rootfs"),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})
}

const (
	// mojoSocketPath indicates the path of the unix socket that ash-chrome creates.
	// This unix socket is used for getting the file descriptor needed to connect mojo
	// from ash-chrome to lacros.
	mojoSocketPath = "/tmp/lacros.socket"

	// DataArtifact holds the name of the tarball which contains the lacros-chrome
	// binary. When using the lacrosStartedByData fixture, you must list this as one
	// of the data dependencies of your test.
	DataArtifact = "lacros_binary.tar"

	// LacrosSquashFSPath indicates the location of the rootfs lacros squashfs filesystem.
	LacrosSquashFSPath = "/opt/google/lacros/lacros.squash"

	// lacrosTestPath is the file path at which all lacros-chrome related test artifacts are stored.
	lacrosTestPath = "/usr/local/lacros_test_artifacts"

	// lacrosRootPath is the root directory for lacros-chrome related binaries.
	lacrosRootPath = lacrosTestPath + "/lacros_binary"
)

// The FixtData object is made available to users of this fixture via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.FixtValue().(lacros.FixtData)
//		...
//	}
type FixtData struct {
	Chrome      *chrome.Chrome   // The CrOS-chrome instance.
	TestAPIConn *chrome.TestConn // The CrOS-chrome connection.
	Mode        SetupMode        // Mode used to get the lacros binary.
	LacrosPath  string           // Root directory for lacros-chrome.
}

// fixtureImpl is a fixture that allows Lacros chrome to be launched.
type fixtureImpl struct {
	mode       SetupMode              // How (pre exist/to be downloaded/) the container image is obtained.
	lacrosPath string                 // Root directory for lacros-chrome, it's dynamically controlled by the lacros.skipInstallation Var.
	cr         *chrome.Chrome         // Connection to CrOS-chrome.
	tconn      *chrome.TestConn       // Test-connection for CrOS-chrome.
	prepared   bool                   // Set to true if Prepare() succeeds, so that future calls can avoid unnecessary work.
	fOpt       chrome.OptionsCallback // Function to generate Chrome Options
}

// SetupMode describes how lacros-chrome should be set-up during the test. See the SetupMode constants for more explanation.
type SetupMode int

const (
	// PreExist denotes that Lacros-chrome already exists during the fixture. It can be already downloaded per the
	// external data dependency or pre-deployed by the caller site that invokes the tests.
	PreExist SetupMode = iota
	// Omaha is used to get the lacros binary.
	Omaha
	// Rootfs is used to force the rootfs version of lacros-chrome. No external data dependency is needed.
	Rootfs
)

// SetUp is called by tast before each test is run. We use this method to initialize
// the fixture data, or return early if the fixture is already active.
// TODO(crbug.com/1127165): Until this bug is resolved, tests must call EnsureLacrosChrome
// at the beginning of their test.
func (f *fixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	ctx, st := timing.Start(ctx, "SetUp")
	defer st.End()

	// Currently we assume the fixture wouldn't be broken, and returns
	// existing fixture data immediately without checking.
	// TODO(crbug.com/1176087): Check whether the current environment is reusable, and if not
	// reset the state.
	if f.prepared {
		s.Logf("Fixture has already been prepared. Returning a cached one. mode: %v, lacros path: %v", f.mode, f.lacrosPath)
		return f.buildFixtData(ctx, s)
	}

	// If initialization fails, this defer is used to clean-up the partially-initialized pre.
	// Stolen verbatim from arc/pre.go
	shouldClose := true
	defer func() {
		if shouldClose {
			f.cleanUp(ctx, s)
		}
	}()

	opts, err := f.fOpt(ctx, s)
	if err != nil {
		s.Fatal("Failed to obtain fixture options: ", err)
	}

	opts = append(opts, chrome.ExtraArgs("--lacros-mojo-socket-for-testing="+mojoSocketPath))

	// We reuse the custom extension from the chrome package for exposing private interfaces.
	// TODO(hidehiko): Set up Tast test extension for lacros-chrome.
	extDirs, err := chrome.DeprecatedPrepareExtensions()
	if err != nil {
		s.Fatal("Failed to prepare extensions: ", err)
	}
	extList := strings.Join(extDirs, ",")
	opts = append(opts, chrome.LacrosExtraArgs(extensionArgs(chrome.TestExtensionID, extList)...))

	deployed := false
	if f.mode == PreExist {
		// The main motivation of this var is to allow Chromium CI to build and deploy a fresh
		// lacros-chrome instead of always downloading from a gcs location.
		// This workaround is to be removed soon once lab provisioning is supported for Lacros.
		var path string
		path, deployed = s.Var(LacrosDeployedBinary)
		if deployed {
			f.lacrosPath = path
		} else {
			f.lacrosPath = lacrosRootPath
		}
		opts = append(opts, chrome.ExtraArgs("--lacros-chrome-path="+f.lacrosPath))
	}

	// If there's a parent fixture and the fixture supplies extra options, use them.
	if extraOpts, ok := s.ParentValue().([]chrome.Option); ok {
		opts = append(opts, extraOpts...)
	}

	if f.cr, err = chrome.New(ctx, opts...); err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	if f.tconn, err = f.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	switch f.mode {
	case PreExist:
		if !deployed {
			if err := prepareLacrosChromeBinary(ctx, s); err != nil {
				s.Fatal("Failed to prepare lacros-chrome, err")
			}
		}
	case Omaha:
		// When launched by Omaha we need to wait several seconds for lacros to be launchable.
		// It is ready when the image loader path is created with the chrome executable.
		testing.ContextLog(ctx, "Waiting for Lacros to initialize")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			matches, err := filepath.Glob("/run/imageloader/lacros-dogfood*/*/chrome")
			if err != nil {
				return errors.Wrap(err, "binary path does not exist yet")
			}
			if len(matches) == 0 {
				return errors.New("binary path does not exist yet")
			}
			f.lacrosPath = matches[0]
			return nil
		}, &testing.PollOptions{Interval: 5 * time.Second}); err != nil {
			s.Fatal("Failed to find lacros binary: ", err)
		}
	case Rootfs:
		// When launched from the rootfs partition, the lacros-chrome is already located
		// at /opt/google/lacros/lacros.squash in the OS, will be mounted at /run/lacros/.
		f.lacrosPath = "/run/lacros"
	default:
		s.Fatal("Unrecognized mode: ", f.mode)
	}

	ret := f.buildFixtData(ctx, s)
	chrome.Lock()
	f.prepared = true
	shouldClose = false
	return ret
}

// TearDown is called after all tests involving this fixture have been run,
// (or failed to be run if the fixture itself fails). Unlocks Chrome's and
// the container's constructors.
func (f *fixtureImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	ctx, st := timing.Start(ctx, "TearDown")
	defer st.End()

	chrome.Unlock()
	f.cleanUp(ctx, s)
}

func (f *fixtureImpl) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *fixtureImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fixtureImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// cleanUp de-initializes the fixture by closing/cleaning-up the relevant
// fields and resetting the struct's fields.
func (f *fixtureImpl) cleanUp(ctx context.Context, s *testing.FixtState) {
	// Nothing special needs to be done to close the test API connection.
	f.tconn = nil

	if f.cr != nil {
		if err := f.cr.Close(ctx); err != nil {
			s.Error("Failure closing chrome: ", err)
		}
		f.cr = nil
	}

}

// buildFixtData is a helper method that resets the machine state in
// advance of building the fixture data for the actual tests.
func (f *fixtureImpl) buildFixtData(ctx context.Context, s *testing.FixtState) FixtData {
	if err := f.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return FixtData{f.cr, f.tconn, f.mode, f.lacrosPath}
}
