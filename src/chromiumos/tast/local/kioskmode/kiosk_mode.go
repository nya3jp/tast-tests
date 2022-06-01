// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
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
	// kioskLaunchSucceededLog is reported by chrome once the kiosk launch is succeeded.
	kioskLaunchSucceededLog = "Kiosk launch succeeded"
	// kioskClosingSplashScreenLog is reported by chtome once the splash screen is gone.
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
// mode starting and also successful launch of Kiosk.
// reader Reader instance should be processing logs filtered for Chrome only.
func ConfirmKioskStarted(ctx context.Context, reader *syslog.Reader) error {
	// Check that Kiosk starts successfully.
	testing.ContextLog(ctx, "Waiting for Kiosk mode start")

	const (
		// logScanTimeout is a timeout for log messages indicating Kiosk startup
		// and successful launch to be present. It is set to over a minute as Kiosk
		// mode launch varies depending on device. crbug.com/1222136
		logScanTimeout = 90 * time.Second
	)

	if _, err := reader.Wait(ctx, logScanTimeout,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, kioskStartingLog)
		},
	); err != nil {
		return errors.Wrap(err, "failed to verify starting of Kiosk mode")
	}

	testing.ContextLog(ctx, "Waiting for successful Kiosk mode launch")
	if _, err := reader.Wait(ctx, logScanTimeout,
		func(e *syslog.Entry) bool {
			return strings.Contains(e.Content, kioskLaunchSucceededLog)
		},
	); err != nil {
		return errors.Wrap(err, "failed to verify successful launch of Kiosk mode")
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

		if err := ConfirmKioskStarted(ctx, reader); err != nil {
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{cfg.m.DeviceLocalAccounts}); err != nil {
				testing.ContextLog(ctx, "Could not serve and refresh policies. If kioskmode.AutoLaunch() option was used it may impact next test : ", err)
			}
			cr.Close(ctx)
			return nil, nil, errors.Wrap(err, "there was a problem while checking chrome logs for Kiosk related entries")
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
	testing.ContextLog(ctx, "Cancelling Kiosk launch via Ctrl+Alt+S")
	kw, err := input.Keyboard(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a keyboard")
	}
	defer kw.Close()

	if err := kw.Accel(ctx, "Ctrl+Alt+S"); err != nil {
		return nil, errors.Wrap(err, "failed to hit Ctrl+Alt+S and attempt to quit a kiosk app")
	}
	// The current Chrome process will exit after pressing Ctrl+Alt+S. Waiting for
	// UI events is therefore not possible. A short delay is needed to make sure
	// launch error event will be triggered before restarting UI.
	if err := testing.Sleep(ctx, 300*time.Millisecond); err != nil {
		return nil, errors.Wrap(err, "failed to wait for Chrome to exit")
	}

	// Restart Chrome without closing since the current Chrome process has already
	// exited itself.
	cr, err := k.restartChromeNoCloseWithOptions(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to restart Chrome")
	}
	return cr, nil
}
