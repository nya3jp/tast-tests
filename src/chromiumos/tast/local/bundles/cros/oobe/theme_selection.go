// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ThemeSelection,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test Theme Selection screen during OOBE to support light/dark/auto themes",
		Contacts: []string{
			"bohdanty@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-oac@google.com",
			"cros-oobe@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.LoginTimeout + 3*time.Minute,
	})
}

// expectedTextColor executes JS code on the Theme Selection screen page to
// fetch current page header text color.
func expectedTextColor(ctx context.Context, oobeConn *chrome.Conn, darkMode bool) (string, error) {
	colorQuery := "getComputedStyle(document.documentElement).getPropertyValue('--cros-text-color-primary-%s').trim();"
	formattedQuery := fmt.Sprintf(colorQuery, "light")
	if darkMode {
		formattedQuery = fmt.Sprintf(colorQuery, "dark")
	}
	var colorData string
	if err := oobeConn.Eval(ctx, formattedQuery, &colorData); err != nil {
		return "", err
	}
	return colorData, nil
}

// nameOfSelectedTheme queries OobeApi to fetch selected value of a theme
// selection radio button. This call might return non-string value in case
// there is no selected value. We raise an error if it happens as we expect one
// of the theme modes to be always selected
func nameOfSelectedTheme(ctx context.Context, oobeConn *chrome.Conn) (string, error) {
	var currentThemeModeResponse interface{}
	if err := oobeConn.Eval(ctx, queryThemeScreen("getNameOfSelectedTheme()"), &currentThemeModeResponse); err != nil {
		return "", errors.Wrap(err, "failed to retrieve current theme mode")
	}

	currentThemeMode, ok := currentThemeModeResponse.(string)
	if !ok {
		return "", errors.New("can't convert theme mode call response to string. This could mean that there is no selection on the radio button")
	}
	return currentThemeMode, nil
}

// waitForColorUpdate polls Theme Selection screen until new color schema is
// applied to the UI. It should break polling and return an error if we fail to
// execute JS calls, otherwise it waits until expected colors are present on the
// screen.
func waitForColorUpdate(ctx context.Context, oobeConn *chrome.Conn, darkMode bool) error {
	var pollOpts = &testing.PollOptions{Interval: 50 * time.Millisecond, Timeout: 20 * time.Second}
	return testing.Poll(ctx, func(ctx context.Context) error {
		var headerTextColor string
		if err := oobeConn.Eval(ctx, queryThemeScreen("getHeaderTextColor()"), &headerTextColor); err != nil {
			// Break polling if we received any error from the OobeAPI.
			return testing.PollBreak(errors.Wrap(err, "failed to get color of the text header"))
		}

		expectedTextColor, err := expectedTextColor(ctx, oobeConn, darkMode)
		if err != nil {
			// Break polling if we failed to query JS on the screen.
			return testing.PollBreak(errors.Wrap(err, "failed to get color of the text header"))
		}

		// We try to unify color strings in the rgb(x,y,z) format by removing
		// whitespaces from them and then compare given values. JS calls might
		// return fetched properties with a different amount of whitespaces
		// inside them, so we want to mitigate this risk here.
		headerTextColor = strings.ReplaceAll(headerTextColor, " ", "")
		expectedTextColor = strings.ReplaceAll(expectedTextColor, " ", "")
		if headerTextColor != expectedTextColor {
			return errors.Errorf("header color (%s) doesn't match expected color (%s)", headerTextColor, expectedTextColor)
		}

		return nil
	}, pollOpts)
}

// queryThemeScreen returns correct OobeAPI path for a given function call.
func queryThemeScreen(call string) string {
	return fmt.Sprintf("OobeAPI.screens.ThemeSelectionScreen.%s", call)
}

func ThemeSelection(ctx context.Context, s *testing.State) {
	const (
		lightTheme = "light"
		darkTheme  = "dark"
		autoTheme  = "auto"
	)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.DontSkipOOBEAfterLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	if err := oobeConn.Eval(ctx, "OobeAPI.advanceToScreen('theme-selection')", nil); err != nil {
		s.Fatal("Failed to advance to the theme selection screen: ", err)
	}

	if err := oobeConn.WaitForExprFailOnErr(ctx, queryThemeScreen("isReadyForTesting()")); err != nil {
		s.Fatal("Failed to wait for the theme selection screen: ", err)
	}

	selectedTheme, err := nameOfSelectedTheme(ctx, oobeConn)
	if err != nil {
		s.Fatal("Failed to get name of a selected theme: ", err)
	}
	if selectedTheme != autoTheme {
		s.Fatal("Device should be in the auto theme mode by default")
	}

	// We can combine testing for light and dark modes as they do same steps.
	for _, theme := range []string{lightTheme, darkTheme} {
		selectFuncCall := "selectLightTheme()"
		if theme == darkTheme {
			selectFuncCall = "selectDarkTheme()"
		}

		if err := oobeConn.Eval(ctx, queryThemeScreen(selectFuncCall), nil); err != nil {
			s.Fatal(fmt.Sprintf("Failed to switch to the %s theme mode: ", theme), err)
		}

		selectedTheme, err := nameOfSelectedTheme(ctx, oobeConn)
		if err != nil {
			s.Fatal("Failed to get name of a selected theme: ", err)
		}
		if selectedTheme != theme {
			s.Fatalf("Device should be in the %s theme mode after we select it", theme)
		}

		if err = waitForColorUpdate(ctx, oobeConn, theme == darkTheme /*darkMode*/); err != nil {
			s.Fatal(fmt.Sprintf("Failed to wait until colors update to %s theme: ", theme), err)
		}
	}
}
