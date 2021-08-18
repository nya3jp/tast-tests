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

// Constants for the global runtime Tast Vars.
const (
	// LacrosDeployedBinary contains the Fixture Var necessary to run lacros.
	// This should be used by any lacros fixtures defined outside this file.
	LacrosDeployedBinary = "lacrosDeployedBinary"

	// LacrosSelection contains the Fixture Var necessary to select lacros
	// between 'stateful', 'rootfs' and 'external' for the binary passed in with -lacrosDeployedBinary or from the GCS path in the lacros_binary.tar.external file
	// If not specified, it will default to 'external'.
	LacrosSelection = "lacrosSelection"
)

// NewStartedByVars creates a new fixture that can launch Lacros chrome specified with the Var lacrosSelection in runtime.
// Note: NewStartedByVars is recommended over NewStartedByData in that it allows the same test to be run for multiple setup modes without adding boilerplate.
func NewStartedByVars(fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return &fixtureImpl{
		mode: Runtime,
		fOpt: fOpt,
	}
}

// NewStartedByData creates a new fixture that can launch Lacros chrome with the given setup mode and
// Chrome options.
// TODO(hyungtaekim): Rename to NewStartedByMode
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

	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosStarted",
		Desc:     "Open Lacros Chrome from the binary specified with the runtime Var lacrosSelection: (1) rootfs (default), (2) omaha or (3) external binary associated with the Var lacrosDeployedBinary",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: NewStartedByVars(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosSelection, LacrosDeployedBinary},
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
	lacrosPath string                 // Root directory for lacros-chrome.
	cr         *chrome.Chrome         // Connection to CrOS-chrome.
	tconn      *chrome.TestConn       // Test-connection for CrOS-chrome.
	prepared   bool                   // Set to true if Prepare() succeeds, so that future calls can avoid unnecessary work.
	fOpt       chrome.OptionsCallback // Function to generate Chrome Options
}

// SetupMode describes how lacros-chrome should be set-up during the test. See the SetupMode constants for more explanation.
type SetupMode string

const (
	// Runtime denotes that the setup mode will be determined by the vars passed in at runtime. This could be one of the sources below - external|omaha|rootfs.
	Runtime = "runtime"
	// PreExist denotes that Lacros-chrome is set up from external source. It can be already downloaded per the
	// external data dependency or pre-deployed by the caller site that invokes the tests.
	// TODO(hyungtaekim): Replace 'PreExist' with 'External' to avoid confusion
	PreExist = "external"
	// Omaha is used to get the lacros binary.
	Omaha = "omaha"
	// Rootfs is used to force the rootfs version of lacros-chrome. No external data dependency is needed.
	Rootfs = "rootfs"
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

	// Check runtime Vars to determine the Lacros binary to select and extra Chrome options to use.
	f.mode, opts = f.checkVars(ctx, s, opts...)

	opts = append(opts, chrome.ExtraArgs("--lacros-mojo-socket-for-testing="+mojoSocketPath))

	// We reuse the custom extension from the chrome package for exposing private interfaces.
	extDirs, err := chrome.DeprecatedPrepareExtensions()
	if err != nil {
		s.Fatal("Failed to prepare extensions: ", err)
	}
	extList := strings.Join(extDirs, ",")
	opts = append(opts, chrome.LacrosExtraArgs(extensionArgs(chrome.TestExtensionID, extList)...))

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
		path, deployed := s.Var(LacrosDeployedBinary)
		if !deployed {
			f.lacrosPath = lacrosRootPath
			if err := prepareLacrosChromeBinary(ctx, s); err != nil {
				s.Fatal("Failed to prepare lacros-chrome, err")
			}
		} else {
			f.lacrosPath = path
		}
	case Omaha:
		// When launched by Omaha we need to wait several seconds for lacros to be launchable.
		// It is ready when the image loader path is created with the chrome executable.
		testing.ContextLog(ctx, "Waiting for Lacros to initialize")
		if f.lacrosPath, err = f.waitForMounted(ctx, "/run/imageloader/lacros-dogfood*/*/chrome"); err != nil {
			s.Fatal("Failed to find lacros binary: ", err)
		}
	case Rootfs:
		// When launched from the rootfs partition, the lacros-chrome is already located
		// at /opt/google/lacros/lacros.squash in the OS, will be mounted at /run/lacros/.
		if f.lacrosPath, err = f.waitForMounted(ctx, "/run/lacros/chrome"); err != nil {
			s.Fatal("Failed to find lacros binary: ", err)
		}
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

// waitForMounted is a helper method that monitors the given binary path pattern and
// returns the directory of the matching pattern or an error if the path matching pattern doesn't exist or timed out if the ctx's timeout is reached.
func (f *fixtureImpl) waitForMounted(ctx context.Context, pattern string) (dir string, err error) {
	return dir, testing.Poll(ctx, func(ctx context.Context) error {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return errors.Wrapf(err, "binary path does not exist yet. expected: %v", pattern)
		}
		if len(matches) == 0 {
			return errors.New("binary path does not exist yet. expected: " + pattern)
		}
		dir = filepath.Dir(matches[0])
		return nil
	}, &testing.PollOptions{Interval: 5 * time.Second})
}

// buildFixtData is a helper method that resets the machine state in
// advance of building the fixture data for the actual tests.
func (f *fixtureImpl) buildFixtData(ctx context.Context, s *testing.FixtState) FixtData {
	if err := f.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return FixtData{f.cr, f.tconn, f.mode, f.lacrosPath}
}

// checkVars reads the Fixture Vars at runtime to determine the SetupMode and extra Chrome options to add.
func (f *fixtureImpl) checkVars(ctx context.Context, s *testing.FixtState, opts ...chrome.Option) (SetupMode, []chrome.Option) {
	// Reads the runtime Var only when f.mode is not specified by users.
	if f.mode != Runtime {
		return f.mode, opts
	}

	var mode SetupMode
	selection, _ := s.Var(LacrosSelection)
	switch selection {
	case PreExist:
		mode = PreExist
		// TODO: Consider an option to decide whether to open by the launcher script or from the Shelf UI.
		opts = append(opts, chrome.EnableFeatures("LacrosSupport"))
	case Rootfs:
		mode = Rootfs
		opts = append(opts, chrome.EnableFeatures("LacrosSupport"), chrome.ExtraArgs("--lacros-selection=rootfs"))
	case Omaha:
		mode = Omaha
		opts = append(opts, chrome.EnableFeatures("LacrosSupport"), chrome.ExtraArgs("--lacros-selection=stateful"))
	default:
		s.Log("Set the Var lacrosSelection to 'external' by default. expected: external|omaha|rootfs, but got: ", selection)
		mode = PreExist
	}
	s.Logf("Vars: %v=%v", LacrosSelection, selection)
	return mode, opts
}
