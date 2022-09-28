// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package ossettings supports controlling the Settings App on ChromeOS.
// This differs from Chrome settings (chrome://settings vs chrome://os-settings)
package ossettings

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/network/netconfig"
	"chromiumos/tast/testing"
)

const uiTimeout = 15 * time.Second

var defaultPollOpts = &testing.PollOptions{Timeout: 10 * time.Second, Interval: 1 * time.Second}

const urlPrefix = "chrome://os-settings/"

// OSSettings represents an instance of the Settings app.
type OSSettings struct {
	ui    *uiauto.Context
	tconn *chrome.TestConn
}

// New returns a new ossettings context.
// OSSettings can be launched from a page or app.
func New(tconn *chrome.TestConn) *OSSettings {
	return &OSSettings{ui: uiauto.New(tconn), tconn: tconn}
}

// Launch launches the Settings app.
// An error is returned if the app fails to launch.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*OSSettings, error) {
	app := apps.Settings
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		return nil, errors.Wrapf(err, "failed to launch %s", app.Name)
	}

	testing.ContextLog(ctx, "Waiting for settings app shown in shelf")
	if err := ash.WaitForApp(ctx, tconn, app.ID, time.Minute); err != nil {
		return nil, errors.Wrapf(err, "%s did not appear in shelf after launch", app.Name)
	}
	return &OSSettings{ui: uiauto.New(tconn), tconn: tconn}, nil
}

// Close closes the Settings app.
// This is automatically done when chrome resets and is not necessary to call.
func (s *OSSettings) Close(ctx context.Context) error {
	app := apps.Settings
	if err := apps.Close(ctx, s.tconn, app.ID); err != nil {
		return errors.Wrap(err, "failed to close settings app")
	}
	if err := ash.WaitForAppClosed(ctx, s.tconn, app.ID); err != nil {
		return errors.Wrap(err, "failed waiting for settings app to close")
	}
	return nil
}

// LaunchAtPage launches the Settings app at a particular page.
// An error is returned if the app fails to launch.
// TODO (b/189055966): Fix the failure to launch the right subpage.
func LaunchAtPage(ctx context.Context, tconn *chrome.TestConn, subpage *nodewith.Finder) (*OSSettings, error) {
	// Launch Settings App.
	s, err := Launch(ctx, tconn)
	if err != nil {
		return nil, err
	}

	// Wait until either the subpage or main menu exist.
	// On small screens the sidebar is collapsed, and the main menu must be clicked.
	subPageInApp := subpage.FinalAncestor(WindowFinder)
	menuButton := MenuButton.Ancestor(WindowFinder)
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := s.ui.Exists(subPageInApp)(ctx); err == nil {
			return nil
		}
		if err := s.ui.Exists(menuButton)(ctx); err == nil {
			return nil
		}
		return errors.New("neither subpage nor main menu exist")
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 30 * time.Second}); err != nil {
		return nil, err
	}

	// If the subpage doesn't exist, click the main menu.
	// Focus the subpage to ensure it is on-screen.
	// Then click the subpage that we want in the sidebar.
	if err := uiauto.Combine("click subpage",
		uiauto.IfSuccessThen(s.ui.Gone(subPageInApp), s.ui.LeftClick(menuButton)),
		s.ui.FocusAndWait(subPageInApp),
		s.ui.LeftClick(subPageInApp),
	)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to click subpage with %v", subpage)
	}
	return s, nil
}

// LaunchAtPageURL launches the Settings app at a particular page via changing URL in javascript.
// It uses a condition check to make sure the function completes correctly.
// It is high recommended to use UI validation in condition check.
func LaunchAtPageURL(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, pageShortURL string, condition func(context.Context) error) (*OSSettings, error) {
	// Launch Settings App.
	s, err := Launch(ctx, tconn)
	if err != nil {
		return nil, err
	}
	return s, s.NavigateToPageURL(ctx, cr, pageShortURL, condition)
}

// LaunchAtAppMgmtPage launches the Settings app at a particular app management page under app
// via changing URL in javascript.
// The URL includes an App ID.
// It calls LaunchAtPageURL.
func LaunchAtAppMgmtPage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, appID string, condition func(context.Context) error) (*OSSettings, error) {
	return LaunchAtPageURL(ctx, tconn, cr, fmt.Sprintf("app-management/detail?id=%s", appID), condition)
}

// OpenMobileDataSubpage navigates Settings app to mobile data subpage.
func OpenMobileDataSubpage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome) (*OSSettings, error) {
	ui := uiauto.New(tconn)

	if _, err := LaunchAtPageURL(ctx, tconn, cr, "Network", ui.Exists(networkFinder)); err != nil {
		return nil, errors.Wrap(err, "failed to launch settings page")
	}

	if err := uiauto.Combine("Go to mobile data page",
		ui.LeftClick(networkFinder),
		ui.LeftClick(mobileButton),
	)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to go to mobile data page")
	}
	return &OSSettings{tconn: tconn, ui: ui}, nil
}

// NavigateToPageURL navigates the Settings app to a particular page.
func (s *OSSettings) NavigateToPageURL(ctx context.Context, cr *chrome.Chrome, pageShortURL string, condition func(context.Context) error) error {
	settingsConn, err := s.ChromeConn(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to connect to OS settings target")
	}
	defer settingsConn.Close()

	return webutil.NavigateToURLInApp(settingsConn, urlPrefix+pageShortURL, condition, 20*time.Second)(ctx)
}

// LaunchHelpApp returns a function that launches Help app by clicking "Get help with ChromeOS".
func (s *OSSettings) LaunchHelpApp() uiauto.Action {
	return s.ui.LeftClick(nodewith.Name("Get help with ChromeOS").Role(role.Link).Ancestor(WindowFinder))
}

// LaunchWhatsNew returns a function that launches Help app by clicking "See what's new".
func (s *OSSettings) LaunchWhatsNew() uiauto.Action {
	return s.ui.LeftClick(nodewith.Name("See what's new").Role(role.Link).Ancestor(WindowFinder))
}

// ChromeConn returns a Chrome connection to the Settings app.
func (s *OSSettings) ChromeConn(ctx context.Context, cr *chrome.Chrome) (*chrome.Conn, error) {
	targetFilter := func(t *chrome.Target) bool { return strings.HasPrefix(t.URL, urlPrefix) }
	settingsConn, err := cr.NewConnForTarget(ctx, targetFilter)
	if err != nil {
		return nil, err
	}
	if err := chrome.AddTastLibrary(ctx, settingsConn); err != nil {
		settingsConn.Close()
		return nil, errors.Wrap(err, "failed to introduce tast library")
	}
	return settingsConn, nil
}

// AuthenticationToken represents an authentication token.
type AuthenticationToken struct {
	Token           string `json:"token"`
	LifetimeSeconds int    `json:"lifetimeSeconds"`
}

// AuthToken retrieves an authentication token that is needed to toggle some protected settings.
// It allows us to automatically change things that would require a user to type their password if done manually.
func (s *OSSettings) AuthToken(ctx context.Context, settingsConn *chrome.Conn, password string) (*AuthenticationToken, error) {
	// Wait for chrome.quickUnlockPrivate to be available.
	if err := settingsConn.WaitForExpr(ctx, `chrome.quickUnlockPrivate !== undefined`); err != nil {
		return nil, errors.Wrap(err, "failed waiting for chrome.quickUnlockPrivate to load")
	}

	// Wait for tast to be available.
	if err := settingsConn.WaitForExpr(ctx, `tast !== undefined`); err != nil {
		return nil, errors.Wrap(err, "failed waiting for tast to load")
	}

	var token AuthenticationToken
	if err := settingsConn.Call(ctx, &token,
		`password => tast.promisify(chrome.quickUnlockPrivate.getAuthToken)(password)`, password,
	); err != nil {
		return nil, errors.Wrap(err, "failed to get auth token")
	}
	return &token, nil
}

// EnablePINUnlock returns a function that enables unlocking the device with the specified PIN.
func (s *OSSettings) EnablePINUnlock(cr *chrome.Chrome, password, PIN string, autosubmit bool) uiauto.Action {
	return func(ctx context.Context) error {
		settingsConn, err := s.ChromeConn(ctx, cr)
		if err != nil {
			return errors.Wrap(err, "failed to connect to OS settings target")
		}
		token, err := s.AuthToken(ctx, settingsConn, password)
		if err != nil {
			return errors.Wrap(err, "failed to get auth token")
		}
		if err := settingsConn.Call(ctx, nil,
			`(token, PIN) => tast.promisify(chrome.quickUnlockPrivate.setModes)(token, [chrome.quickUnlockPrivate.QuickUnlockMode.PIN], [PIN])`, token.Token, PIN,
		); err != nil {
			return errors.Wrap(err, "failed to get auth token")
		}

		if err := settingsConn.Call(ctx, nil,
			`tast.promisify(chrome.quickUnlockPrivate.setPinAutosubmitEnabled)`, token.Token, PIN, autosubmit,
		); err != nil {
			return errors.Wrap(err, "failed to get auth token")
		}
		return nil
	}
}

// WaitForSearchBox returns a function that waits for the search box to appear.
// Useful for checking that some content has loaded and Settings is ready to use.
func (s *OSSettings) WaitForSearchBox() uiauto.Action {
	return s.ui.WaitUntilExists(SearchBoxFinder)
}

// EvalJSWithShadowPiercer executes javascript in Settings app web page.
func (s *OSSettings) EvalJSWithShadowPiercer(ctx context.Context, cr *chrome.Chrome, expr string, out interface{}) error {
	conn, err := s.ChromeConn(ctx, cr)
	if err != nil {
		return errors.Wrap(err, "failed to connect to Settings web page")
	}
	defer conn.Close()
	return webutil.EvalWithShadowPiercer(ctx, conn, expr, out)
}

// SetToggleOption clicks toggle option to enable or disable an option.
// It does nothing if the option is already expected.
func (s *OSSettings) SetToggleOption(cr *chrome.Chrome, optionName string, expected bool) uiauto.Action {
	return func(ctx context.Context) error {
		if isEnabled, err := s.IsToggleOptionEnabled(ctx, cr, optionName); err != nil {
			return err
		} else if isEnabled == expected {
			return nil
		}
		optionFinder := nodewith.Name(optionName).Role(role.ToggleButton)
		return uiauto.Combine("set toggle option",
			s.ui.WaitUntilEnabled(optionFinder),
			s.LeftClick(optionFinder),
			s.WaitUntilToggleOption(cr, optionName, expected),
		)(ctx)
	}
}

// SetDropDownOption sets dropdown option to a value.
func (s *OSSettings) SetDropDownOption(cr *chrome.Chrome, optionName, expected string) uiauto.Action {
	optionFinder := nodewith.Name(optionName).Role(role.ComboBoxSelect)
	// TODO(crbug/1364495): remove old finder once crrev.com/c/3868204 upreved.
	oldOptionFinder := nodewith.Name(optionName).Role(role.PopUpButton)
	settingFinder := nodewith.Name(expected).Role(role.ListBoxOption)
	return uiauto.Combine("set drop down option",
		uiauto.IfFailThen(
			s.LeftClick(oldOptionFinder),
			s.LeftClick(optionFinder),
		),
		s.LeftClick(settingFinder),
		uiauto.Sleep(time.Second),
	)
}

// IsToggleOptionEnabled checks whether the toggle option is enabled or not.
func (s *OSSettings) IsToggleOptionEnabled(ctx context.Context, cr *chrome.Chrome, optionName string) (bool, error) {
	toggleButtonCSSSelector := fmt.Sprintf(`cr-toggle[aria-label=%q]`, optionName)
	expr := fmt.Sprintf(`
		var optionNode = shadowPiercingQuery(%q);
		if(optionNode == undefined){
			throw new Error("%s setting item is not found.");
		}
		optionNode.getAttribute("aria-pressed")==="true";
		`, toggleButtonCSSSelector, optionName)

	var isEnabled bool
	if err := s.EvalJSWithShadowPiercer(ctx, cr, expr, &isEnabled); err != nil {
		return isEnabled, errors.Wrapf(err, "failed to get status of option: %q", optionName)
	}
	return isEnabled, nil
}

// DropdownValue returns the value of a dropdown setting.
func (s *OSSettings) DropdownValue(ctx context.Context, cr *chrome.Chrome, dropdownName string) (string, error) {
	dropdownElementSelector := fmt.Sprintf(`select[aria-label=%q]`, dropdownName)
	expr := fmt.Sprintf(`
		var optionNode = shadowPiercingQuery(%q);
		if(optionNode == undefined){
			throw new Error("%s dropdown setting is not found.");
		}
		optionNode.value;
		`, dropdownElementSelector, dropdownName)

	var value string
	if err := s.EvalJSWithShadowPiercer(ctx, cr, expr, &value); err != nil {
		return value, errors.Wrapf(err, "failed to get the value of dropdown: %q", dropdownName)
	}
	return value, nil
}

// WaitUntilToggleOption returns an action to wait until the toggle option enabled or disabled.
func (s *OSSettings) WaitUntilToggleOption(cr *chrome.Chrome, optionName string, expected bool) uiauto.Action {
	return func(ctx context.Context) error {
		return testing.Poll(ctx, func(ctx context.Context) error {
			if isEnabled, err := s.IsToggleOptionEnabled(ctx, cr, optionName); err != nil {
				// JS evaluation is not always reliable. So do not break if failed.
				return err
			} else if isEnabled != expected {
				return errors.Errorf("Option %q is unpected: got %v; want %v", optionName, isEnabled, expected)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second})
	}
}

// OpenNetworkDetailPage navigates to the detail page for a particular Cellular or WiFi network.
func OpenNetworkDetailPage(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, networkName string, networkType netconfig.NetworkType) (*OSSettings, error) {
	ui := uiauto.New(tconn)

	app, err := Launch(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch settings page")
	}

	subpageArrowFinder := nodewith.Role(role.Button).ClassName("subpage-arrow")
	subpageArrowFinderWithName, err := func() (*nodewith.Finder, error) {
		var technology string
		if networkType == netconfig.Cellular {
			technology = "Mobile data"
		} else if networkType == netconfig.WiFi {
			technology = "Wi-Fi"
		} else {
			return nil, errors.New("Network technology must be Cellular or WiFi")
		}
		// We append " enable" here since SetToggleOption requires the exact toggle name.
		if err := app.SetToggleOption(cr, technology+" enable", true)(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to enable network technology: "+technology)
		}
		return subpageArrowFinder.NameContaining(technology), nil
	}()

	if err != nil {
		return nil, errors.Wrap(err, "failed to determine network subpage finder")
	}

	if err = ui.LeftClick(subpageArrowFinderWithName)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to navigate to the network subpage")
	}

	networkDetailPageFinder := subpageArrowFinder.NameContaining(networkName).First()
	if err = ui.LeftClick(networkDetailPageFinder)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to open detail page")
	}

	return &OSSettings{tconn: tconn, ui: ui}, nil
}

// SearchWithKeyword searches the demand keyword by input text in the `SearchBox`.
func (s *OSSettings) SearchWithKeyword(ctx context.Context, kb *input.KeyboardEventWriter,
	keyword string) (results []uiauto.NodeInfo, mismatched bool, err error) {

	if err := uiauto.Combine(fmt.Sprintf("query with keywords %q", keyword),
		kb.TypeAction(keyword),
		s.WaitUntilExists(nodewith.HasClass("ContentsWebView").Focused()),
		// WaitUntilExists returns once the node is found, while WaitForLocation waits
		// until the node exists and the location is not changing for two iterations of polling.
		// In this case, the node will show the previous result first, then hide and reappear with the new result,
		// so use WaitForLocation to wait until it stabilizes.
		s.WaitForLocation(searchResultFinder.First()),
	)(ctx); err != nil {
		return nil, false, err
	}

	results, err = s.NodesInfo(ctx, searchResultFinder)
	if len(results) <= 0 {
		return nil, false, errors.New("no search result found")
	} else if regexp.MustCompile(searchMismatched).MatchString(results[0].Name) {
		mismatched = true
	}
	return results, mismatched, err
}

// ClearSearch clears text in `SearchBox` and waits for the search results to be gone.
func (s *OSSettings) ClearSearch() uiauto.Action {
	clearSearchBtn := nodewith.NameContaining("Clear search").Role(role.Button)
	return uiauto.Combine("clear text in search box",
		uiauto.IfSuccessThen(s.ui.WaitUntilExists(clearSearchBtn), s.LeftClick(clearSearchBtn)),
		s.WaitUntilGone(clearSearchBtn),
		s.WaitUntilGone(searchResultFinder),
	)
}

// UninstallApp uninstalls an app from the Settings app.
// It first opens the Settings app on the apps page.
// If it fails to open the page, it thinks the app is not there so no need to uninstall and returns nil.
// Then it clicks the Uninstall button to uninstall.
func UninstallApp(ctx context.Context, tconn *chrome.TestConn, cr *chrome.Chrome, appName, appID string) error {
	ui := uiauto.New(tconn)
	appNode := nodewith.Name(appName).ClassName("cr-title-text")
	if _, err := LaunchAtPageURL(ctx, tconn, cr, "app-management/detail?id="+appID, ui.WaitUntilExists(appNode)); err != nil {
		testing.ContextLogf(ctx, "Failed to open Settings app at the app %s, it may not exist: %s", appName, err)
		return nil
	}

	uninstall := nodewith.Name("Uninstall").Role(role.Button)
	uninstallWindow := nodewith.NameStartingWith("Uninstall").Role(role.Window)

	return uiauto.Combine("uninstall the app",
		ui.LeftClick(uninstall),
		ui.WaitUntilExists(uninstallWindow),
		ui.LeftClick(uninstall.Ancestor(uninstallWindow)),
		ui.WaitUntilGone(uninstallWindow))(ctx)
}

// CommonSections returns a map that contains *nodewith.Finder for OS-Settings UI elements of common sections.
func CommonSections(advanceExpanded bool) map[string]*nodewith.Finder {
	sections := map[string]*nodewith.Finder{
		"Network":              Network,
		"Bluetooth":            Bluetooth,
		"Connected Devices":    ConnectedDevices,
		"Accounts":             Accounts,
		"Device":               Device,
		"Personalization":      Personalization,
		"Security And Privacy": SecurityAndPrivacy,
		"Apps":                 Apps,
		"About ChromeOS":       AboutChromeOS,
	}

	if advanceExpanded {
		sections["Date And Time"] = DateAndTime
		sections["Languages And Inputs"] = LanguagesAndInputs
		sections["Files"] = Files
		sections["Print And Scan"] = PrintAndScan
		sections["Developers"] = Developers
		sections["Accessibility"] = Accessibility
		sections["Reset Settings"] = ResetSettings
	}

	return sections
}
