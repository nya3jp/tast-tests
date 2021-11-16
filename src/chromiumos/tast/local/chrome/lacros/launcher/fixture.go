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

// NewFixture creates a new fixture that can launch Lacros chrome with the given setup mode and
// Chrome options.
func NewFixture(mode SetupMode, fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return NewComposedFixture(mode, func(v FixtValue, pv interface{}) interface{} {
		return v
	}, fOpt)
}

// NewComposedFixture is similar to NewFixture but allows tests to customise the FixtValue
// used. This lets tests compose fixtures via struct embedding.
func NewComposedFixture(mode SetupMode, makeValue func(v FixtValue, pv interface{}) interface{},
	fOpt chrome.OptionsCallback) testing.FixtureImpl {
	return &fixtImpl{
		mode:      mode,
		fOpt:      fOpt,
		makeValue: makeValue,
	}
}

func init() {
	// lacros uses rootfs lacros, which is the recommend way to use lacros
	// in Tast tests, unless you have a specific use case for using lacros from
	// another source.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacros",
		Desc:     "Lacros Chrome from a pre-built image",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosBypassPermissions is the same as lacros but
	// camera/microphone permissions are enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosBypassPermissions",
		Desc:     "Lacros Chrome from a pre-built image with camera/microphone permissions",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--use-fake-ui-for-media-stream"),
				chrome.LacrosExtraArgs("--use-fake-ui-for-media-stream")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosWith100FakeApps is the same as lacros but
	// creates 100 fake apps that are shown in the ash-chrome launcher.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosWith100FakeApps",
		Desc:     "Lacros Chrome from a pre-built image with 100 fake apps installed",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		Parent:          "install100Apps",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosForceComposition is the same as lacros but
	// forces composition for ash-chrome.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosForceComposition",
		Desc:     "Lacros Chrome from a pre-built image with composition forced on",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--enable-hardware-overlays=\"\"")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosForceDelegation is the same as lacros but
	// forces delegated composition for ash-chrome.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosForceDelegated",
		Desc:     "Lacros Chrome from a pre-built image with delegated compositing forced on",
		Contacts: []string{"petermcneeley@chromium.org", "edcourtney@chromium.org"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.LacrosExtraArgs("--enable-gpu-memory-buffer-compositor-resources"),
				chrome.LacrosExtraArgs("--enable-features=DelegatedCompositing")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosWithArcEnabled is the same as lacros but with ARC enabled.
	// See also lacrosWithArcBooted in src/chromiumos/tast/local/arc/fixture.go.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosWithArcEnabled",
		Desc:     "Lacros Chrome from a pre-built image with ARC enabled",
		Contacts: []string{"amusbach@chromium.org", "xiyuan@chromium.org"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled()}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosUI is similar to lacros but should be used
	// by tests that will launch lacros from the ChromeOS UI (e.g shelf) instead
	// of by command line.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosUI",
		Desc:     "Lacros Chrome from a pre-built image using the UI",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosOmaha is a fixture to enable Lacros by feature flag in Chrome.
	// This does not require downloading a binary from Google Storage before the test.
	// It will use the currently available fishfood release of Lacros from Omaha.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosOmaha",
		Desc:     "Lacros Chrome from omaha",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewFixture(Omaha, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosRootfs is a fixture to bring up Lacros from the rootfs partition.
	// This does not require downloading a binary from Google Storage before the tests.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosRootfs",
		Desc:     "Lacros Chrome from rootfs",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})
}

const (
	// mojoSocketPath indicates the path of the unix socket that ash-chrome creates.
	// This unix socket is used for getting the file descriptor needed to connect mojo
	// from ash-chrome to lacros.
	mojoSocketPath = "/tmp/lacros.socket"

	// dataArtifact holds the name of the tarball which contains the lacros-chrome
	// binary.
	dataArtifact = "lacros_binary.tar"

	// LacrosSquashFSPath indicates the location of the rootfs lacros squashfs filesystem.
	LacrosSquashFSPath = "/opt/google/lacros/lacros.squash"

	// lacrosTestPath is the file path at which all lacros-chrome related test artifacts are stored.
	lacrosTestPath = "/usr/local/lacros_test_artifacts"

	// lacrosRootPath is the root directory for lacros-chrome related binaries.
	lacrosRootPath = lacrosTestPath + "/lacros_binary"
)

// The FixtValue object is made available to users of this fixture via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.FixtValue().(lacros.FixtValue)
//		...
//	}
type FixtValue interface {
	Chrome() *chrome.Chrome        // The CrOS-chrome instance.
	TestAPIConn() *chrome.TestConn // The CrOS-chrome test connection.
	Mode() SetupMode               // Mode used to get the lacros binary.
	LacrosPath() string            // Root directory for lacros-chrome.
}

// fixtValueImpl holds values related to the lacros instance and connection.
// Tests should not use this directly, unless they are composing fixtures
// and need to embed this struct in their own FixtValue. Instead, access
// needed values through the lacros FixtValue interface.
type fixtValueImpl struct {
	chrome      *chrome.Chrome
	testAPIConn *chrome.TestConn
	mode        SetupMode
	lacrosPath  string
}

// Chrome gets the CrOS-chrome instance.
func (f *fixtValueImpl) Chrome() *chrome.Chrome {
	return f.chrome
}

// TestAPIConn gets the CrOS-chrome test connection.
func (f *fixtValueImpl) TestAPIConn() *chrome.TestConn {
	return f.testAPIConn
}

// Mode gets the mode used to get the lacros binary.
func (f *fixtValueImpl) Mode() SetupMode {
	return f.mode
}

// LacrosPath gets the root directory for lacros-chrome.
func (f *fixtValueImpl) LacrosPath() string {
	return f.lacrosPath
}

// fixtImpl is a fixture that allows Lacros chrome to be launched.
type fixtImpl struct {
	mode       SetupMode                                     // How (pre exist/to be downloaded/) the container image is obtained.
	lacrosPath string                                        // Root directory for lacros-chrome, it's dynamically controlled by the lacros.skipInstallation Var.
	cr         *chrome.Chrome                                // Connection to CrOS-chrome.
	tconn      *chrome.TestConn                              // Test-connection for CrOS-chrome.
	prepared   bool                                          // Set to true if Prepare() succeeds, so that future calls can avoid unnecessary work.
	fOpt       chrome.OptionsCallback                        // Function to generate Chrome Options
	makeValue  func(v FixtValue, pv interface{}) interface{} // Closure to create FixtValue to return from SetUp. Used for composable fixtures.
}

// SetupMode describes how lacros-chrome should be set-up during the test.
// See the SetupMode constants for more explanation. Use Rootfs as a default.
// Note that if the lacrosDeployedBinary var is specified, the lacros binary
// located at the path specified by that var will be used in all cases.
type SetupMode int

const (
	// External denotes a lacros-chrome downloaded per the external data dependency.
	// This may be overridden by a pre-deployed binary by specifying the lacrosDeployedBinary Var.
	External SetupMode = iota
	// Omaha is used to get the lacros binary.
	Omaha
	// Rootfs is used to force the rootfs version of lacros-chrome. No external data dependency is needed.
	// For tests that don't care which lacros they are using, use this as a default.
	Rootfs
)

// SetUp is called by tast before each test is run. We use this method to initialize
// the fixture data, or return early if the fixture is already active.
func (f *fixtImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
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
	opts = append(opts, chrome.LacrosExtraArgs(ExtensionArgs(chrome.TestExtensionID, extList)...))

	// The main motivation of this var is to allow Chromium CI to build and deploy a fresh
	// lacros-chrome instead of always downloading from a gcs location.
	// This workaround is to be removed soon once lab provisioning is supported for Lacros.
	deployedPath, deployed := s.Var(LacrosDeployedBinary)
	if deployed {
		f.lacrosPath = deployedPath
	} else if f.mode == External {
		f.lacrosPath = lacrosRootPath
	}

	// Note that specifying the feature LacrosSupport has side-effects, so
	// we specify it even if the lacros path is being overridden by lacrosDeployedBinary.
	if f.mode == Rootfs {
		opts = append(opts, chrome.EnableFeatures("LacrosSupport", "ForceProfileMigrationCompletion"),
			chrome.ExtraArgs("--lacros-selection=rootfs"))
	} else if f.mode == Omaha {
		opts = append(opts, chrome.EnableFeatures("LacrosSupport", "ForceProfileMigrationCompletion"),
			chrome.ExtraArgs("--lacros-selection=stateful"))
	}

	// If External or deployed we should specify the path.
	// This will override the lacros-selection argument.
	if deployed || f.mode == External {
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

	// Prepare the lacros binary if it isn't deployed already via lacrosDeployedBinary.
	if !deployed {
		switch f.mode {
		case External:
			if err := prepareLacrosChromeBinary(ctx, s); err != nil {
				s.Fatal("Failed to prepare lacros-chrome, err")
			}
		case Omaha:
			// When launched by Omaha we need to wait several seconds for lacros to be launchable.
			// It is ready when the image loader path is created with the chrome executable.
			testing.ContextLog(ctx, "Waiting for Lacros to initialize")
			matches, err := f.waitForPathToExist(ctx, "/run/imageloader/lacros-dogfood*/*/chrome")
			if err != nil {
				s.Fatal("Failed to find lacros binary: ", err)
			}
			f.lacrosPath = filepath.Dir(matches[0])
		case Rootfs:
			// When launched from the rootfs partition, the lacros-chrome is already located
			// at /opt/google/lacros/lacros.squash in the OS, will be mounted at /run/lacros/.
			matches, err := f.waitForPathToExist(ctx, "/run/lacros/chrome")
			if err != nil {
				s.Fatal("Failed to find lacros binary: ", err)
			}
			f.lacrosPath = filepath.Dir(matches[0])
		default:
			s.Fatal("Unrecognized mode: ", f.mode)
		}
	}

	testing.ContextLog(ctx, "Using lacros located at ", f.lacrosPath)

	val := f.buildFixtData(ctx, s)
	chrome.Lock()
	f.prepared = true
	shouldClose = false
	return f.makeValue(val, s.ParentValue())
}

// TearDown is called after all tests involving this fixture have been run,
// (or failed to be run if the fixture itself fails). Unlocks Chrome's and
// the container's constructors.
func (f *fixtImpl) TearDown(ctx context.Context, s *testing.FixtState) {
	ctx, st := timing.Start(ctx, "TearDown")
	defer st.End()

	chrome.Unlock()
	f.cleanUp(ctx, s)
}

func (f *fixtImpl) Reset(ctx context.Context) error {
	if err := f.cr.Responded(ctx); err != nil {
		return errors.Wrap(err, "existing Chrome connection is unusable")
	}
	if err := f.cr.ResetState(ctx); err != nil {
		return errors.Wrap(err, "failed resetting existing Chrome session")
	}
	return nil
}

func (f *fixtImpl) PreTest(ctx context.Context, s *testing.FixtTestState) {}

func (f *fixtImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {}

// cleanUp de-initializes the fixture by closing/cleaning-up the relevant
// fields and resetting the struct's fields.
func (f *fixtImpl) cleanUp(ctx context.Context, s *testing.FixtState) {
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
func (f *fixtImpl) buildFixtData(ctx context.Context, s *testing.FixtState) *fixtValueImpl {
	if err := f.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return &fixtValueImpl{f.cr, f.tconn, f.mode, f.lacrosPath}
}

// waitForPathToExist is a helper method that waits the given binary path to be present
// then returns the matching paths or it will be timed out if the ctx's timeout is reached.
func (f *fixtImpl) waitForPathToExist(ctx context.Context, pattern string) (matches []string, err error) {
	return matches, testing.Poll(ctx, func(ctx context.Context) error {
		m, err := filepath.Glob(pattern)
		if err != nil {
			return errors.Wrapf(err, "binary path does not exist yet. expected: %v", pattern)
		}
		if len(m) == 0 {
			return errors.New("binary path does not exist yet. expected: " + pattern)
		}
		matches = append(matches, m...)
		return nil
	}, &testing.PollOptions{Interval: 5 * time.Second})
}
