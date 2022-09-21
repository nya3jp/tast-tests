// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package kioskmode provides ways to set policies for local device accounts
// in a Kiosk mode.
package kioskmode

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash/ashproc"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
	"chromiumos/tast/timing"
)

var (
	// WebKioskAccountID identifier of the web Kiosk application.
	WebKioskAccountID   = "arbitrary_id_web_kiosk_1@managedchrome.com"
	webKioskAccountType = policy.AccountTypeKioskWebApp
	webKioskIconURL     = "https://www.google.com"
	webKioskTitle       = "TastKioskModeSetByPolicyGooglePage"
	webKioskURL         = "https://www.google.com"
	// DeviceLocalAccountInfo uses *string instead of string for internal data
	// structure. That is needed since fields in json are marked as omitempty.
	webKioskPolicy = policy.DeviceLocalAccountInfo{
		AccountID:   &WebKioskAccountID,
		AccountType: &webKioskAccountType,
		WebKioskAppInfo: &policy.WebKioskAppInfo{
			Url:     &webKioskURL,
			Title:   &webKioskTitle,
			IconUrl: &webKioskIconURL,
		}}

	// KioskAppAccountID identifier of the Kiosk application.
	KioskAppAccountID   = "arbitrary_id_store_app_2@managedchrome.com"
	kioskAppAccountType = policy.AccountTypeKioskApp
	// KioskAppID pointing to the Printtest app - not listed in the WebStore.
	KioskAppID = "aajgmlihcokkalfjbangebcffdoanjfo"
	// KioskAppBtnName is the name of the Printest app which shows up in the Apps
	// menu on the sign-in screen.
	KioskAppBtnName = "Simple Printest"
	// KioskAppBtnNode node representing this application on the Apps menu on
	// the Sign-in screen.
	KioskAppBtnNode = nodewith.Name(KioskAppBtnName).ClassName("MenuItemView")
	kioskAppPolicy  = policy.DeviceLocalAccountInfo{
		AccountID:   &KioskAppAccountID,
		AccountType: &kioskAppAccountType,
		KioskAppInfo: &policy.KioskAppInfo{
			AppId: &KioskAppID,
		}}

	// DefaultLocalAccountsConfiguration holds default Kiosks accounts
	// configuration. Each, when setting public account policies can be
	// referred by id: KioskAppAccountID and WebKioskAccountID
	DefaultLocalAccountsConfiguration = policy.DeviceLocalAccounts{
		Val: []policy.DeviceLocalAccountInfo{
			kioskAppPolicy,
			webKioskPolicy,
		},
	}
)

const (
	// kioskStartingLog is reported by chrome once the kiosk mode is starting.
	kioskStartingLog = "Starting kiosk mode"
	// kioskReadyToLaunchLog is reported by chrome once the kiosk mode is ready to launch.
	kioskReadyToLaunchLog = "Kiosk app is ready to launch."
	// kioskLaunchSucceededLog is reported by chrome once the kiosk launch is succeeded.
	kioskLaunchSucceededLog = "Kiosk launch succeeded"
	// kioskClosingSplashScreenLog is reported by chrome once the splash screen is gone.
	kioskClosingSplashScreenLog = "App window created, closing splash screen."
)

// Kiosk structure holds necessary references and provides a way to safely
// close Kiosk mode.
type Kiosk struct {
	cr            *chrome.Chrome
	fdms          *fakedms.FakeDMS
	localAccounts *policy.DeviceLocalAccounts
	autostart     bool
}

// Close clears policies, but keeps serving device local accounts then closes
// Chrome. Ideally we would serve an empty policies slice however, that makes
// Chrome crashes when AutoLaunch() option was used.
func (k *Kiosk) Close(ctx context.Context) (retErr error) {
	// If Chrome fails to start in RestartChromeWithOptions it has already been
	// cleaned up by startChromeClearPolicies.
	if k.cr == nil {
		return errors.New("Skipping kiosk.Close() because Chrome is nil")
	}

	// Using defer to make sure Chrome is always closed.
	defer func(ctx context.Context) {
		if err := k.cr.Close(ctx); err != nil {
			// Chrome error supersedes previous error if any.
			retErr = errors.Wrap(err, "could not close Chrome while closing Kiosk session")
		}
	}(ctx)

	var policies []policy.Policy
	// When AutoLaunch() option was used, then the corresponding policy has to
	// be removed before starting a new Chrome session. Otherwise Kiosk will
	// start again. When applying an empty policies slice, Chrome crashes.
	// Hence the safest way is to apply local accounts again. That way Chrome
	// will load them but will start normally. If the next tests want to use
	// policy they will override them.
	if k.autostart {
		policies = append(policies, k.localAccounts)
	}
	if err := policyutil.ServeAndRefresh(ctx, k.fdms, k.cr, policies); err != nil {
		testing.ContextLog(ctx, "Could not serve and refresh policies. If kioskmode.AutoLaunch() option was used it may impact next test : ", err)
		return errors.Wrap(err, "could not clear policies")
	}
	return nil
}

// ConfirmKioskStarted uses reader for looking for logs that confirm Kiosk
// mode starting, ready for launch, and successful launch of Kiosk.
// reader Reader instance should be processing logs filtered for Chrome only.
func ConfirmKioskStarted(ctx context.Context, reader *syslog.Reader) error {
	if err := confirmKioskInitialized(ctx, reader); err != nil {
		return errors.Wrap(err, "failed to verify starting sequence of Kiosk mode")
	}

	testing.ContextLog(ctx, "Waiting for successful Kiosk mode launch")
	// Wait for kioskLaunchSucceededLog to be present in logs. Used timeout
	// accommodates for launching up from the moment all data for its launch
	// is available.
	if _, err := reader.Wait(ctx, 60*time.Second,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, kioskLaunchSucceededLog)
		},
	); err != nil {
		return errors.Wrap(err, "failed to verify successful launch of Kiosk mode")
	}

	return nil
}

// confirmKioskInitialized uses reader for looking for logs that confirm Kiosk
// mode starting and ready for launch.
func confirmKioskInitialized(ctx context.Context, reader *syslog.Reader) error {
	testing.ContextLog(ctx, "Waiting for Kiosk mode start")
	// Wait for kioskStartingLog to be present in logs. Used timeout should be
	// rather short as this kicks off the Kiosk right away.
	if _, err := reader.Wait(ctx, 30*time.Second,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, kioskStartingLog)
		},
	); err != nil {
		return errors.Wrap(err, "failed to verify starting of Kiosk mode")
	}

	// Wait for kioskReadyToLaunchLog to be present in logs. Used timeout needs
	// to accommodate downloading apps, and extensions. It should be kept over
	// one minute.
	if _, err := reader.Wait(ctx, 90*time.Second,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, kioskReadyToLaunchLog)
		},
	); err != nil {
		return errors.Wrap(err, "failed to verify Kiosk being ready for launch")
	}

	return nil
}

// IsKioskAppStarted searches for existing logs to confirm Kiosk is running.
func IsKioskAppStarted(ctx context.Context) error {
	logContent, err := ioutil.ReadFile(syslog.ChromeLogFile)
	if err != nil {
		return errors.Wrap(err, "failed to read "+syslog.ChromeLogFile)
	}

	if !strings.Contains(string(logContent), kioskClosingSplashScreenLog) {
		return errors.New("failed to verify successful launch of Kiosk mode")
	}
	return nil
}

// New starts Chrome, sets passed Kiosk related options to policies and
// restarts Chrome. When kioskmode.AutoLaunch() is used, then it auto starts
// given Kiosk application. Alternatively use kioskmode.ExtraChromeOptions()
// passing chrome.LoadSigninProfileExtension(). In that case Chrome is started
// and stays on Signin screen with Kiosk accounts loaded.
// Use defer kiosk.Close(ctx) to clean.
func New(ctx context.Context, fdms *fakedms.FakeDMS, opts ...Option) (*Kiosk, *chrome.Chrome, error) {
	cfg, err := NewConfig(opts)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to process options")
	}

	if cfg.m.DeviceLocalAccounts == nil {
		return nil, nil, errors.Wrap(err, "local device accounts were not set")
	}

	err = func(ctx context.Context) error {
		testing.ContextLog(ctx, "Kiosk mode: Starting Chrome to set Kiosk policies")
		cr, err := chrome.New(
			ctx,
			chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}), // Required as refreshing policies require test API.
			chrome.DMSPolicy(fdms.URL),
			chrome.KeepEnrollment(),
		)
		if err != nil {
			return errors.Wrap(err, "failed to start Chrome")
		}

		// Close the previous Chrome instance.
		defer cr.Close(ctx)

		// Set local accounts policy.
		policies := []policy.Policy{
			cfg.m.DeviceLocalAccounts,
		}

		// Handle the AutoLaunch setup.
		if cfg.m.AutoLaunch == true {
			policies = append(policies, &policy.DeviceLocalAccountAutoLoginId{
				Val: *cfg.m.AutoLaunchKioskAppID,
			})
		}

		// Handle setting device policies.
		if cfg.m.ExtraPolicies != nil {
			policies = append(policies, cfg.m.ExtraPolicies...)
		}

		pb := policy.NewBlob()
		pb.AddPolicies(policies)
		// Handle public account policies.
		if cfg.m.PublicAccountPolicies != nil {
			for accountID, policies := range cfg.m.PublicAccountPolicies {
				pb.AddPublicAccountPolicies(accountID, policies)
			}
		}
		// Handle custom directory api id.
		if cfg.m.CustomDirectoryAPIID != nil {
			pb.DirectoryAPIID = *cfg.m.CustomDirectoryAPIID
		}
		// Update policies.
		if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
			// In case of AutoLaunch was used we try to override policies with
			// local accounts similarly as in kioskmode.Close().
			if cfg.m.AutoLaunch == true {
				if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{cfg.m.DeviceLocalAccounts}); err != nil {
					testing.ContextLog(ctx, "Could not serve and refresh policies. If kioskmode.AutoLaunch() option was used it may impact next test : ", err)
				}
			}
			return errors.Wrap(err, "failed to serve and refresh policies")
		}

		return nil
	}(ctx)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed preparing Chrome to start with given Kiosk configuration")
	}

	reader, err := syslog.NewReader(ctx, syslog.Program("chrome"))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start log reader")
	}
	defer reader.Close()

	var cr *chrome.Chrome
	if cfg.m.AutoLaunch {
		opts := []chrome.Option{
			chrome.NoLogin(),
			chrome.DMSPolicy(fdms.URL),
			chrome.KeepEnrollment(),
		}
		opts = append(opts, cfg.m.ExtraChromeOptions...)

		testing.ContextLog(ctx, "Kiosk mode: Starting Chrome in Kiosk mode")
		// Restart Chrome. After that Kiosk auto starts.
		cr, err = chrome.New(ctx, opts...)
		if err != nil {
			if err := startChromeClearPolicies(ctx, fdms, fixtures.Username, fixtures.Password); err != nil {
				return nil, nil, errors.Wrap(err, "could not finish cleanup")
			}
			return nil, nil, errors.Wrap(err, "Chrome restart failed")
		}

		if !cfg.m.SkipSuccessfulLaunchCheck {
			// Library waits for Kiosk start sequence to start then it checks
			// that Kiosk is ready for launch, and finally it waits for Kiosk
			// to be launched.
			if err := ConfirmKioskStarted(ctx, reader); err != nil {
				if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{cfg.m.DeviceLocalAccounts}); err != nil {
					testing.ContextLog(ctx, "Could not serve and refresh policies. If kioskmode.AutoLaunch() option was used it may impact next test: ", err)
				}
				cr.Close(ctx)
				return nil, nil, errors.Wrap(err, "there was a problem while checking chrome logs for Kiosk related entries")
			}
		} else {
			// If a test does not want to wait for Kiosk to be launched the
			// library will still make sure that Kiosk start sequence started
			// and Kiosk is ready for launch.
			if err := confirmKioskInitialized(ctx, reader); err != nil {
				if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{cfg.m.DeviceLocalAccounts}); err != nil {
					testing.ContextLog(ctx, "Could not serve and refresh policies. If kioskmode.AutoLaunch() option was used it may impact next test: ", err)
				}
				cr.Close(ctx)
				return nil, nil, errors.Wrap(err, "there was a problem while checking chrome logs for Kiosk startup")
			}
		}
	} else {
		opts := []chrome.Option{
			chrome.DeferLogin(),
			chrome.DMSPolicy(fdms.URL),
			chrome.KeepEnrollment(),
		}
		opts = append(opts, cfg.m.ExtraChromeOptions...)

		testing.ContextLog(ctx, "Kiosk mode: Starting Chrome on Signin screen with set Kiosk apps")
		// Restart Chrome. Chrome stays on Sing-in screen
		cr, err = chrome.New(ctx, opts...)
		if err != nil {
			return nil, nil, errors.Wrap(err, "Chrome restart failed")
		}
	}

	return &Kiosk{cr: cr, fdms: fdms, localAccounts: cfg.m.DeviceLocalAccounts, autostart: cfg.m.AutoLaunch}, cr, nil
}

// startChromeClearPolicies is called when Chrome fails to start in autostart
// mode - when kioskmode.AutoLaunch() option was used. We need to start Chrome
// and clean policies to prevent Chrome starting automatically in Kiosk mode
// for next test.
// FIXME: this cleanup doesn't work on some devices (e.g. chell). FakeLogin()
// doesn't work either. Need to figure out some way to fix this.
func startChromeClearPolicies(ctx context.Context, fdms *fakedms.FakeDMS, username, password string) error {
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
	)
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome for cleanup")
	}
	defer cr.Close(ctx)

	if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{}); err != nil {
		return errors.Wrap(err, "failed to clear policies")
	}
	return nil
}

// WaitForCrxInCache waits for Kiosk crx to be available in cache.
func WaitForCrxInCache(ctx context.Context, id string) error {
	const crxCachePath = "/home/chronos/kiosk/crx/"
	ctx, st := timing.Start(ctx, "wait_crx_cache")
	defer st.End()

	return testing.Poll(ctx, func(ctx context.Context) error {
		files, err := ioutil.ReadDir(crxCachePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return errors.Wrap(err, "Kiosk crx cache does not exist yet")
			}
			return testing.PollBreak(errors.Wrap(err, "failed to list content of Kiosk cache"))
		}

		for _, file := range files {
			if strings.HasPrefix(file.Name(), id) {
				testing.ContextLog(ctx, "Found crx in cache: "+file.Name())
				return nil
			}
		}

		return errors.Wrap(err, "Kiosk crx cache does not have "+id)
	}, nil)
}

// restartChromeNoCloseWithOptions replaces the current Chrome in kiosk instance
// with a new one using custom options without closing the old one. It will be
// closed by Kiosk.Close(). Useful when Chrome already closes itself, for
// example when cancelling a Kiosk launch.
func (k *Kiosk) restartChromeNoCloseWithOptions(ctx context.Context, opts ...chrome.Option) (*chrome.Chrome, error) {
	k.cr = nil

	testing.ContextLog(ctx, "Restarting ui")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		if err := startChromeClearPolicies(ctx, k.fdms, fixtures.Username, fixtures.Password); err != nil {
			return nil, errors.Wrap(err, "could not finish cleanup")
		}
		return nil, errors.Wrap(err, "failed to restart ui")
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		if err := startChromeClearPolicies(ctx, k.fdms, fixtures.Username, fixtures.Password); err != nil {
			return nil, errors.Wrap(err, "could not finish cleanup")
		}
		return nil, errors.Wrap(err, "failed to start new Chrome")
	}
	k.cr = cr
	return cr, err
}

// RestartChromeWithOptions replaces the current Chrome in kiosk instance with
// a new one using custom options. It will be closed by Kiosk.Close().
func (k *Kiosk) RestartChromeWithOptions(ctx context.Context, opts ...chrome.Option) (*chrome.Chrome, error) {
	if err := k.cr.Close(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to close Chrome")
	}
	return k.restartChromeNoCloseWithOptions(ctx, opts...)
}

// StartFromSignInScreen starts a Kiosk app from the Apps menu on the sign-in
// screen, simulating a manual launch. It doesn't wait for a successful launch
// so that the launch can be cancelled by pressing Ctrl+Alt+S.
// TODO(b/230840565): Extract and extend this function to support MGS.
func StartFromSignInScreen(ctx context.Context, ui *uiauto.Context, name string) error {
	testing.ContextLog(ctx, "Starting Kiosk app from sign-in screen: "+name)
	localAccountsBtn := nodewith.Name("Apps").HasClass("MenuButton")
	kioskAppBtn := nodewith.Name(name).HasClass("MenuItemView")
	cancelLaunchText := nodewith.Name("Press Ctrl + Alt + S to switch to ChromeOS").Role("staticText")
	if err := uiauto.Combine("launch Kiosk app from menu",
		ui.WaitUntilExists(localAccountsBtn),
		ui.LeftClick(localAccountsBtn),
		ui.WaitUntilExists(kioskAppBtn),
		ui.LeftClick(kioskAppBtn),
		ui.WaitUntilExists(cancelLaunchText),
	)(ctx); err != nil {
		return errors.Wrap(err, "failed to start Kiosk application from apps menu")
	}
	return nil
}

// CancelKioskLaunch cancels the current Kiosk launch by pressing Ctrl+Alt+S.
// Must be invoked on the Kiosk splash screen. A new Chrome instance will be
// started with given options. It verifies a successful cancel by checking for
// cancelled message on the screen.
func (k *Kiosk) CancelKioskLaunch(ctx context.Context, opts ...chrome.Option) (*chrome.Chrome, error) {
	const disableChromeRestartFile = "/run/disable_chrome_restart"

	testing.ContextLog(ctx, "Cancelling Kiosk launch via Ctrl+Alt+S")
	kw, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a keyboard")
	}
	defer kw.Close()

	if err := chrome.PrepareForRestart(); err != nil {
		return nil, errors.Wrap(err, "failed to remove old dev tools port file")
	}

	// Create the flag file to make sure session_manager does not start Chrome
	// again after Chrome exits.
	_, err = os.Create(disableChromeRestartFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Chrome flag file")
	}
	defer func(ctx context.Context) {
		if err := os.RemoveAll(disableChromeRestartFile); err != nil && !os.IsNotExist(err) {
			testing.ContextLog(ctx, "Failed to remove flag file: ", err)
		}
	}(ctx)

	// Find the current Chrome process to wait for it to shut down later.
	old, err := ashproc.WaitForRoot(ctx, time.Minute)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find the browser process")
	}

	if err := kw.Accel(ctx, "Ctrl+Alt+S"); err != nil {
		return nil, errors.Wrap(err, "failed to hit Ctrl+Alt+S and attempt to quit a kiosk app")
	}

	// Wait for the current Chrome to shut down.
	if err := procutil.WaitForTerminated(ctx, old, 10*time.Second); err != nil {
		return nil, errors.Wrap(err, "browser process didn't terminate")
	}

	// Remove flag file so that session_manager will start Chrome after UI task is
	// restarted.
	if err := os.RemoveAll(disableChromeRestartFile); err != nil {
		return nil, errors.Wrap(err, "failed to remove flag file")
	}

	// Restart Chrome without closing since the current Chrome process has already
	// exited itself.
	cr, err := k.restartChromeNoCloseWithOptions(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to restart Chrome")
	}
	return cr, nil
}
