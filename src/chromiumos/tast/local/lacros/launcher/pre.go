// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

const (
	// mojoSocketPath indicates the path of the unix socket that ash-chrome creates.
	// This unix socket is used for getting the file descriptor needed to connect mojo
	// from ash-chrome to lacros.
	mojoSocketPath = "/tmp/lacros.socket"

	// DataArtifact holds the name of the tarball which contains the lacros-chrome
	// binary. When using the StartedByData precondition, you must list this as one
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
//		d := s.PreValue().(lacros.PreData)
//		...
//	}
type PreData struct {
	Chrome      *chrome.Chrome   // The CrOS-chrome instance.
	TestAPIConn *chrome.TestConn // The CrOS-chrome connection.
	Mode        setupMode        // Mode used to get the lacros binary.
	LacrosPath  string           // Root directory for lacros-chrome.
}

// StartedByData uses a pre-built image downloaded from cloud storage as a
// data-dependency. This precondition should be used by tests that start lacros from the lacros/launcher package.
// To use this precondition you must have lacros.DataArtifact as a data dependency.
func StartedByData() testing.Precondition { return startedByDataPre }

// startedByDataWithChromeOSChromeOptions is the same as StartedByData but allows passing of Options to Chrome.
func startedByDataWithChromeOSChromeOptions(suffix string, opts ...chrome.Option) testing.Precondition {
	return &preImpl{
		name:    "lacros_started_by_artifact_" + suffix,
		timeout: chrome.LoginTimeout + 7*time.Minute,
		mode:    preExist,
		opts:    append(opts, chrome.ExtraArgs("--lacros-mojo-socket-for-testing="+mojoSocketPath)),
	}
}

// StartedByDataWith100FakeApps is the same as StartedByData but creates 100 fake apps that are shown in the
// ChromeOS-chrome launcher.
func StartedByDataWith100FakeApps() testing.Precondition {
	return startedByDataWith100FakeAppsPre
}

// StartedByDataForceComposition is the same as StartedByData but forces composition for ChromeOS-chrome.
func StartedByDataForceComposition() testing.Precondition { return startedByDataForceCompositionPre }

// StartedByDataUI is similar to StartedByData but should be used by tests that will launch lacros from the ChromeOS UI (e.g shelf) instead of by command line.
// To use this precondition you must have lacros.DataArtifact as a data dependency.
func StartedByDataUI() testing.Precondition { return startedByDataUIPre }

// StartedByOmaha is a precondition to enable Lacros by feature flag in Chrome.
// This does not require downloading a binary from Google Storage before the test.
// It will use the currently available fishfood release of Lacros from Omaha.
func StartedByOmaha() testing.Precondition { return startedByOmahaPre }

type setupMode int

const (
	// Lacros-chrome already exists during the precondition. It can be already downloaded per the
	// external data dependency or pre-deployed by the caller site that invokes the tests.
	preExist setupMode = iota
	// Omaha is used to get the lacros binary.
	omaha
)

var startedByDataPre = &preImpl{
	name:    "lacros_started_by_artifact",
	timeout: chrome.LoginTimeout + 7*time.Minute,
	mode:    preExist,
	opts:    []chrome.Option{chrome.ExtraArgs("--lacros-mojo-socket-for-testing=" + mojoSocketPath)},
}

var startedByDataForceCompositionPre = &preImpl{
	name:    "lacros_started_by_artifact_force_composition",
	timeout: chrome.LoginTimeout + 7*time.Minute,
	mode:    preExist,
	opts: []chrome.Option{chrome.ExtraArgs(
		"--lacros-mojo-socket-for-testing="+mojoSocketPath,
		"--enable-hardware-overlays=\"\"")}, // Force composition.
}

var startedByDataWith100FakeAppsPre = ash.NewFakeAppPrecondition("fake_apps", 100, startedByDataWithChromeOSChromeOptions, false)

var startedByDataUIPre = &preImpl{
	name:    "lacros_started_by_artifact_ui",
	timeout: chrome.LoginTimeout + 7*time.Minute,
	mode:    preExist,
	opts:    []chrome.Option{chrome.EnableFeatures("LacrosSupport")},
}

var startedByOmahaPre = &preImpl{
	name:    "lacros_started_by_omaha",
	timeout: chrome.LoginTimeout + 7*time.Minute,
	mode:    omaha,
	opts:    []chrome.Option{chrome.EnableFeatures("LacrosSupport")},
}

// Implementation of lacros's precondition.
type preImpl struct {
	name       string           // Name of this precondition (for logging/uniqueing purposes).
	timeout    time.Duration    // Timeout for completing the precondition.
	mode       setupMode        // How (pre exist/to be downloaded/) the container image is obtained.
	lacrosPath string           // Root directory for lacros-chrome, it's dynamically controlled by the lacros.skipInstallation Var.
	cr         *chrome.Chrome   // Connection to CrOS-chrome.
	tconn      *chrome.TestConn // Test-connection for CrOS-chrome.
	prepared   bool             // Set to true if Prepare() succeeds, so that future calls can avoid unnecessary work.
	opts       []chrome.Option  // Options to be run for CrOS-chrome.
}

// Interface methods for a testing.Precondition.
func (p *preImpl) String() string         { return p.name }
func (p *preImpl) Timeout() time.Duration { return p.timeout }

// prepareLacrosChromeBinary ensures that lacros-chrome binary is available on
// disk and ready to launch. Does not launch the binary.
func (p *preImpl) prepareLacrosChromeBinary(ctx context.Context, s *testing.PreState) error {
	mountCmd := testexec.CommandContext(ctx, "mount", "-o", "remount,exec", "/mnt/stateful_partition")
	if err := mountCmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to remount stateful partition with exec privilege")
	}

	if err := os.RemoveAll(lacrosTestPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove old test artifacts directory")
	}

	if err := os.MkdirAll(lacrosTestPath, os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to make new test artifacts directory")
	}

	artifactPath := s.DataPath(DataArtifact)
	tarCmd := testexec.CommandContext(ctx, "tar", "-xvf", artifactPath, "-C", lacrosTestPath)
	if err := tarCmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to untar test artifacts")
	}
	if err := os.Chmod(binaryPath, 0777); err != nil {
		return errors.Wrap(err, "failed to change permissions of binary dir path")
	}

	return nil
}

// Called by tast before each test is run. We use this method to initialize
// the precondition data, or return early if the precondition is already
// active.
func (p *preImpl) Prepare(ctx context.Context, s *testing.PreState) interface{} {
	ctx, st := timing.Start(ctx, "prepare_"+p.name)
	defer st.End()

	// Currently we assume the precondition wouldn't be broken, and returns
	// existing precondition data immediately without checking.
	// TODO: Check whether the current environment is reusable, and if not
	// reset the state.
	if p.prepared {
		return p.buildPreData(ctx, s)
	}

	// If initialization fails, this defer is used to clean-up the partially-initialized pre.
	// Stolen verbatim from arc/pre.go
	shouldClose := true
	defer func() {
		if shouldClose {
			p.cleanUp(ctx, s)
		}
	}()

	// We reuse the custom extension from the chrome package for exposing private interfaces.
	// TODO(hidehiko): Set up Tast test extension for lacros-chrome.
	c := &chrome.Chrome{}
	if err := c.PrepareExtensions(ctx); err != nil {
		return err
	}
	extList := strings.Join(c.ExtDirs(), ",")
	extensionArgs := extensionArgs(chrome.TestExtensionID, extList)
	p.opts = append(p.opts, chrome.ExtraArgs("--lacros-chrome-additional-args="+strings.Join(extensionArgs, "####")))

	// The main motivation of this var is to allow Chromium CI to build and deploy a fresh
	// lacros-chrome instead of always downloading from a gcs location.
	// All lacros tests are required to register this var regardless needing it or not, please see
	// crbug.com/1163160. This is going to inconvenience to authoring new lacros tests temporarily,
	// but this workaround is to be removed soon once lab provisioning is supported for Lacros.
	path, deployed := s.Var("lacrosDeployedBinary")
	if deployed {
		p.lacrosPath = path
	} else {
		p.lacrosPath = binaryPath
	}

	if p.mode == preExist {
		p.opts = append(p.opts, chrome.ExtraArgs("--lacros-chrome-path="+p.lacrosPath))
	}

	var err error
	if p.cr, err = chrome.New(ctx, p.opts...); err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	if p.tconn, err = p.cr.TestAPIConn(ctx); err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	switch p.mode {
	case preExist:
		if !deployed {
			if err := p.prepareLacrosChromeBinary(ctx, s); err != nil {
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
		s.Fatal("Unrecognized mode: ", p.mode)
	}

	ret := p.buildPreData(ctx, s)
	chrome.Lock()
	p.prepared = true
	shouldClose = false
	return ret
}

// Close is called after all tests involving this precondition have been run,
// (or failed to be run if the precondition itself fails). Unlocks Chrome's and
// the container's constructors.
func (p *preImpl) Close(ctx context.Context, s *testing.PreState) {
	ctx, st := timing.Start(ctx, "close_"+p.name)
	defer st.End()

	chrome.Unlock()
	p.cleanUp(ctx, s)
}

// cleanUp de-initializes the precondition by closing/cleaning-up the relevant
// fields and resetting the struct's fields.
func (p *preImpl) cleanUp(ctx context.Context, s *testing.PreState) {
	// Nothing special needs to be done to close the test API connection.
	p.tconn = nil

	if p.cr != nil {
		if err := p.cr.Close(ctx); err != nil {
			s.Error("Failure closing chrome: ", err)
		}
		p.cr = nil
	}

}

// buildPreData is a helper method that resets the machine state in
// advance of building the precondition data for the actual tests.
func (p *preImpl) buildPreData(ctx context.Context, s *testing.PreState) PreData {
	if err := p.cr.ResetState(ctx); err != nil {
		s.Fatal("Failed to reset chrome's state: ", err)
	}
	return PreData{p.cr, p.tconn, p.mode, p.lacrosPath}
}
