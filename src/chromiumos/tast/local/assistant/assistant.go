// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package assistant supports interaction with Assistant service.
package assistant

import (
	"context"
	"path/filepath"

	"chromiumos/tast/errors"
	"chromiumos/tast/framework/protocol"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

// QueryResponse contains a subset of the results returned from the Assistant server
// when it received a query. This struct contains the only fields which are used in
// the tests.
type QueryResponse struct {
	// Contains the HTML string of the response.
	HTML string `json:"htmlResponse"`
}

// QueryStatus contains a subset of the status of an interaction with Assistant started
// by sending a query, e.g. query text, mic status, and query response. This struct contains
// the only fields which are used in the tests.
//
// TODO(meilinw): Add a reference for the struct after the API change landed (crrev.com/c/1552293).
type QueryStatus struct {
	QueryResponse `json:"queryResponse"`
}

// Accelerator used by Assistant
type Accelerator ash.Accelerator

// Accelerators to toggle Assistant UI
var (
	AccelAssistantKey = Accelerator{KeyCode: "assistant"}
	AccelSearchPlusA  = Accelerator{KeyCode: "a", Search: true}
)

// Enable brings up Google Assistant service and returns any errors.
func Enable(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setAssistantEnabled)`, true, 10*1000 /* timeout_ms */)
}

// Disable stops the Google Assistant service and returns any errors.
func Disable(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setAssistantEnabled)`, false, 10*1000 /* timeout_ms */)
}

// Cleanup stops the Google Assistant service so other tests are not impacted.
// If a failure happened, we make a screenshot beforehand so the Assistant UI
// is visible in the screenshot.
func Cleanup(ctx context.Context, hasError func() bool, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	if hasError() {
		outDir, ok := testing.ContextOutDir(ctx)
		if !ok {
			return errors.New("outdir not available")
		}
		screenshot.CaptureChrome(ctx, cr, filepath.Join(outDir, "screenshot.png"))
	}

	return Disable(ctx, tconn)
}

// EnableAndWaitForReady brings up Google Assistant service, waits for
// NEW_READY signal and returns any errors.
func EnableAndWaitForReady(ctx context.Context, tconn *chrome.TestConn) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.enableAssistantAndWaitForReady)`)
}

// ResolveAssistantHotkey resolves an Assistant hotkey of a device. Search+A is a hotkey to activate
// the Assistant if a device does not have an Assistant key.
func ResolveAssistantHotkey(dutFeatures *protocol.DUTFeatures) (Accelerator, error) {
	satisfied, _, err := hwdep.AssistantKey().Satisfied(dutFeatures.GetHardware())
	if err != nil {
		return Accelerator{}, errors.Wrap(err, "failed to resolve existence of an Assistant key on a device")
	}

	if satisfied {
		return AccelAssistantKey, nil
	}

	return AccelSearchPlusA, nil
}

// SendTextQuery sends text query to Assistant and returns the query status.
func SendTextQuery(ctx context.Context, tconn *chrome.TestConn, query string) (QueryStatus, error) {
	var status QueryStatus
	err := tconn.Call(ctx, &status, `tast.promisify(chrome.autotestPrivate.sendAssistantTextQuery)`, query, 30*1000 /* timeout_ms */)
	return status, err
}

// SendTextQueryViaUI sends a text query via launcher.
// Comparing with SendTextQuery, this one does not wait an Assistant response.
func SendTextQueryViaUI(ctx context.Context, tconn *chrome.TestConn, query string, accel Accelerator) error {
	// Open Assistant UI.
	if err := ToggleUIWithHotkey(ctx, tconn, accel); err != nil {
		return errors.Wrap(err, "failed to toggle Assistant UI with hotkey")
	}

	assistantUI := nodewith.HasClass("AssistantDialogPlate")
	if err := uiauto.New(tconn).WaitUntilExists(assistantUI)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait AssistantDialogPlate")
	}

	// In tablet mode, default input mode is voice. Press AssistantButton to change it to text.
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get tablet mode status")
	}

	if tabletModeEnabled {
		if err := uiauto.New(tconn).LeftClick(nodewith.HasClass("AssistantButton"))(ctx); err != nil {
			return errors.Wrap(err, "failed to click AssistantButton to switch to text query mode")
		}
	}

	// Text filed can be animating/not focused state. Wait until it gets focused before start typing a query.
	if err := uiauto.New(tconn).EnsureFocused(nodewith.HasClass("AssistantTextfield"))(ctx); err != nil {
		return errors.Wrap(err, "failed to wait AssistantTextfield getting focused to type a query")
	}

	// Type query.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create a KeyboardEventWriter")
	}
	defer kb.Close()

	if err := kb.Type(ctx, query); err != nil {
		return errors.Wrapf(err, "failed to type query: %s", query)
	}
	if err := kb.TypeKey(ctx, input.KEY_ENTER); err != nil {
		return errors.Wrap(err, "failed to type Enter key to submit a query")
	}

	return nil
}

// SetHotwordEnabled turns on/off "OK Google" hotword detection for Assistant.
func SetHotwordEnabled(ctx context.Context, tconn *chrome.TestConn, enabled bool) error {
	return setPrefValue(ctx, tconn, "settings.voice_interaction.hotword.enabled", enabled)
}

// SetContextEnabled enables/disables the access to the screen context for Assistant.
// This pref corresponds to the "Related Info" setting of Assistant.
func SetContextEnabled(ctx context.Context, tconn *chrome.TestConn, enabled bool) error {
	return setPrefValue(ctx, tconn, "settings.voice_interaction.context.enabled", enabled)
}

// SetVoiceInteractionConsentValue enables/disables the consent value for Assistant voice interaction.
func SetVoiceInteractionConsentValue(ctx context.Context, tconn *chrome.TestConn, value int) error {
	return setPrefValue(ctx, tconn, "settings.voice_interaction.activity_control.consent_status", value)
}

// SetBetterOnboardingEnabled enables/disables the Assistant onboarding feature
// by controlling the number of sessions where onboarding screen has shown.
// Note that true pref value will *not* be restored later, so tests that need
// this feature must explicitly enable it during setup. Also note that once
// better onboarding has been activated for a session, it will remain enabled
// for the duration of that session until an Assistant interaction happens.
// It is recommended to disable Better Onboarding for Assistant performance
// tests that are not explicitly testing the Better Onboarding feature.
func SetBetterOnboardingEnabled(ctx context.Context, tconn *chrome.TestConn, enabled bool) error {
	// The maximum number of user sessions in which to show Assistant onboarding.
	// Please keep it synced to |kOnboardingMaxSessionsShown| stored in
	// ash/assistant/ui/assistant_ui_constants.h.
	value := 3
	if enabled {
		value = 0
	}
	return setPrefValue(ctx, tconn, "ash.assistant.num_sessions_where_onboarding_shown", value)
}

// setPrefValue is a helper function to set value for Assistant related preferences.
func setPrefValue(ctx context.Context, tconn *chrome.TestConn, prefName string, value interface{}) error {
	return tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setAllowedPref)`, prefName, value)
}

// ToggleUIWithHotkey mimics the Assistant key press to open/close the Assistant UI.
func ToggleUIWithHotkey(ctx context.Context, tconn *chrome.TestConn, accel Accelerator) error {
	if err := tconn.Call(ctx, nil, `async (accel) => {
		  accel.pressed = true;
		  // Triggers the hotkey pressed event.
		  await tast.promisify(chrome.autotestPrivate.activateAccelerator)(accel);
		  // Releases the key for cleanup. This release event will not be handled as it is
		  // not a registered accelerator, so we ignore the result and don't wait for this
		  // async call returned.
		  accel.pressed = false;
		  chrome.autotestPrivate.activateAccelerator(accel, () => {});
		}`, accel); err != nil {
		return errors.Wrap(err, "failed to execute accelerator")
	}

	return nil
}

// VerboseLogging is a helper function passed into chrome.New which will:
//   - Enable VLOG traces in the assistant code.
//   - Enable PII in VLOG traces in the assistant code. This will log the
//     actual queries sent, and the replies received.
func VerboseLogging() chrome.Option {
	return chrome.ExtraArgs(
		"--vmodule=*/assistant/*=3",
		"--enable-features=AssistantDebugging",
	)
}

// VerboseLoggingEnabled creates a new precondition which can be shared by
// tests that require an already-started Chromeobject with verbose logging
// enabled.
func VerboseLoggingEnabled() testing.Precondition { return verboseLoggingPre }

var verboseLoggingPre = chrome.NewPrecondition("verbose-logging", VerboseLogging())

// Assistant tests use Google News as a test app.
const (
	// Apk name of a test apk (fake Google News app).
	ApkName = "AssistantAndroidAppTest.apk"
	// Package name of the test apk.
	GoogleNewsPackageName = "com.google.android.apps.magazines"
	// Application name of Google News Android.
	GoogleNewsAppTitle = "Google News"
	// Chrome window title of Google News Web.
	GoogleNewsWebTitle = "Chrome - Google News"
	// Web URL of Google News.
	GoogleNewsWebURL = "https://news.google.com/"
)

// InstallTestApkAndWaitReady installs a test apk and waits for it to become available.
func InstallTestApkAndWaitReady(ctx context.Context, tconn *chrome.TestConn, a *arc.ARC) error {
	if err := a.Install(ctx, arc.APKPath(ApkName)); err != nil {
		return errors.Wrap(err, "failed to install a test app")
	}

	if err := pollForArcPackageAvailable(ctx, tconn, GoogleNewsPackageName); err != nil {
		return errors.Wrap(err, "failed to wait arc package becomes available")
	}

	return nil
}

type arcPackageDict struct {
	PackageName         string  `json:"packageName"`
	PackageVersion      int64   `json:"packageVersion"`
	LastBackupAndroidID string  `json:"lastBackupAndroidId"`
	LastBackupTime      float64 `json:"lastBackupTime"`
	ShouldSync          bool    `json:"shouldSync"`
	System              bool    `json:"system"`
	VpnProvider         bool    `json:"vpnProvider"`
}

func pollForArcPackageAvailable(ctx context.Context, tconn *chrome.TestConn, packageName string) error {
	f := func(ctx context.Context) error {
		var packageDict arcPackageDict
		return tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.getArcPackage.bind(this,"`+packageName+`"))()`, &packageDict)
	}
	return testing.Poll(ctx, f, &testing.PollOptions{})
}

// WaitForGoogleNewsWebActivation waits that Google New Web gets focused.
func WaitForGoogleNewsWebActivation(ctx context.Context, tconn *chrome.TestConn) error {
	predGoogleNewsWeb := func(window *ash.Window) bool {
		return window.IsActive && window.Title == GoogleNewsWebTitle && window.IsVisible && window.ARCPackageName == ""
	}
	if err := ash.WaitForCondition(ctx, tconn, predGoogleNewsWeb, &testing.PollOptions{}); err != nil {
		return errors.Wrap(err, "failed to confirm that Google News web page gets opened")
	}
	return nil
}

// WaitForGoogleNewsAppActivation waits that Google News Android gets focused.
func WaitForGoogleNewsAppActivation(ctx context.Context, tconn *chrome.TestConn) error {
	predGoogleNewsApp := func(window *ash.Window) bool {
		return window.IsActive && window.IsVisible && window.ARCPackageName == GoogleNewsPackageName
	}
	if err := ash.WaitForCondition(ctx, tconn, predGoogleNewsApp, &testing.PollOptions{}); err != nil {
		return errors.Wrap(err, "failed to confirm that Google News app gets opened")
	}
	return nil
}
