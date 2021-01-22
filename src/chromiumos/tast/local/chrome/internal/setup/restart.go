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
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/internal/chromeproc"
	"chromiumos/tast/local/chrome/internal/config"
	"chromiumos/tast/local/chrome/internal/extension"
	"chromiumos/tast/local/minidump"
	"chromiumos/tast/local/session"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

// uiRestartTimeout is the maximum amount of time that it takes to restart
// the ui upstart job.
// ui-post-stop can sometimes block for an extended period of time
// waiting for "cryptohome --action=pkcs11_terminate" to finish: https://crbug.com/860519
const uiRestartTimeout = 60 * time.Second

// RestartChromeForTesting restarts the ui job, asks session_manager to enable Chrome testing,
// and waits for Chrome to listen on its debugging port.
func RestartChromeForTesting(ctx context.Context, cfg *config.Config, exts *extension.Files) error {
	ctx, st := timing.Start(ctx, "restart")
	defer st.End()

	if err := restartSession(ctx, cfg); err != nil {
		// Timeout is often caused by TPM slowness. Save minidumps of related processes.
		if dir, ok := testing.ContextOutDir(ctx); ok {
			minidump.SaveWithoutCrash(ctx, dir,
				minidump.MatchByName("chapsd", "cryptohome", "cryptohomed", "session_manager", "tcsd"))
		}
		return err
	}

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		return err
	}

	// Remove the file where Chrome will write its debugging port after it's restarted.
	os.Remove(cdputil.DebuggingPortPath)

	testing.ContextLog(ctx, "Asking session_manager to enable Chrome testing")
	args := []string{
		"--remote-debugging-port=0",                  // Let Chrome choose its own debugging port.
		"--disable-logging-redirect",                 // Disable redirection of Chrome logging into cryptohome.
		"--ash-disable-system-sounds",                // Disable system startup sound.
		"--autoplay-policy=no-user-gesture-required", // Allow media autoplay.
		"--enable-experimental-extension-apis",       // Allow Chrome to use the Chrome Automation API.
		"--redirect-libassistant-logging",            // Redirect libassistant logging to /var/log/chrome/.
		"--no-startup-window",                        // Do not start up chrome://newtab by default to avoid unexpected patterns(doodle etc.)
		"--no-first-run",                             // Prevent showing up offer pages, e.g. google.com/chromebooks.
		"--cros-region=" + cfg.Region,                // Force the region.
		"--cros-regions-mode=hide",                   // Ignore default values in VPD.
		"--enable-oobe-test-api",                     // Enable OOBE helper functions for authentication.
		"--disable-hid-detection-on-oobe",            // Skip OOBE check for keyboard/mouse on chromeboxes/chromebases.
	}
	if cfg.Enroll {
		args = append(args, "--disable-policy-key-verification") // Remove policy key verification for fake enrollment
	}

	if cfg.SkipOOBEAfterLogin {
		args = append(args, "--oobe-skip-postlogin")
	}

	if !cfg.InstallWebApp {
		args = append(args, "--disable-features=DefaultWebAppInstallation")
	}

	if cfg.VKEnabled {
		args = append(args, "--enable-virtual-keyboard")
	}

	// Enable verbose logging on some enrollment related files.
	if cfg.EnableLoginVerboseLogs {
		args = append(args,
			"--vmodule="+strings.Join([]string{
				"*auto_enrollment_check_screen*=1",
				"*enrollment_screen*=1",
				"*login_display_host_common*=1",
				"*wizard_controller*=1",
				"*auto_enrollment_controller*=1"}, ","))
	}

	if cfg.LoginMode != config.GAIALogin {
		args = append(args, "--disable-gaia-services")
	}

	// Enable verbose logging on gaia_auth_fetcher to help debug some login failures. See crbug.com/1166530
	if cfg.LoginMode == config.GAIALogin {
		args = append(args, "--vmodule=gaia_auth_fetcher=1")
	}

	args = append(args, exts.ChromeArgs()...)
	if cfg.PolicyEnabled {
		args = append(args, "--profile-requires-policy=true")
	} else {
		args = append(args, "--profile-requires-policy=false")
	}
	if cfg.DMSAddr != "" {
		args = append(args, "--device-management-url="+cfg.DMSAddr)
	}
	switch cfg.ARCMode {
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
	if cfg.ARCMode == config.ARCEnabled || cfg.ARCMode == config.ARCSupported {
		args = append(args,
			// Do not sync the locale with ARC.
			"--arc-disable-locale-sync",
			// Do not update Play Store automatically.
			"--arc-play-store-auto-update=off",
			// Make 1 Android pixel always match 1 Chrome devicePixel.
			"--force-remote-shell-scale=")
		if !cfg.RestrictARCCPU {
			args = append(args,
				// Disable CPU restrictions to let tests run faster
				"--disable-arc-cpu-restriction")
		}
	}

	if len(cfg.EnableFeatures) != 0 {
		args = append(args, "--enable-features="+strings.Join(cfg.EnableFeatures, ","))
	}

	if len(cfg.DisableFeatures) != 0 {
		args = append(args, "--disable-features="+strings.Join(cfg.DisableFeatures, ","))
	}

	args = append(args, cfg.ExtraArgs...)
	var envVars []string
	if cfg.BreakpadTestMode {
		envVars = append(envVars,
			"CHROME_HEADLESS=",
			"BREAKPAD_DUMP_LOCATION=/home/chronos/crash") // Write crash dumps outside cryptohome.
	}

	// Wait for a browser to start since session_manager can take a while to start it.
	var oldPID int
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		var err error
		oldPID, err = chromeproc.GetRootPID()
		return err
	}, nil); err != nil {
		return errors.Wrap(err, "failed to find the browser process")
	}

	if _, err = sm.EnableChromeTesting(ctx, true, args, envVars); err != nil {
		return err
	}

	// Wait for a new browser to appear.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		newPID, err := chromeproc.GetRootPID()
		if err != nil {
			return err
		}
		if newPID == oldPID {
			return errors.New("Original browser still running")
		}
		return nil
	}, &testing.PollOptions{Interval: 10 * time.Millisecond, Timeout: 10 * time.Second}); err != nil {
		return err
	}
	return nil
}

// restartSession stops the "ui" job, clears policy files and the user's cryptohome if requested,
// and restarts the job.
func restartSession(ctx context.Context, cfg *config.Config) error {
	testing.ContextLog(ctx, "Restarting ui job")
	ctx, st := timing.Start(ctx, "restart_ui")
	defer st.End()

	ctx, cancel := context.WithTimeout(ctx, uiRestartTimeout)
	defer cancel()

	if err := upstart.StopJob(ctx, "ui"); err != nil {
		return err
	}

	if !cfg.KeepState {
		const chronosDir = "/home/chronos"
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

		// Delete files from shadow directory.
		const shadowDir = "/home/.shadow"
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

		// Delete policy files to clear the device's ownership state since the account
		// whose cryptohome we'll delete may be the owner: http://cbug.com/897278
		if err := session.ClearDeviceOwnership(ctx); err != nil {
			return err
		}
	}
	return upstart.EnsureJobRunning(ctx, "ui")
}
