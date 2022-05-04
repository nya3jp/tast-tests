// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/action"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProxyRetainedAfterLogin,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that the proxy settings will be retained after login",
		Contacts: []string{
			"lance.wang@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Fixture:      "shillReset",
	})
}

// ProxyRetainedAfterLogin verifies proxy settings will be retained after login.
func ProxyRetainedAfterLogin(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	signInProfileKey := s.RequiredVar("ui.signinProfileTestExtensionManifestKey")

	user := newProxyRetainedUser(s.OutDir())

	s.Log("Login to DUT first in order to create login screen")
	if err := user.performLogin(ctx, signInProfileKey, true /* isLoggedIn */, false /* keepState */); err != nil {
		s.Fatal("Failed to login to DUT: ", err)
	}
	defer user.cr.Close(cleanupCtx)

	s.Log("Back to login screen")
	if err := user.performLogin(ctx, signInProfileKey, false /* isLoggedIn */, true /* keepState */); err != nil {
		s.Fatal("Failed to enter to login screen: ", err)
	}
	defer user.cr.Close(cleanupCtx)

	if err := user.launchProxySection(ctx); err != nil {
		s.Fatal("Failed to launch proxy section in login screen: ", err)
	}
	defer faillog.DumpUITreeOnErrorToFile(cleanupCtx, s.OutDir(), s.HasError, user.tconn, "dump_before_login")

	if err := user.setupProxy(ctx); err != nil {
		s.Fatal("Failed to setup proxy in login screen: ", err)
	}

	s.Log("Login user to DUT")
	if err := user.performLogin(ctx, signInProfileKey, true /* isLoggedIn */, true /* keepState */); err != nil {
		s.Fatal("Failed to login to DUT: ", err)
	}
	defer user.cr.Close(cleanupCtx)

	if err := user.launchProxySection(ctx); err != nil {
		s.Fatal("Failed to launch proxy section: ", err)
	}
	defer apps.Close(cleanupCtx, user.tconn, apps.Settings.ID)
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, user.cr, "dump_after_login")

	s.Log("Verify proxy values")
	if err := user.verifyProxyValues(ctx); err != nil {
		s.Fatal("Failed to verify proxy values: ", err)
	}
}

// proxyRetained holds all necessary components as a Chrome user instance for test case ProxyRetainedAfterLogin.
type proxyRetainedUser struct {
	/* Basic components */
	isLoggedIn     bool
	cr             *chrome.Chrome
	tconn          *chrome.TestConn
	ui             *uiauto.Context
	ourDir         string
	windowAncestor *nodewith.Finder
	/* Proxy values */
	httpProxy  string
	httpPort   string
	httpsProxy string
	httpsPort  string
	socksProxy string
	socksPort  string
}

// newProxyRetainedUser returns a new Chrome user instance with required proxy values.
// User has not been initialized yet.
func newProxyRetainedUser(outDir string) *proxyRetainedUser {
	return &proxyRetainedUser{
		ourDir:     outDir,
		httpProxy:  "localhost",
		httpPort:   "123",
		httpsProxy: "https://localhost",
		httpsPort:  "456",
		socksProxy: "socks5://localhost",
		socksPort:  "8080",
	}
}

// performLogin perform login operations to desktop or login screen.
func (p *proxyRetainedUser) performLogin(ctx context.Context, profileKey string, isLoggedIn, keepState bool) error {
	p.isLoggedIn = isLoggedIn

	var options []chrome.Option
	if !p.isLoggedIn {
		options = append(options, chrome.LoadSigninProfileExtension(profileKey), chrome.NoLogin())
	}

	if keepState {
		options = append(options, chrome.KeepState())
	}

	cr, err := chrome.New(ctx, options...)
	if err != nil {
		return errors.Wrap(err, "failed to start Chrome")
	}
	p.cr = cr

	var tconn *chrome.TestConn
	if !p.isLoggedIn {
		p.windowAncestor = nodewith.HasClass("BubbleFrameView").Role(role.Client)
		tconn, err = p.cr.SigninProfileTestAPIConn(ctx)
	} else {
		p.windowAncestor = ossettings.WindowFinder
		tconn, err = p.cr.TestAPIConn(ctx)
	}
	if err != nil {
		return errors.Wrap(err, "failed to connect to test API")
	}
	p.tconn = tconn

	p.ui = uiauto.New(p.tconn)
	return nil
}

// launchProxySection launches a proxy section via Quick Settings.
// This function also turns on "Allow proxies for shared networks" option if the user logged in to DUT.
func (p *proxyRetainedUser) launchProxySection(ctx context.Context) (retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	if err := quicksettings.NavigateToNetworkDetailedView(ctx, p.tconn); err != nil {
		return errors.Wrap(err, "failed to navigate to network detailed view")
	}

	if err := quicksettings.OpenNetworkSettings(ctx, p.tconn); err != nil {
		return errors.Wrap(err, "failed to open network settings")
	}
	defer func(ctx context.Context) {
		if retErr != nil && p.isLoggedIn {
			if err := apps.Close(ctx, p.tconn, apps.Settings.ID); err != nil {
				testing.ContextLog(ctx, "Failed to close settings: ", err)
			}
		}
	}(cleanupCtx)

	// If the user login to the DUT, settings app should be opened and navigate to internet section.
	if p.isLoggedIn {
		showProxySettingsTab := nodewith.HasClass("settings-box").Name("Show proxy settings").Role(role.GenericContainer).Ancestor(p.windowAncestor)
		if err := p.ui.WaitUntilExists(showProxySettingsTab)(ctx); err != nil {
			return errors.Wrap(err, "failed to find 'Shared networks' toggle button")
		}

		sharedNetworksToggleButton := nodewith.Name("Allow proxies for shared networks").Role(role.ToggleButton).Ancestor(p.windowAncestor)
		if err := uiauto.Combine("expand 'Proxy' section",
			p.ui.LeftClick(showProxySettingsTab),
			p.ui.WaitForLocation(sharedNetworksToggleButton),
		)(ctx); err != nil {
			return err
		}

		if toggleInfo, err := p.ui.Info(ctx, sharedNetworksToggleButton); err != nil {
			return errors.Wrap(err, "failed to get toggle button info")
		} else if toggleInfo.Checked == checked.True {
			testing.ContextLog(ctx, "'Allow proxies for shared networks' is already turned on")
			return nil
		}

		confirmButton := nodewith.Name("Confirm").Role(role.Button).Ancestor(p.windowAncestor)
		if err := uiauto.Combine("turn on 'Allow proxies for shared networks' option",
			p.ui.LeftClick(sharedNetworksToggleButton),
			p.ui.LeftClick(confirmButton),
		)(ctx); err != nil {
			return err
		}
	}

	return nil
}

// setupProxy sets up proxy values.
func (p *proxyRetainedUser) setupProxy(ctx context.Context) (retErr error) {
	proxyDropDownMenu := nodewith.HasClass("md-select").NameContaining(" type").Role(role.PopUpButton).Ancestor(p.windowAncestor)
	proxyOption := nodewith.Role(role.ListBoxOption).Name("Manual proxy configuration").Ancestor(p.windowAncestor)
	if err := uiauto.Combine("setup proxy to 'Manual proxy configuration'",
		p.ui.LeftClickUntil(proxyDropDownMenu, p.ui.Exists(proxyOption)),
		p.ui.LeftClick(proxyOption),
		p.ui.WaitUntilGone(proxyOption),
	)(ctx); err != nil {
		return err
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get keyboard")
	}
	defer kb.Close()

	// replaceAndTypeText returns an action that focuses, deletes old content and types specified value to given input node.
	replaceAndTypeText := func(node *nodewith.Finder, value string) action.Action {
		return uiauto.Combine(fmt.Sprintf("replace and type text %q", value),
			p.ui.EnsureFocused(node),
			kb.AccelAction("ctrl+a"),
			kb.AccelAction("backspace"),
			kb.TypeAction(value),
		)
	}

	testing.ContextLog(ctx, "Start setting proxy values")
	textField := nodewith.Role(role.TextField).Ancestor(p.windowAncestor)
	saveButton := nodewith.HasClass("action-button").Name("Save").Role(role.Button).Ancestor(p.windowAncestor)
	closeButton := nodewith.HasClass("ImageButton").Name("Close").Role(role.Button).Ancestor(p.windowAncestor)

	return uiauto.Combine("setup proxy settings",
		// Type hosts and ports.
		replaceAndTypeText(textField.Name("HTTP Proxy - Host"), p.httpProxy),
		replaceAndTypeText(textField.Name("HTTP Proxy - Port"), p.httpPort),
		replaceAndTypeText(textField.Name("Secure HTTP Proxy - Host"), p.httpsProxy),
		replaceAndTypeText(textField.Name("Secure HTTP Proxy - Port"), p.httpsPort),
		replaceAndTypeText(textField.Name("SOCKS Host - Host"), p.socksProxy),
		replaceAndTypeText(textField.Name("SOCKS Host - Port"), p.socksPort),
		// Save changes.
		p.ui.MakeVisible(saveButton),
		p.ui.LeftClick(saveButton),
		// Close the window.
		p.ui.LeftClick(closeButton),
	)(ctx)
}

func (p *proxyRetainedUser) verifyProxyValues(ctx context.Context) error {
	// checkTextfieldValue checks if given node has specified value.
	checkTextfieldValue := func(node *nodewith.Finder, value string) error {
		if err := uiauto.Combine("ensure node exists and is shown on the screen",
			p.ui.WaitUntilExists(node),
			p.ui.MakeVisible(node),
		)(ctx); err != nil {
			return err
		}
		info, err := p.ui.Info(ctx, node)
		if err != nil {
			return errors.Wrap(err, "failed to get node info")
		}
		if info.Value != value {
			return errors.Errorf("expected value %q, got %q", value, info.Value)
		}
		return nil
	}

	testing.ContextLog(ctx, "Verify proxy values")
	textField := nodewith.Role(role.TextField).Ancestor(p.windowAncestor)
	for node, value := range map[*nodewith.Finder]string{
		textField.Name("HTTP Proxy - Host"):        p.httpProxy,
		textField.Name("HTTP Proxy - Port"):        p.httpPort,
		textField.Name("Secure HTTP Proxy - Host"): p.httpsProxy,
		textField.Name("Secure HTTP Proxy - Port"): p.httpsPort,
		textField.Name("SOCKS Host - Host"):        p.socksProxy,
		textField.Name("SOCKS Host - Port"):        p.socksPort,
	} {
		if err := checkTextfieldValue(node, value); err != nil {
			return errors.Wrap(err, "failed to verify proxy value")
		}
	}
	return nil
}
