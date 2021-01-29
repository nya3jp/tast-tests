// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
	"context"
	"path/filepath"
	"strings"
	"time"
)


func newStartedByData(mode setupMode, opts ...chrome.Option) *fixtureImpl {
	return &fixtureImpl{
		mode:    mode,
		opts:    append(opts, chrome.ExtraArgs("--lacros-mojo-socket-for-testing="+mojoSocketPath)),
	}
}

func init() {
	// lacrosStartedByData uses a pre-built image downloaded from cloud storage as a
	// data-dependency. This precondition should be used by tests that start lacros from the lacros/launcher package.
	// To use this precondition you must have lacros.DataArtifact as a data dependency.
	testing.AddFixture(&testing.Fixture{
		Name:            "lacrosStartedByData",
		Desc:            "Logged into a user session",
		Contacts:        []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl:            newStartedByData(preExist),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:         []string{"lacrosDeployedBinary"},
	})

	// lacrosStartedByDataWith100FakeApps is the same as lacrosStartedByData but creates 100 fake apps that are shown in the
  // ChromeOS-chrome launcher.
	testing.AddFixture(&testing.Fixture{
		Name:            "lacrosStartedByDataWith100FakeApps",
		Desc:            "Logged into a user session",
		Contacts:        []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl:            newStartedByData(preExist),
		Parent: "install100Apps",
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:         []string{"lacrosDeployedBinary"},
	})

// lacrosStartedByDataForceComposition is the same as lacrosStartedByData but forces composition for ChromeOS-chrome.
	testing.AddFixture(&testing.Fixture{
		Name:            "lacrosStartedByDataForceComposition",
		Desc:            "Logged into a user session",
		Contacts:        []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl:            newStartedByData(preExist, chrome.ExtraArgs("--enable-hardware-overlays=\"\"")),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:         []string{"lacrosDeployedBinary"},
	})


	// lacrosStartedByDataUI is similar to lacrosStartedByData but should be used
	// by tests that will launch lacros from the ChromeOS UI (e.g shelf) instead
	// of by command line. To use this precondition you must have
	// lacros.DataArtifact as a data dependency.
	testing.AddFixture(&testing.Fixture{
		Name:            "lacrosStartedByDataUI",
		Desc:            "Logged into a user session",
		Contacts:        []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl:            newStartedByData(preExist, chrome.EnableFeatures("LacrosSupport")),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:         []string{"lacrosDeployedBinary"},
	})

	// lacrosStartedByOmaha is a precondition to enable Lacros by feature flag in Chrome.
	// This does not require downloading a binary from Google Storage before the test.
	// It will use the currently available fishfood release of Lacros from Omaha.
	testing.AddFixture(&testing.Fixture{
		Name:            "lacrosStartedByOmaha",
		Desc:            "Logged into a user session",
		Contacts:        []string{"hidehiko@chromium.org", "edcourtney@chromium.org"},
		Impl:            newStartedByData(omaha, chrome.EnableFeatures("LacrosSupport")),
		SetUpTimeout:    chrome.LoginTimeout + 7*time.Minute,
		ResetTimeout:    chrome.ResetTimeout,
		TearDownTimeout: chrome.ResetTimeout,
		Vars:         []string{"lacrosDeployedBinary"},
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

	// LacrosTestPath is the file path at which all lacros-chrome related test artifacts are stored.
	lacrosTestPath = "/mnt/stateful_partition/lacros_test_artifacts"

	// binaryPath is the root directory for lacros-chrome related binaries.
	binaryPath = lacrosTestPath + "/lacros_binary"
)

// The PreData object is made available to users of this precondition via:
//
//	func DoSomething(ctx context.Context, s *testing.State) {
//		d := s.FixtValue().(lacros.PreData)
//		...
//	}
type PreData struct {
	Chrome      *chrome.Chrome   // The CrOS-chrome instance.
	TestAPIConn *chrome.TestConn // The CrOS-chrome connection.
	Mode        setupMode        // Mode used to get the lacros binary.
	LacrosPath  string           // Root directory for lacros-chrome.
}

// Implementation of lacros's precondition.
type fixtureImpl struct {
	mode       setupMode        // How (pre exist/to be downloaded/) the container image is obtained.
	lacrosPath string           // Root directory for lacros-chrome, it's dynamically controlled by the lacros.skipInstallation Var.
	cr         *chrome.Chrome   // Connection to CrOS-chrome.
	tconn      *chrome.TestConn // Test-connection for CrOS-chrome.
	prepared   bool             // Set to true if Prepare() succeeds, so that future calls can avoid unnecessary work.
	opts       []chrome.Option  // Options to be run for CrOS-chrome.
}

type setupMode int

const (
	// Lacros-chrome already exists during the precondition. It can be already downloaded per the
	// external data dependency or pre-deployed by the caller site that invokes the tests.
	preExist setupMode = iota
	// Omaha is used to get the lacros binary.
	omaha
)

// Called by tast before each test is run. We use this method to initialize
// the precondition data, or return early if the precondition is already
// active.
func (f *fixtureImpl) SetUp(ctx context.Context, s *testing.FixtState) interface{} {
	ctx, st := timing.Start(ctx, "SetUp")
	defer st.End()

	// Currently we assume the precondition wouldn't be broken, and returns
	// existing precondition data immediately without checking.
	// TODO: Check whether the current environment is reusable, and if not
	// reset the state.
	if f.prepared {
		return f.buildPreData(ctx, s)
	}

	// If initialization fails, this defer is used to clean-up the partially-initialized pre.
	// Stolen verbatim from arc/pre.go
	shouldClose := true
	defer func() {
		if shouldClose {
			f.cleanUp(ctx, s)
		}
	}()

	// We reuse the custom extension from the chrome package for exposing private interfaces.
	// TODO(hidehiko): Set up Tast test extension for lacros-chrome.
	extDirs, err := chrome.DeprecatedPrepareExtensions()
	if err != nil {
		return err
	}
	extList := strings.Join(extDirs, ",")
	extensionArgs := extensionArgs(chrome.TestExtensionID, extList)
	f.opts = append(f.opts, chrome.ExtraArgs("--lacros-chrome-additional-args="+strings.Join(extensionArgs, "####")))

	// The main motivation of this var is to allow Chromium CI to build and deploy a fresh
	// lacros-chrome instead of always downloading from a gcs location.
	// This workaround is to be removed soon once lab provisioning is supported for Lacros.
	path, deployed := s.Var("lacrosDeployedBinary")
	if deployed {
		f.lacrosPath = path
	} else {
		f.lacrosPath = binaryPath
	}

	if f.mode == preExist {
		f.opts = append(f.opts, chrome.ExtraArgs("--lacros-chrome-path="+f.lacrosPath))
	}

	if f.cr, err = chrome.New(ctx, f.opts...); err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	if f.tconn, err = f.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	switch f.mode {
	case preExist:
		if !deployed {
			if err := prepareLacrosChromeBinary(ctx, s); err != nil {
				s.Fatal("Failed to prepare lacros-chrome, err")
			}
		}
	case omaha:
		// When launched by Omaha we need to wait several seconds for lacros to be launchable.
		// It is ready when the image loader path is created with the chrome executable.
		testing.ContextLog(ctx, "Waiting for Lacros to initialize")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			matches, err := filepath.Glob("/run/imageloader/lacros-fishfood/*/chrome")
			if err != nil {
				return errors.Wrap(err, "binary path does not exist yet")
			}
			if len(matches) == 0 {
				return errors.New("binary path does not exist yet")
			}
			return nil
		}, &testing.PollOptions{Interval: 5 * time.Second}); err != nil {
			s.Fatal("Failed to find lacros binary: ", err)
		}
	default:
		s.Fatal("Unrecognized mode: ", f.mode)
	}

	ret := f.buildPreData(ctx, s)
	chrome.Lock()
	f.prepared = true
	shouldClose = false
	return ret
}

// Close is called after all tests involving this precondition have been run,
// (or failed to be run if the precondition itself fails). Unlocks Chrome's and
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

// cleanUp de-initializes the precondition by closing/cleaning-up the relevant
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

// buildPreData is a helper method that resets the machine state in
// advance of building the precondition data for the actual tests.
func (f *fixtureImpl) buildPreData(ctx context.Context, s *testing.FixtState) PreData {
	if err := f.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return PreData{f.cr, f.tconn, f.mode, f.lacrosPath}
}
