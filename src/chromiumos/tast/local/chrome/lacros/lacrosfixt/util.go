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

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// prepareLacrosBinary ensures that lacros-chrome binary is available on
// disk and ready to launch. Does not launch the binary.
// This will extract lacros-chrome to where the lacrosRootPath constant points to.
func prepareLacrosBinary(ctx context.Context, dataArtifactPath string) error {
	testing.ContextLog(ctx, "Preparing the environment to run Lacros")
	if err := os.RemoveAll(lacrosTestPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove old test artifacts directory")
	}

	if err := os.MkdirAll(lacrosTestPath, os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to make new test artifacts directory")
	}

	if err := os.Chown(lacrosTestPath, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		return errors.Wrap(err, "failed to chown test artifacts directory")
	}

	testing.ContextLog(ctx, "Extracting lacros binary")
	tarCmd := testexec.CommandContext(ctx, "sudo", "-E", "-u", "chronos",
		"tar", "-xvf", dataArtifactPath, "-C", lacrosTestPath)

	if err := tarCmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to untar test artifacts")
	}

	if err := os.Chmod(lacrosRootPath, 0777); err != nil {
		return errors.Wrap(err, "failed to change permissions of the binary root dir path")
	}

	return nil
}

// ExtensionArgs returns a list of args needed to pass to a lacros instance to enable the test extension.
func ExtensionArgs(extID, extList string) []string {
	return []string{
		"--remote-debugging-port=0",              // Let Chrome choose its own debugging port.
		"--enable-experimental-extension-apis",   // Allow Chrome to use the Chrome Automation API.
		"--allowlisted-extension-id=" + extID,    // Whitelists the test extension to access all Chrome APIs.
		"--load-extension=" + extList,            // Load extensions.
		"--disable-extensions-except=" + extList, // Disable extensions other than the Tast test extension.
	}
}

// TestingState is a mixin interface that allows both testing.FixtState and testing.State to be passed into WithVar.
type TestingState interface {
	Var(string) (string, bool)
	DataPath(string) string
}

// LacrosConfig holds runtime vars or other variables needed to set up Lacros.
// This will be passed in to DefaultOpts that returns common chrome options, and to WaitForReady that prepares Lacros for use by either fixture or test.
// TODO(hyungtaekim): Add 'Deployed' to SetupMode, remove deployed from struct.
type LacrosConfig struct {
	Mode         SetupMode
	Availability LacrosAvailability
	deployed     bool
	deployedPath string // dirpath to lacros executable file
}

// WithVar is a method to configure from the runtime var lacrosDeployedBinary and external data artifact.
// It is useful when Lacros should be deployed with the runtime var primariliy in Chromium. This will precede over any existing config.
// TestingState allows both testing.FixtState and testing.State to be passed in.
func (cfg LacrosConfig) WithVar(s TestingState) LacrosConfig {
	// The main motivation of this var is to allow Chromium CI to build and deploy a fresh
	// lacros-chrome instead of always downloading from a gcs location.
	cfg.deployedPath, cfg.deployed = s.Var(LacrosDeployedBinary)
	// If External, deployedPath is set to the path to the compressed Lacros image that needs to be installed in prepareLacrosBinary later.
	if cfg.Mode == External {
		cfg.deployedPath = s.DataPath(dataArtifact)
	}
	return cfg
}

// DefaultOpts returns common chrome options for Lacros given the cfg and setup mode passed in.
func DefaultOpts(cfg LacrosConfig) ([]chrome.Option, error) {
	var opts []chrome.Option

	// mojoSocketPath indicates the path of the unix socket that ash-chrome creates.
	// This unix socket is used for getting the file descriptor needed to connect mojo
	// from ash-chrome to lacros.
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
		return nil, errors.Wrap(err, "failed to prepare extensions")
	}
	extList := strings.Join(extDirs, ",")
	opts = append(opts, chrome.LacrosExtraArgs(ExtensionArgs(chrome.TestExtensionID, extList)...))

	// Enable Lacros.
	// Note that specifying the feature LacrosSupport has side-effects, so
	// we specify it even if the lacros path is being overridden by lacrosDeployedBinary.
	opts = append(opts, chrome.EnableFeatures("LacrosSupport", "ForceProfileMigrationCompletion"))
	switch cfg.Mode {
	case Rootfs:
		opts = append(opts, chrome.ExtraArgs("--lacros-selection=rootfs"))
	case Omaha:
		opts = append(opts, chrome.ExtraArgs("--lacros-selection=stateful"))
	case External:
		// If External or deployed we should specify the path.
		// This will override the lacros-selection argument.
		opts = append(opts, chrome.ExtraArgs("--lacros-chrome-path="+cfg.deployedPath))
	}
	if cfg.deployed {
		opts = append(opts, chrome.ExtraArgs("--lacros-chrome-path="+cfg.deployedPath))
	}

	// Set Lacros Availability and required options.
	switch cfg.Availability {
	case LacrosPrimary:
		opts = append(opts,
			chrome.EnableFeatures("LacrosPrimary"),
			chrome.ExtraArgs("--disable-lacros-keep-alive", "--disable-login-lacros-opening"))
	case LacrosOnly:
		// TODO: Implement LacrosOnly or other availabilities when needed.
	}
	return opts, nil
}

// EnsureLacrosReadyForLaunch waits for the lacros binary to be provisioned and ready for launch in test,
// then returns the following variables used to launch:
// - the dir of lacros executable file ('chrome')
func EnsureLacrosReadyForLaunch(ctx context.Context, cfg LacrosConfig) (string, error) {
	testing.ContextLogf(ctx, "Waiting for Lacros ready with config: %+v", cfg)
	if cfg.deployed {
		// Skip preparing the lacros binary if it is already deployed for use.
		return cfg.deployedPath, nil
	}

	// Throw an error if lacros has been deployed, but the var lacrosDeployedBinary is unset.
	if cfg.Mode == Omaha || cfg.Mode == Rootfs {
		config, err := ioutil.ReadFile("/etc/chrome_dev.conf")
		if err == nil {
			for _, line := range strings.Split(string(config), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "--lacros-chrome-path") {
					return "", errors.New("found --lacros-chrome-path in /etc/chrome_dev.conf, but lacrosDeployedBinary is not specified")
				}
			}
		}
	}

	// Prepare the lacros binary if it isn't deployed already via lacrosDeployedBinary.
	var lacrosPath string
	switch cfg.Mode {
	case External:
		if err := prepareLacrosBinary(ctx, cfg.deployedPath); err != nil {
			return "", errors.Wrap(err, "failed to prepare lacros-chrome")
		}
		lacrosPath = lacrosRootPath
	case Omaha:
		// When launched by Omaha we need to wait several seconds for lacros to be launchable.
		// It is ready when the image loader path is created with the chrome executable.
		testing.ContextLog(ctx, "Waiting for Lacros to initialize")
		matches, err := waitForPathToExist(ctx, "/run/imageloader/lacros-dogfood*/*/chrome")
		if err != nil {
			return "", errors.Wrap(err, "failed to find lacros binary")
		}
		lacrosPath = filepath.Dir(matches[0])
	case Rootfs:
		// When launched from the rootfs partition, the lacros-chrome is already located
		// at /opt/google/lacros/lacros.squash in the OS, will be mounted at /run/lacros/.
		matches, err := waitForPathToExist(ctx, "/run/lacros/chrome")
		if err != nil {
			return "", errors.Wrap(err, "failed to find lacros binary")
		}
		lacrosPath = filepath.Dir(matches[0])
	default:
		return "", errors.New("Unrecognized mode: " + string(cfg.Mode))
	}

	return lacrosPath, nil
}

// waitForPathToExist is a helper method that waits the given binary path to be present
// then returns the matching paths or it will be timed out if the ctx's timeout is reached.
func waitForPathToExist(ctx context.Context, pattern string) (matches []string, err error) {
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
