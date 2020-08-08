// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package settingsapp supports controlling the Settings app on Chrome OS.
package settingsapp

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/ui"
)

// SettingsApp represents an instance of the Settings app.
type SettingsApp struct {
	Root  *ui.Node
	Conn  *chrome.Conn
	tconn *chrome.TestConn
}

const defaultTimeout = 10 * time.Second

// SettingsSection is the name of a section in the Settings app.
// These names also correspond to the Name attribute of their respective heading nodes in the UI.
type SettingsSection string

// Names for the Setting app sections.
const (
	DateAndTime        SettingsSection = "Date and time"
	PrivacyAndSecurity                 = "Privacy and security"
	LanguagesAndInput                  = "Languages and input"
	Files                              = "Files"
	Printing                           = "Printing"
	Accessibility                      = "Accessibility"
	ResetSettings                      = "Reset settings"
)

// AdvancedSubHeadings contains the sections found within the 'Advanced' section of the Settings app.
var AdvancedSubHeadings = []SettingsSection{
	DateAndTime,
	PrivacyAndSecurity,
	LanguagesAndInput,
	Files,
	Printing,
	Accessibility,
	ResetSettings,
}

// ChromeConnection gets a Chrome connection to an already-running instance of the Settings app.
// It also will wait for the Settings app to load by checking its document.readyState property,
// which seems to be a good indicator that the app is ready to be used.
func ChromeConnection(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	settingsConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://os-settings/"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get chrome connection to settings app")
	}

	if err := settingsConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "failed waiting for settings app document state to be ready")
	}

	return settingsConn, nil
}

// Launch will launch the Settings app and wait for it to be ready in a number of ways:
// 1. Wait until the Settings app is present in the shelf.
// 2. Establish a Chrome connection to the Settings app and wait for the page to be loaded.
// 3. Find the Settings window in the UI.
// The 'launch' argument may be given as False to perform the waiting steps for an already-launched
// Settings app instance and get a SettingsApp struct for it.
func Launch(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, launch bool) (*SettingsApp, error) {
	if err := apps.Launch(ctx, tconn, apps.Settings.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch settings app")
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Settings.ID); err != nil {
		return nil, errors.Wrap(err, "settings app did not appear in the shelf")
	}

	settingsConn, err := ChromeConnection(ctx, cr)
	if err != nil {
		return nil, err
	}

	r, err := regexp.Compile("Settings")
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regexp for settings app name param")
	}
	attributes := map[string]interface{}{
		"name": r,
	}
	settingsRootParams := ui.FindParams{
		Role:       ui.RoleTypeWindow,
		ClassName:  "BrowserRootView",
		Attributes: attributes,
	}
	settingsRoot, err := ui.FindWithTimeout(ctx, tconn, settingsRootParams, defaultTimeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get settings root node")
	}

	return &SettingsApp{Root: settingsRoot, Conn: settingsConn, tconn: tconn}, nil
}

// Close will close the Settings app and release its underlying resources.
// This should be deferred by caller after calling Launch.
func (s *SettingsApp) Close(ctx context.Context) error {
	s.Root.Release(ctx)

	if err := s.Conn.Close(); err != nil {
		return errors.Wrap(err, "failed to close settings app chrome conn")
	}

	if err := apps.Close(ctx, s.tconn, apps.Settings.ID); err != nil {
		return errors.Wrap(err, "failed to close settings app")
	}

	return nil
}

// ToggleAdvanced clicks the 'Advanced' button at the bottom of the Settings app, expanding or collapsing the Advanced settings section.
func (s *SettingsApp) ToggleAdvanced(ctx context.Context) error {
	// Find the "Advanced" heading and associated button.
	advHeadingParams := ui.FindParams{
		Role: ui.RoleTypeHeading,
		Name: "Advanced",
	}
	advHeading, err := s.Root.DescendantWithTimeout(ctx, advHeadingParams, defaultTimeout)
	if err != nil {
		return errors.Wrap(err, "failed waiting for 'advanced' heading")
	}
	defer advHeading.Release(ctx)

	advBtn, err := advHeading.DescendantWithTimeout(ctx, ui.FindParams{Role: ui.RoleTypeButton, Name: "Advanced"}, defaultTimeout)
	if err != nil {
		return errors.Wrap(err, "failed waiting for 'advanced' button")
	}
	defer advBtn.Release(ctx)

	// Click the Advanced button to expand/collapse the section.
	// We need to focus the button first so it will be clickable.
	if advBtn.State[ui.StateTypeOffscreen] {
		if err := advBtn.FocusAndWait(ctx, defaultTimeout); err != nil {
			return errors.Wrap(err, "failed to move focus to the 'advanced' button")
		}
	}

	if err := advBtn.LeftClick(ctx); err != nil {
		return errors.Wrap(err, "failed to click the 'advanced' button")
	}

	return nil
}

// WaitForAdvancedSectionShown waits for all Advanced settings sections to be present within the expanded 'Advanced' section.
// Returns a list of missing section headings and an error if not all sections are shown in the UI.
func (s *SettingsApp) WaitForAdvancedSectionShown(ctx context.Context) ([]SettingsSection, error) {
	var missingHeadings []SettingsSection
	for _, heading := range AdvancedSubHeadings {
		if err := s.Root.WaitUntilDescendantExists(ctx, ui.FindParams{Role: ui.RoleTypeHeading, Name: string(heading)}, defaultTimeout); err != nil {
			missingHeadings = append(missingHeadings, heading)
		}
	}

	if len(missingHeadings) > 0 {
		return missingHeadings, errors.New("advanced section not completely shown")
	}

	return missingHeadings, nil
}

// WaitForAdvancedSectionHidden waits for all Advanced settings sections to be hidden.
// Returns a list of visible section headings and an error if any sections are present in the UI.
func (s *SettingsApp) WaitForAdvancedSectionHidden(ctx context.Context) ([]SettingsSection, error) {
	var visibleHeadings []SettingsSection
	for _, heading := range AdvancedSubHeadings {
		if err := s.Root.WaitUntilDescendantGone(ctx, ui.FindParams{Role: ui.RoleTypeHeading, Name: string(heading)}, defaultTimeout); err != nil {
			visibleHeadings = append(visibleHeadings, heading)
		}
	}

	if len(visibleHeadings) > 0 {
		return visibleHeadings, errors.New("advanced section not completely hidden")
	}

	return visibleHeadings, nil
}

// LaunchWhatsNew launches the What's New PWA (a.k.a. Release Notes).
func (s *SettingsApp) LaunchWhatsNew(ctx context.Context) error {
	// Launch What's New using the Settings page JS functions. The same JS is tied to the UI link's on-click property.
	if err := s.Conn.WaitForExpr(ctx, `typeof(settings) == "object"`); err != nil {
		return errors.Wrap(err, "failed waiting for settings object to exist")
	}
	if err := s.Conn.Eval(ctx,
		"settings.AboutPageBrowserProxyImpl.getInstance().launchReleaseNotes()",
		nil); err != nil {
		return errors.Wrap(err, "failed to run javascript to launch what's new")
	}

	// Wait for What's New to open by checking in the shelf.
	if err := ash.WaitForApp(ctx, s.tconn, apps.WhatsNew.ID); err != nil {
		return errors.Wrap(err, "what's new did not appear in the shelf")
	}

	return nil
}
