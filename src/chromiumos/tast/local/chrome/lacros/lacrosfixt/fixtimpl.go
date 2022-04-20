// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacrosfixt

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
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
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfigFromState(s).Opts()
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
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfigFromState(s, ChromeOptions(
				chrome.ExtraArgs("--use-fake-ui-for-media-stream"),
				chrome.ExtraArgs("--autoplay-policy=no-user-gesture-required"), // Allow media autoplay.
				chrome.LacrosExtraArgs("--use-fake-ui-for-media-stream"),
				chrome.LacrosExtraArgs("--autoplay-policy=no-user-gesture-required"))).Opts()
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
		Impl: NewFixture(lacros.Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--disable-lacros-keep-alive"),
				chrome.LacrosExtraArgs("--no-first-run")}, nil
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
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfigFromState(s, ChromeOptions(chrome.ExtraArgs("--enable-hardware-overlays=\"\""))).Opts()
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
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfigFromState(s, ChromeOptions(
				chrome.LacrosExtraArgs("--enable-gpu-memory-buffer-compositor-resources"),
				chrome.LacrosEnableFeatures("DelegatedCompositing"))).Opts()
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
		Impl: NewFixture(lacros.Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ARCEnabled(),
				chrome.ExtraArgs("--disable-lacros-keep-alive"),
				chrome.LacrosExtraArgs("--no-first-run")}, nil
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
		Impl: NewFixture(lacros.Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.ExtraArgs("--disable-lacros-keep-alive"),
				chrome.LacrosExtraArgs("--no-first-run")}, nil
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
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfigFromState(s, Selection(lacros.Omaha)).Opts()
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
		Impl: NewFixture(lacros.Rootfs, func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return []chrome.Option{chrome.EnableFeatures("LacrosPrimary"),
				chrome.ExtraArgs("--disable-lacros-keep-alive"),
				chrome.LacrosExtraArgs("--no-first-run")}, nil
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
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfigFromState(s, KeepAlive(true), Mode(lacros.LacrosPrimary)).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})

	// lacrosVariation is similar to lacros but should be used
	// by variation smoke tests that will launch lacros with variation service enabled,
	testing.AddFixture(&testing.Fixture{
		Name:     "lacrosVariationEnabled",
		Desc:     "Lacros with variation service enabled",
		Contacts: []string{"yjt@google.com", "lacros-team@google.com"},
		Impl: chrome.NewLoggedInFixture(func(ctx context.Context, s *testing.FixtState) ([]chrome.Option, error) {
			return NewConfigFromState(s, Mode(lacros.LacrosPrimary), ChromeOptions(
				chrome.LacrosExtraArgs("--fake-variations-channel=beta"),
				chrome.LacrosExtraArgs("--variations-server-url=https://clients4.google.com/chrome-variations/seed"))).Opts()
		}),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:            []string{LacrosDeployedBinary},
	})
}

const (
	// LacrosSquashFSPath indicates the location of the rootfs lacros squashfs filesystem.
	LacrosSquashFSPath = "/opt/google/lacros/lacros.squash"
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
	selection   lacros.Selection
}

// Chrome gets the CrOS-chrome instance.
func (f *fixtValueImpl) Chrome() *chrome.Chrome {
	return f.chrome
}

// TestAPIConn gets the CrOS-chrome test connection.
func (f *fixtValueImpl) TestAPIConn() *chrome.TestConn {
	return f.testAPIConn
}

// fixtImpl is a fixture that allows Lacros chrome to be launched.
type fixtImpl struct {
	selection lacros.Selection                              // How (pre exist/to be downloaded/) the container image is obtained.
	cr        *chrome.Chrome                                // Connection to CrOS-chrome.
	tconn     *chrome.TestConn                              // Test-connection for CrOS-chrome.
	prepared  bool                                          // Set to true if Prepare() succeeds, so that future calls can avoid unnecessary work.
	fOpt      chrome.OptionsCallback                        // Function to generate Chrome Options
	makeValue func(v FixtValue, pv interface{}) interface{} // Closure to create FixtValue to return from SetUp. Used for composable fixtures.
}

// SetUp is called by tast before each test is run. We use this method to initialize
// the fixture data, or return early if the fixture is already active.
func (f *fixtImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	ctx, st := timing.Start(ctx, "SetUp")
	defer st.End()

	if f.prepared {
		s.Log("Fixture has already been prepared. Returning a cached one. mode: ", f.selection)
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
	// If there's a parent fixture and the fixture supplies extra options, use them.
	if extraOpts, ok := s.ParentValue().([]chrome.Option); ok {
		opts = append(opts, extraOpts...)
	}
	// Set opts for Lacros based on the selection and the runtime var.
	cfg := NewConfigFromState(s, Selection(f.selection))
	lacrosOpts, err := cfg.Opts()
	if err != nil {
		s.Fatal("Failed to set default options: ", err)
	}
	opts = append(opts, lacrosOpts...)

	if f.cr, err = chrome.New(ctx, opts...); err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	if f.tconn, err = f.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	if cfg.deployed {
		testing.ContextLog(ctx, "Using lacros located at ", cfg.deployedPath)
	}

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

func (f *fixtImpl) PostTest(ctx context.Context, s *testing.FixtTestState) {
	if out, ok := testing.ContextOutDir(ctx); !ok {
		testing.ContextLog(ctx, "OutDir not found")
	} else {
		if err := fsutil.CopyFile(LacrosLogPath, filepath.Join(out, "lacros.log")); err != nil {
			testing.ContextLog(ctx, "Failed to save lacros logs: ", err)
		}
	}
}

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

	f.prepared = false
}

// buildFixtData is a helper method that resets the machine state in
// advance of building the fixture data for the actual tests.
func (f *fixtImpl) buildFixtData(ctx context.Context, s *testing.FixtState) *fixtValueImpl {
	if err := f.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return &fixtValueImpl{f.cr, f.tconn, f.selection}
}
