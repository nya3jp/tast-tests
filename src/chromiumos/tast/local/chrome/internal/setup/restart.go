// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package setup

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/internal/extension"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// RestartChromeForTesting restarts the ui job, asks session_manager to enable Chrome testing,
// and waits for Chrome to listen on its debugging port.
func RestartChromeForTesting(ctx context.Context, cfg *config.Config, exts *extension.Files) error {
	ctx, st := timing.Start(ctx, "restart")
	defer st.End()

	if err := restartSession(ctx, cfg); err != nil {
		return err
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return err
	}

	// Remove the file where Chrome will write its debugging port after it's restarted.
	if err := driver.PrepareForRestart(); err != nil {
		return err
	}

	testing.ContextLog(ctx, "Asking session_manager to enable Chrome testing")
	args := []string{
		"--remote-debugging-port=0",            // Let Chrome choose its own debugging port.
		"--disable-logging-redirect",           // Disable redirection of Chrome logging into cryptohome.
		"--ash-disable-system-sounds",          // Disable system startup sound.
		"--enable-experimental-extension-apis", // Allow Chrome to use the Chrome Automation API.
		"--redirect-libassistant-logging",      // Redirect libassistant logging to /var/log/chrome/.
		"--no-first-run",                       // Prevent showing up offer pages, e.g. google.com/chromebooks.
		"--cros-region=" + cfg.Region(),        // Force the region.
		"--cros-regions-mode=hide",             // Ignore default values in VPD.
		"--enable-oobe-test-api",               // Enable OOBE helper functions for authentication.
		"--disable-hid-detection-on-oobe",      // Skip OOBE check for keyboard/mouse on chromeboxes/chromebases.
	}
	if !cfg.EnableRestoreTabs() {
		args = append(args, "--no-startup-window") // Do not start up chrome://newtab by default to avoid unexpected patterns (doodle etc.)
	}

	if cfg.SkipOOBEAfterLogin() {
		args = append(args, "--oobe-skip-postlogin")
	}

	if !cfg.InstallWebApp() {
		args = append(args, "--disable-features=DefaultWebAppInstallation")
	}

	if cfg.VKEnabled() {
		args = append(args, "--enable-virtual-keyboard")
	}

	// Enable verbose logging on some enrollment related files.
	if cfg.EnableLoginVerboseLogs() {
		args = append(args,
			"--vmodule="+strings.Join([]string{
				"*auto_enrollment_check_screen*=1",
				"*enrollment_screen*=1",
				"*login_display_host_common*=1",
				"*wizard_controller*=1",
				"*auto_enrollment_controller*=1"}, ","))
	}

	if cfg.LoginMode() != config.GAIALogin {
		args = append(args, "--disable-gaia-services")
	}

	// Enable verbose logging on gaia_auth_fetcher to help debug some login failures. See crbug.com/1166530
	if cfg.LoginMode() == config.GAIALogin {
		args = append(args, "--vmodule=gaia_auth_fetcher=1")
	}
	if cfg.SkipForceOnlineSignInForTesting() {
		args = append(args, "--skip-force-online-signin-for-testing")
	}

	args = append(args, exts.ChromeArgs()...)
	if cfg.PolicyEnabled() {
		args = append(args, "--profile-requires-policy=true")
	} else {
		args = append(args, "--profile-requires-policy=false")
	}
	if cfg.DMSAddr() != "" {
		args = append(args, "--device-management-url="+cfg.DMSAddr())
	}
	if cfg.DisablePolicyKeyVerification() {
		args = append(args, "--disable-policy-key-verification")
	}
	switch cfg.ARCMode() {
	case config.ARCDisabled:
		// Make sure ARC is never enabled.
		args = append(args, "--arc-availability=none")
	case config.ARCEnabled:
		args = append(args,
			// Disable ARC opt-in verification to test ARC with mock GAIA accounts.
			"--disable-arc-opt-in-verification",
			// Always start ARC to avoid unnecessarily stopping mini containers.
			"--arc-start-mode=always-start-with-no-play-store")
	case config.ARCSupported:
		// Allow ARC being enabled on the device to test ARC with real gaia accounts.
		args = append(args, "--arc-availability=officially-supported")
	}
	if cfg.ARCMode() == config.ARCEnabled || cfg.ARCMode() == config.ARCSupported {
		args = append(args,
			// Do not sync the locale with ARC.
			"--arc-disable-locale-sync",
			// Do not update Play Store automatically.
			"--arc-play-store-auto-update=off",
			// Make 1 Android pixel always match 1 Chrome devicePixel.
			"--force-remote-shell-scale=")
		if !cfg.RestrictARCCPU() {
			args = append(args,
				// Disable CPU restrictions to let tests run faster
				"--disable-arc-cpu-restriction")
		}
	}

	if cfg.ARCMode() == config.ARCEnabled && cfg.ARCUseHugePages() == true {
		args = append(args,
			// Enable huge pages for guest memory
			"--arcvm-use-hugepages")
	}

	if fs := cfg.EnableFeatures(); len(fs) != 0 {
		args = append(args, "--enable-features="+strings.Join(fs, ","))
	}

	if fs := cfg.DisableFeatures(); len(fs) != 0 {
		args = append(args, "--disable-features="+strings.Join(fs, ","))
	}

	// Lacros Chrome additional arguments are delimited by '####'. See browser_manager.cc in Chrome source.
	if as := cfg.LacrosExtraArgs(); len(args) != 0 {
		args = append(args, "--lacros-chrome-additional-args="+strings.Join(as, "####"))
	}

	args = append(args, cfg.ExtraArgs()...)
	var envVars []string
	if cfg.BreakpadTestMode() {
		envVars = append(envVars,
			"CHROME_HEADLESS=",
			"BREAKPAD_DUMP_LOCATION=/home/chronos/crash") // Write crash dumps outside cryptohome.
	}

	_, err = sm.EnableChromeTestingAndWait(ctx, true, args, envVars)
	return err
}

// restartSession stops the "ui" job, clears policy files and the user's cryptohome if requested,
// and restarts the job.
func restartSession(ctx context.Context, cfg *config.Config) error {
	testing.ContextLog(ctx, "Restarting ui job")
	ctx, st := timing.Start(ctx, "restart_ui")
	defer st.End()

	ctx, cancel := context.WithTimeout(ctx, upstart.UIRestartTimeout)
	defer cancel()

	if err := upstart.StopJob(ctx, "ui"); err != nil {
		return err
	}

	if !cfg.KeepState() {
		const chronosDir = "/home/chronos"
		const shadowDir = "/home/.shadow"

		if !cfg.KeepOwnership() {
			// This always fails because /home/chronos is a mount point, but all files
			// under the directory should be removed.
			os.RemoveAll(chronosDir)
			fis, err := ioutil.ReadDir(chronosDir)
			if err != nil {
				return err
			}
			// Retry cleanup of remaining files. Don't fail if removal reports an error.
			for _, left := range fis {
				if err := os.RemoveAll(filepath.Join(chronosDir, left.Name())); err != nil {
					testing.ContextLogf(ctx, "Failed to clear %s; failed to remove %q: %v", chronosDir, left.Name(), err)
				} else {
					testing.ContextLogf(ctx, "Failed to clear %s; %q needed repeated removal", chronosDir, left.Name())
				}
			}
		}

		// Delete files from shadow directory.
		shadowFiles, err := ioutil.ReadDir(shadowDir)
		if err != nil {
			return errors.Wrapf(err, "failed to read directory %q", shadowDir)
		}
		for _, file := range shadowFiles {
			if !file.IsDir() {
				continue
			}
			// Only look for chronos file with names matching u-*.
			chronosName := filepath.Join(chronosDir, "u-"+file.Name())
			shadowName := filepath.Join(shadowDir, file.Name())
			// Remove the shadow directory if it does not have a corresponding chronos directory.
			if _, err := os.Stat(chronosName); err != nil && os.IsNotExist(err) {
				if err := os.RemoveAll(shadowName); err != nil {
					testing.ContextLogf(ctx, "Failed to remove %q: %v", shadowName, err)
				}
			}
		}

		if !cfg.KeepOwnership() {
			// Delete policy files to clear the device's ownership state since the account
			// whose cryptohome we'll delete may be the owner: http://crbug.com/897278
			if err := session.ClearDeviceOwnership(ctx); err != nil {
				return err
			}
		}
	}

	return upstart.EnsureJobRunning(ctx, "ui")
}
