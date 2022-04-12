// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacrosfixt

import (
	"io/ioutil"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/lacros"
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
	Selection    lacros.Selection
	Mode         lacros.Mode
	deployed     bool
	deployedPath string // dirpath to lacros executable file
}

// NewLacrosConfig creates a new LacrosConfig instance and returns a pointer.
func NewLacrosConfig(selection lacros.Selection, mode lacros.Mode) *LacrosConfig {
	return &LacrosConfig{
		Selection: selection,
		Mode:      mode,
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
		Selection:    cfg.Selection,
		Mode:         cfg.Mode,
		deployed:     deployed,
		deployedPath: deployedPath,
	}
}

// DefaultOpts returns common chrome options for Lacros given the cfg.
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
	opts = append(opts, chrome.LacrosDisableFeatures("ChromeWhatsNewUI"))

	// Force color profile to sRGB regardless of device. See b/221643955 for details.
	opts = append(opts, chrome.LacrosExtraArgs("--force-color-profile=srgb"))
	opts = append(opts, chrome.LacrosExtraArgs("--force-raster-color-profile=srgb"))

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
	switch cfg.Selection {
	case lacros.Rootfs:
		opts = append(opts, chrome.ExtraArgs("--lacros-selection=rootfs"))
	case lacros.Omaha:
		opts = append(opts, chrome.ExtraArgs("--lacros-selection=stateful"))
	}
	if cfg.deployed {
		opts = append(opts, chrome.ExtraArgs("--lacros-chrome-path="+cfg.deployedPath))
	}

	// Set required options based on Mode.
	switch cfg.Mode {
	case lacros.NotSpecified, lacros.LacrosSideBySide:
		// No-op since it's the system default for now.
	case lacros.LacrosPrimary:
		opts = append(opts, chrome.EnableFeatures("LacrosPrimary"), chrome.ExtraArgs("--disable-lacros-keep-alive"))
	case lacros.LacrosOnly:
		return nil, errors.New("options for LacrosOnly not implemented")
	}

	// Throw an error if lacros has been deployed, but the var lacrosDeployedBinary is unset.
	if !cfg.deployed && (cfg.Selection == lacros.Omaha || cfg.Selection == lacros.Rootfs) {
		config, err := ioutil.ReadFile("/etc/chrome_dev.conf")
		if err == nil {
			for _, line := range strings.Split(string(config), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "--lacros-chrome-path") {
					return nil, errors.New(
						"found --lacros-chrome-path in /etc/chrome_dev.conf, but lacrosDeployedBinary is not specified, " +
							"you may need to pass `-var lacrosDeployedBinary=/usr/local/lacros-chrome` to `tast run` " +
							"if you've deployed your own Lacros binary to the DUT, " +
							"or you may need to comment out/remove --lacros-chrome-path in /etc/chrome_dev.conf")
				}
			}
		}
	}

	return opts, nil
}
