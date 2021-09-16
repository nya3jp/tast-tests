// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/policy/fakedms"
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
	return &fixtImpl{
		mode: mode,
		fOpt: fOpt,
	}
}

func init() {
	// lacros uses a pre-built image downloaded from cloud storage as a
	// data-dependency. This fixture should be used by tests that start lacros from the lacros/launcher package.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacros",
		Desc:     "Lacros Chrome from a pre-built image",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewFixture(External, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{DataArtifact},
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosBypassPermissions is the same as lacros but
	// camera/microphone permissions are enabled by default.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosBypassPermissions",
		Desc:     "Lacros Chrome from a pre-built image with camera/microphone permissions",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewFixture(External, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--use-fake-ui-for-media-stream"),
				chrome.LacrosExtraArgs("--use-fake-ui-for-media-stream")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{DataArtifact},
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosWith100FakeApps is the same as lacros but
	// creates 100 fake apps that are shown in the ash-chrome launcher.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosWith100FakeApps",
		Desc:     "Lacros Chrome from a pre-built image with 100 fake apps installed",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewFixture(External, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return nil, nil
		}),
		Parent:          "install100Apps",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{DataArtifact},
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosPolicyLoggedIn is the same as lacros but with fake DMS to serve
	// policy.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosPolicyLoggedIn",
		Desc:     "Lacros Chrome from a pre-built image with fake DMS to server policy",
		Contacts: []string{"wtlee@chromium.org", "lacros-team@google.com"},
		Impl: &fixtImpl{
			mode: External,
			fOpt: func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
				return nil, nil
			},
			policy: true,
		},
		Parent:          "fakeDMS",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{DataArtifact},
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosForceComposition is the same as lacros but
	// forces composition for ash-chrome.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosForceComposition",
		Desc:     "Lacros Chrome from a pre-built image with composition forced on",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewFixture(External, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--enable-hardware-overlays=\"\"")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{DataArtifact},
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosStartedByDataWithArcEnabled is the same as lacrosStartedByData but with ARC enabled.
	// See also lacrosStartedByDataWithArcBooted in src/chromiumos/tast/local/arc/fixture.go.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosStartedByDataWithArcEnabled",
		Desc:     "Lacros Chrome from a pre-built image with ARC enabled",
		Contacts: []string{"amusbach@chromium.org", "xiyuan@chromium.org"},
		Impl: NewFixture(External, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled()}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{DataArtifact},
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosUI is similar to lacros but should be used
	// by tests that will launch lacros from the ChromeOS UI (e.g shelf) instead
	// of by command line.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosUI",
		Desc:     "Lacros Chrome from a pre-built image using the UI",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewFixture(External, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.EnableFeatures("LacrosSupport")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Data:            []string{DataArtifact},
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
			return []chrome.Option{
				chrome.EnableFeatures("LacrosSupport"),
				chrome.ExtraArgs("--lacros-selection=stateful"),
			}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
	})

	// lacrosRootfs is a fixture to bring up Lacros from the rootfs partition.
	// This does not require downloading a binary from Google Storage before the tests.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosRootfs",
		Desc:     "Lacros Chrome from rootfs",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
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
	// binary. When using the lacros fixture, you must list this as one
	// of the data dependencies of your test.
	DataArtifact = "lacros_binary.tar"

	// LacrosSquashFSPath indicates the location of the rootfs lacros squashfs filesystem.
	LacrosSquashFSPath = "/opt/google/lacros/lacros.squash"

	// lacrosTestPath is the file path at which all lacros-chrome related test artifacts are stored.
	lacrosTestPath = "/usr/local/lacros_test_artifacts"

	// lacrosRootPath is the root directory for lacros-chrome related binaries.
	lacrosRootPath = lacrosTestPath + "/lacros_binary"
)

// The FixtValueImpl object is made available to users of this fixture via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.FixtValueImpl().(lacros.FixtValueImpl)
//		...
//	}
type FixtValueImpl struct {
	Chrome      *chrome.Chrome   // The CrOS-chrome instance.
	TestAPIConn *chrome.TestConn // The CrOS-chrome connection.
	Mode        SetupMode        // Mode used to get the lacros binary.
	LacrosPath  string           // Root directory for lacros-chrome.
	FakeDMS     *fakedms.FakeDMS // Fake DMS to serve policy.
}

// fixtImpl is a fixture that allows Lacros chrome to be launched.
type fixtImpl struct {
	mode       SetupMode              // How (pre exist/to be downloaded/) the container image is obtained.
	lacrosPath string                 // Root directory for lacros-chrome, it's dynamically controlled by the lacros.skipInstallation Var.
	cr         *chrome.Chrome         // Connection to CrOS-chrome.
	tconn      *chrome.TestConn       // Test-connection for CrOS-chrome.
	prepared   bool                   // Set to true if Prepare() succeeds, so that future calls can avoid unnecessary work.
	fOpt       chrome.OptionsCallback // Function to generate Chrome Options
	policy     bool                   // Whether to use fake DMS to serve policy
	fdms       *fakedms.FakeDMS       // The instance of the fake DMS
}

// SetupMode describes how lacros-chrome should be set-up during the test. See the SetupMode constants for more explanation.
type SetupMode int

const (
	// External denotes a lacros-chrome downloaded per the external data dependency.
	// This may be overridden by a pre-deployed binary by specifying the lacrosDeployedBinary Var.
	External SetupMode = iota
	// Omaha is used to get the lacros binary.
	Omaha
	// Rootfs is used to force the rootfs version of lacros-chrome. No external data dependency is needed.
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
	opts = append(opts, chrome.LacrosExtraArgs(extensionArgs(chrome.TestExtensionID, extList)...))

	deployed := false
	if f.mode == External {
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

	if f.policy {
		fdms, ok := s.ParentValue().(*fakedms.FakeDMS)
		if !ok {
			s.Fatal("Parent is not a FakeDMS fixture")
		}
		f.fdms = fdms
		opts = append(opts,
			chrome.DMSPolicy(fdms.URL),
			chrome.FakeLogin(chrome.Creds{User: "tast-user@managedchrome.com", Pass: "test0000"}))
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
	case External:
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

	val := f.buildFixtData(ctx, s)
	chrome.Lock()
	f.prepared = true
	shouldClose = false
	return val
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
func (f *fixtImpl) buildFixtData(ctx context.Context, s *testing.FixtState) FixtValueImpl {
	if err := f.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return FixtValueImpl{f.cr, f.tconn, f.mode, f.lacrosPath, f.fdms}
}
