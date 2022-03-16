// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacrosfixt

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

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
type LacrosConfig struct {
	SetupMode    SetupMode
	LacrosMode   LacrosMode
	deployed     bool
	deployedPath string // dirpath to lacros executable file
}

// NewLacrosConfig creates a new LacrosConfig instance and returns a pointer.
func NewLacrosConfig(setupMode SetupMode, lacrosMode LacrosMode) *LacrosConfig {
	return &LacrosConfig{
		SetupMode:  setupMode,
		LacrosMode: lacrosMode,
	}
}

// WithVar is a method to configure from the runtime var lacrosDeployedBinary and external data artifact.
// It is useful when Lacros should be deployed with the runtime var primarily in Chromium. This will precede over any existing config.
// TestingState allows both testing.FixtState and testing.State to be passed in.
func (cfg *LacrosConfig) WithVar(s TestingState) *LacrosConfig {
	// The main motivation of this var is to allow Chromium CI to build and deploy a fresh
	// lacros-chrome instead of always downloading from a gcs location.
	deployedPath, deployed := s.Var(LacrosDeployedBinary)
	return &LacrosConfig{
		SetupMode:    cfg.SetupMode,
		LacrosMode:   cfg.LacrosMode,
		deployed:     deployed,
		deployedPath: deployedPath,
	}
}

// DefaultOpts returns common chrome options for Lacros given the cfg and setup mode passed in.
func DefaultOpts(cfg *LacrosConfig) ([]chrome.Option, error) {
	var opts []chrome.Option

	// Disable launching lacros on login.
	opts = append(opts, chrome.ExtraArgs("--disable-login-lacros-opening"))

	// Don't show the restore pages popup if lacros crashed in an earlier test.
	// This can interfere with tests.
	opts = append(opts, chrome.LacrosExtraArgs("--hide-crash-restore-bubble"))

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
	switch cfg.SetupMode {
	case Rootfs:
		opts = append(opts, chrome.ExtraArgs("--lacros-selection=rootfs"))
	case Omaha:
		opts = append(opts, chrome.ExtraArgs("--lacros-selection=stateful"))
	}
	if cfg.deployed {
		opts = append(opts, chrome.ExtraArgs("--lacros-chrome-path="+cfg.deployedPath))
	}

	// Set required options based on LacrosMode.
	switch cfg.LacrosMode {
	case NotSpecified, LacrosSideBySide:
		// No-op since it's the system default for now.
	case LacrosPrimary:
		opts = append(opts, chrome.EnableFeatures("LacrosPrimary"), chrome.ExtraArgs("--disable-lacros-keep-alive"))
	case LacrosOnly:
		return nil, errors.New("options for LacrosOnly not implemented")
	}
	return opts, nil
}

// EnsureLacrosReadyForLaunch waits for the lacros binary to be provisioned and ready for launch in test,
// then returns the following variables used to launch:
// - the dir of lacros executable file ('chrome')
func EnsureLacrosReadyForLaunch(ctx context.Context, cfg *LacrosConfig) (string, error) {
	testing.ContextLogf(ctx, "Waiting for Lacros ready with config: %+v", cfg)
	if cfg.deployed {
		// Skip preparing the lacros binary if it is already deployed for use.
		return cfg.deployedPath, nil
	}

	// Throw an error if lacros has been deployed, but the var lacrosDeployedBinary is unset.
	if cfg.SetupMode == Omaha || cfg.SetupMode == Rootfs {
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
	switch cfg.SetupMode {
	case Rootfs:
		// When launched from the rootfs partition, the lacros-chrome is already located
		// at /opt/google/lacros/lacros.squash in the OS, will be mounted at /run/lacros/.
		matches, err := waitForPathToExist(ctx, "/run/lacros/chrome")
		if err != nil {
			return "", errors.Wrap(err, "failed to find lacros binary")
		}
		lacrosPath = filepath.Dir(matches[0])
	case Omaha:
		// When launched by Omaha we need to wait several seconds for lacros to be launchable.
		// It is ready when the image loader path is created with the chrome executable.
		testing.ContextLog(ctx, "Waiting for Lacros to initialize")
		matches, err := waitForPathToExist(ctx, "/run/imageloader/lacros-dogfood*/*/chrome")
		if err != nil {
			return "", errors.Wrap(err, "failed to find lacros binary")
		}
		lacrosPath = filepath.Dir(matches[0])
	default:
		return "", errors.Errorf("Unrecognized mode: %s", cfg.SetupMode)
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
