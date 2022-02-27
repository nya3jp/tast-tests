// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacrosfixt

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// LacrosDeployedBinary contains the Fixture Var necessary to run lacros.
// This should be used by any lacros fixtures defined outside this file.
const LacrosDeployedBinary = "lacrosDeployedBinary"

func init() {
	// lacros uses rootfs lacros, which is the recommend way to use lacros
	// in Tast tests, unless you have a specific use case for using lacros from
	// another source.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacros",
		Desc:     "Lacros Chrome from a pre-built image",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--disable-lacros-keep-alive")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosAudio is the same as lacros but has some special flags for audio
	// tests.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosAudio",
		Desc:     "Lacros Chrome from a pre-built image with camera/microphone permissions",
		Contacts: []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--use-fake-ui-for-media-stream"),
				chrome.ExtraArgs("--autoplay-policy=no-user-gesture-required"), // Allow media autoplay.
				chrome.ExtraArgs("--disable-lacros-keep-alive"),
				chrome.LacrosExtraArgs("--use-fake-ui-for-media-stream"),
				chrome.LacrosExtraArgs("--autoplay-policy=no-user-gesture-required")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosWaylandDecreasedPriority is the same as lacros but using Wayland
	// normal thread priority flag.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosWaylandDecreasedPriority",
		Desc:     "Lacros Chrome from a pre-built image using normal thread priority for Wayland",
		Contacts: []string{"hidehiko@chromium.org", "tvignatti@igalia.com"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--disable-lacros-keep-alive"),
				chrome.LacrosExtraArgs(lacrosWaylandDecreasedPriorityArgs...)}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosAshLikeThreadsPriority is the same as lacros but using display
	// thread priority for browser UI and IO threads; GPU main, viz compositor
	// and IO threads; and renderer compositor and IO threads. These are finch
	// flags enabled by default on ash-chrome.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosAshLikeThreadsPriority",
		Desc:     "Lacros Chrome from a pre-built image using display thread priority similar to ash-chrome",
		Contacts: []string{"hidehiko@chromium.org", "tvignatti@igalia.com"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--disable-lacros-keep-alive"),
				chrome.LacrosExtraArgs(lacrosAshLikeThreadsPriorityArgs...)}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosWaylandDecreasedAndAshLikeThreadsPriority is the same as lacros
	// but using Wayland normal thread priority and ash-chrome like threads
	// flags (see lacrosAshLikeThreadsPriority above).
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosWaylandDecreasedAndAshLikeThreadsPriority",
		Desc:     "Lacros Chrome from a pre-built image normal thread priority for Wayland and display thread priority similar to ash-chrome",
		Contacts: []string{"hidehiko@chromium.org", "tvignatti@igalia.com"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--disable-lacros-keep-alive"),
				chrome.LacrosExtraArgs(lacrosWaylandDecreasedPriorityArgs...),
				chrome.LacrosExtraArgs(lacrosAshLikeThreadsPriorityArgs...)}, nil
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
			return []chrome.Option{chrome.ExtraArgs("--disable-lacros-keep-alive")}, nil
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
			return []chrome.Option{chrome.ExtraArgs("--enable-hardware-overlays=\"\""),
				chrome.ExtraArgs("--disable-lacros-keep-alive")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosForceDelegation is the same as lacros but
	// forces delegated composition.
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
			return []chrome.Option{chrome.ARCEnabled(),
				chrome.ExtraArgs("--disable-lacros-keep-alive")}, nil
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
			return []chrome.Option{chrome.ExtraArgs("--disable-lacros-keep-alive")}, nil
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
			return []chrome.Option{chrome.ExtraArgs("--disable-lacros-keep-alive")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosPrimary is a fixture to bring up Lacros as a primary browser from the rootfs partition by default.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosPrimary",
		Desc:     "Lacros Chrome from rootfs as a primary browser",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.EnableFeatures("LacrosPrimary"),
				chrome.ExtraArgs("--disable-lacros-keep-alive",
					"--disable-login-lacros-opening")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosPrimaryRestorable is a fixture to bring up Lacros as a primary browser from the rootfs partition by default
	// and support the fullrestore after a reboot or restarting a ui session.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosPrimaryRestorable",
		Desc:     "Lacros Chrome from rootfs as a primary browser",
		Contacts: []string{"hyungtaekim@chromium.org", "lacros-team@google.com", "abhijeet@igalia.com"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.EnableFeatures("LacrosPrimary", "FullRestore"),
				chrome.ExtraArgs("--disable-lacros-keep-alive")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 1*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosUIKeepAlive is similar to lacros but should be used
	// by tests that will launch lacros from the ChromeOS UI (e.g shelf) instead
	// of by command line, and this test assuming that Lacros will be keep alive
	// in the background even if the browser is turned off.
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosUIKeepAlive",
		Desc:     "Lacros Chrome from a pre-built image using the UI and the Lacros chrome will stay alive even when the browser terminated",
		Contacts: []string{"mxcai@chromium.org", "hidehiko@chromium.org"},
		Impl: NewFixture(Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.EnableFeatures("LacrosPrimary")}, nil
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})
}

// TODO(tvignatti): how do we make sure Lacros has this flag implemented? See crrev.com/c/3304121.
var lacrosWaylandDecreasedPriorityArgs = []string{
	// Use Wayland event watcher thread with normal priority.
	"--use-wayland-normal-thread-priority",
}
var lacrosAshLikeThreadsPriorityArgs = []string{
	// Enable display priority for browser UI and IO threads.
	"--enable-features=BrowserUseDisplayThreadPriority",
	// Enable display priority for GPU main, viz compositor and IO threads.
	"--enable-features=GpuUseDisplayThreadPriority",
	// Enable display priority for Renderer compositor and IO threads.
	"--enable-features=BlinkCompositorUseDisplayThreadPriority",
}

const (
	// MojoSocketPath indicates the path of the unix socket that ash-chrome creates.
	// This unix socket is used for getting the file descriptor needed to connect mojo
	// from ash-chrome to lacros.
	MojoSocketPath = "/tmp/lacros.socket"

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

// Verify that *fixtValueImpl implements FixtValue interface.
var _ FixtValue = (*fixtValueImpl)(nil)

// fixtValueImpl holds values related to the lacros instance and connection.
// Tests should not use this directly, unless they are composing fixtures
// and need to embed this struct in their own FixtValue. Instead, access
// needed values through the lacros FixtValue interface.
type fixtValueImpl struct {
	chrome      *chrome.Chrome
	testAPIConn *chrome.TestConn
	mode        SetupMode
	lacrosPath  string
	// Each Reset() of the fixture creates a new user tmp directory. Use a
	// pointer here such that FixtValue().UserTmpDir() can return the
	// current one.  TODO(hidehiko): Clean this up.
	userTmpDir *string
	options    []config.Option
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

// UserTmpDir returns the path to be used for Lacros's user data directory.
// This directory will be wiped on every reset call.
// We used to use generic tmp directory, and kept it until whole Tast run
// completes, but Lacros user data consumes more disk than other cases,
// and we hit out-of-diskspace on some devices which has very limited disk
// space. To avoid that problem, the user data will be wiped for each
// test run.
func (f *fixtValueImpl) UserTmpDir() string {
	return *f.userTmpDir
}

// Options returns chrome.Option used to launch a CrOS-chrome.
func (f *fixtValueImpl) Options() []chrome.Option {
	return f.options
}

// fixtImpl is a fixture that allows Lacros chrome to be launched.
type fixtImpl struct {
	mode         SetupMode                                     // How (pre exist/to be downloaded/) the container image is obtained.
	lacrosPath   string                                        // Root directory for lacros-chrome, it's dynamically controlled by the lacros.skipInstallation Var.
	cr           *chrome.Chrome                                // Connection to CrOS-chrome.
	tconn        *chrome.TestConn                              // Test-connection for CrOS-chrome.
	prepared     bool                                          // Set to true if Prepare() succeeds, so that future calls can avoid unnecessary work.
	fOpt         chrome.OptionsCallback                        // Function to generate Chrome Options
	makeValue    func(v FixtValue, pv interface{}) interface{} // Closure to create FixtValue to return from SetUp. Used for composable fixtures.
	userTmpDir   string                                        // Path to the tmp directory storing lacros user data.
	optsToReturn []chrome.Option                               // Options used to launch a CrOS-chrome.
}

// SetUp is called by tast before each test is run. We use this method to initialize
// the fixture data, or return early if the fixture is already active.
func (f *fixtImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	ctx, st := timing.Start(ctx, "SetUp")
	defer st.End()

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

	userTmpDir, err := createUserTempDir()
	if err != nil {
		s.Fatal("Failed to create new user tmp directory: ", err)
	}
	f.userTmpDir = userTmpDir

	opts, err := f.fOpt(ctx, s)
	if err != nil {
		s.Fatal("Failed to obtain fixture options: ", err)
	}

	opts = append(opts, chrome.ExtraArgs("--lacros-mojo-socket-for-testing="+MojoSocketPath))

	// Suppress experimental Lacros infobar and possible others as well.
	opts = append(opts, chrome.LacrosExtraArgs("--test-type"))

	// The What's-New feature automatically redirects the browser to a WebUI page to display the
	// new feature if this is first time the user opens the browser or the user has upgraded
	// Chrome to a different milestone. Disables the feature in testing to make the test
	// expectations more predirectable, and thus make the tests more stable.
	opts = append(opts, chrome.LacrosExtraArgs("--disable-features=ChromeWhatsNewUI"))

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

	// Cache the chrome.Option and
	f.optsToReturn = opts
	if f.cr, err = chrome.New(ctx, opts...); err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	if f.tconn, err = f.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Prepare the lacros binary if it isn't deployed already via lacrosDeployedBinary.
	if !deployed {
		if f.mode == Omaha || f.mode == Rootfs {
			config, err := ioutil.ReadFile("/etc/chrome_dev.conf")
			if err == nil {
				for _, line := range strings.Split(string(config), "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "--lacros-chrome-path") {
						s.Fatal("Found --lacros-chrome-path in /etc/chrome_dev.conf, but lacrosDeployedBinary is not specified")
					}
				}
			}
		}
		switch f.mode {
		case External:
			if err := prepareLacrosBinary(ctx, s); err != nil {
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
	if err := os.RemoveAll(f.userTmpDir); err != nil {
		return errors.Wrap(err, "failed resetting user tmp directory")
	}
	// Reset the member temporarily, so even if temp dir creation just below fails,
	// the state will be kept gracefully.
	f.userTmpDir = ""
	userTmpDir, err := createUserTempDir()
	if err != nil {
		return errors.Wrap(err, "failed to create new user tmp directory")
	}
	f.userTmpDir = userTmpDir
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

	if f.userTmpDir != "" {
		if err := os.RemoveAll(f.userTmpDir); err != nil {
			s.Error("Failed to remove user tmp directory: ", err)
		}
		f.userTmpDir = ""
	}

	f.prepared = false
}

// buildFixtData is a helper method that resets the machine state in
// advance of building the fixture data for the actual tests.
func (f *fixtImpl) buildFixtData(ctx context.Context, s *testing.FixtState) *fixtValueImpl {
	if err := f.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return &fixtValueImpl{f.cr, f.tconn, f.mode, f.lacrosPath, &f.userTmpDir, f.optsToReturn}
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

// createUserTempDir creates a temporary directory to store Lacros's user data.
// On success, it is caller's responsibility to delete the directory eventually.
func createUserTempDir() (string, error) {
	dir, err := ioutil.TempDir("", "lacros_user_data")
	if err != nil {
		return "", errors.Wrap(err, "failed to create lacros user data directory")
	}
	if err := os.Chmod(dir, 0777); err != nil {
		os.RemoveAll(dir)
		return "", errors.Wrap(err, "failed to set permissions for lacros user data directory")
	}
	return dir, nil
}
