// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"fmt"
	"time"

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

// proxyRetainTestParam is a parameter for ProxyRetain test.
type proxyRetainTestParam struct {
	description string
	test        proxyRetainTest
	loginUsers  []chrome.Option
}

// Define account information for primary user and secondary user.
var (
	primaryUser   chrome.Creds = chrome.Creds{User: "testuser1@gmail.com", Pass: "123456"}
	secondaryUser chrome.Creds = chrome.Creds{User: "testuser2@gmail.com", Pass: "123456"}
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProxyRetain,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies that the proxy settings will be retained after login or across different users",
		Contacts: []string{
			"lance.wang@cienet.com",
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
		Fixture:      "shillReset",
		Timeout:      5 * time.Minute,
		Params: []testing.Param{
			{
				Name: "after_login",
				Val: &proxyRetainTestParam{
					description: "Proxy information retained after login",
					test:        &retainAfterLoginTest{},
					loginUsers: []chrome.Option{
						chrome.FakeLogin(primaryUser),
					},
				},
			}, {
				Name: "across_users",
				Val: &proxyRetainTestParam{
					description: "Proxy information retained after logging to different types of users",
					test:        &retainAcrossUsersTest{},
					loginUsers: []chrome.Option{
						chrome.FakeLogin(primaryUser),
						chrome.FakeLogin(secondaryUser),
						chrome.GuestLogin(),
					},
				},
			},
		},
	})
}

// proxyRetainResource holds resources for ProxyRetain test.
type proxyRetainResource struct {
	tconn       *chrome.TestConn
	ui          *uiauto.Context
	kb          *input.KeyboardEventWriter
	outDir      string
	manifestKey string
}

// proxyValues defines the proxy UI text box field and its value.
type proxyValues map[string]struct {
	// node is the proxy UI node on the proxy section. (e.g., "nodewith.Name("HTTP Proxy - Host").Role(role.TextField)")
	node *nodewith.Finder
	// value is the expected value of the proxy UI text box field. (e.g., "localhost")
	value string
}

// ProxyRetain verifies proxy settings will be retained.
func ProxyRetain(ctx context.Context, s *testing.State) {
	param := s.Param().(*proxyRetainTestParam)
	// proxyValues holds proxy hosts and ports for http, https and socks.
	proxyValues := proxyValues{
		"http host":  {ossettings.HTTPHostTextField, "localhost"},
		"http port":  {ossettings.HTTPPortTextField, "123"},
		"https host": {ossettings.HTTPSHostTextField, "localhost"},
		"https port": {ossettings.HTTPSPortTextField, "456"},
		"socks host": {ossettings.SocksHostTextField, "socks5://localhost"},
		"socks port": {ossettings.SocksPortTextField, "8080"},
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer kb.Close()

	resources := &proxyRetainResource{
		kb:     kb,
		outDir: s.OutDir(),
	}

	if _, ok := param.test.(*retainAfterLoginTest); ok {
		// The retainAfterLoginTest test requires the manifest key to do operations without login.
		resources.manifestKey = s.RequiredVar("ui.signinProfileTestExtensionManifestKey")
	}

	s.Log("Creating user pod by signing in and out")
	cr, err := startChrome(ctx, resources, false /* isNoLogin */, false /* isKeepState */, chrome.FakeLogin(primaryUser))
	if err != nil {
		s.Fatal("Failed to create user pod: ", err)
	}

	if err := cr.Close(ctx); err != nil {
		s.Fatal("Failed to close Chrome instance: ", err)
	}

	if err := param.test.preparationAtLoginScreen(ctx, resources, proxyValues); err != nil {
		s.Fatalf("Failed to set proxy at login screen for test %q: %v", param.description, err)
	}

	if err := param.test.preparationAfterLoggedIn(ctx, resources, proxyValues); err != nil {
		s.Fatalf("Failed to set proxy after logged in for test %q: %v", param.description, err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	s.Logf("Looping proxy verification for test %q", param.description)
	for _, loginOpt := range param.loginUsers {
		func() {
			if cr, err = startChrome(ctx, resources, false /* isNoLogin */, true /* isKeepState */, loginOpt); err != nil {
				s.Fatal("Failed to sign in: ", err)
			}
			defer cr.Close(cleanupCtx)

			if err := launchProxySection(ctx, resources); err != nil {
				s.Fatal("Failed to launch proxy section: ", err)
			}
			defer apps.Close(cleanupCtx, resources.tconn, apps.Settings.ID)
			defer faillog.DumpUITreeOnErrorToFile(cleanupCtx, s.OutDir(), s.HasError, resources.tconn, "before_close_ossettings_ui_dump")

			if err := expandProxyOption(ctx, resources); err != nil {
				s.Fatal("Failed to expand proxy option: ", err)
			}

			if err := verifyProxy(ctx, resources.ui, proxyValues); err != nil {
				s.Fatal("Failed to complete test: ", err)
			}
		}()
	}
}

// startChrome starts the Chrome with specified configurations and returns the Chrome instance.
// It also reestablish other resources associated with the chrome.Chrome instance.
func startChrome(ctx context.Context, res *proxyRetainResource, isNoLogin, isKeepState bool, loginOpt ...chrome.Option) (cr *chrome.Chrome, retErr error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	opts := loginOpt
	if isNoLogin {
		opts = append(opts,
			chrome.NoLogin(),
			chrome.LoadSigninProfileExtension(res.manifestKey),
		)
	}

	if isKeepState {
		opts = append(opts, chrome.KeepState())
	}

	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome")
	}
	defer func(ctx context.Context) {
		if retErr != nil {
			if err := cr.Close(ctx); err != nil {
				testing.ContextLog(ctx, "Failed to close Chrome: ", err)
			}
		}
	}(cleanupCtx)

	getTconn := cr.TestAPIConn
	if isNoLogin {
		getTconn = cr.SigninProfileTestAPIConn
	}

	tconn, err := getTconn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Test API connection")
	}
	res.tconn = tconn
	res.ui = uiauto.New(tconn)

	return cr, nil
}

// launchProxySection launches a proxy section via Quick Settings.
// The Quick Settings can launch the current network setup page directly,
// without knowing which wifi and / or ethernet is currently connecting, therefore, the OS Settings wasn't used.
func launchProxySection(ctx context.Context, res *proxyRetainResource) (retErr error) {
	if err := quicksettings.NavigateToNetworkDetailedView(ctx, res.tconn, false); err != nil {
		return errors.Wrap(err, "failed to navigate to network detailed view")
	}

	if err := quicksettings.OpenNetworkSettings(ctx, res.tconn, false); err != nil {
		return errors.Wrap(err, "failed to open network settings")
	}
	return nil
}

// setupProxy sets up proxy values.
func setupProxy(ctx context.Context, res *proxyRetainResource, pv proxyValues) (retErr error) {
	if err := uiauto.Combine("setup proxy to 'Manual proxy configuration'",
		res.ui.LeftClickUntil(ossettings.ProxyDropDownMenu, res.ui.Exists(ossettings.ManualProxyOption)),
		res.ui.LeftClick(ossettings.ManualProxyOption),
		res.ui.WaitUntilGone(ossettings.ManualProxyOption),
	)(ctx); err != nil {
		return err
	}

	for fieldName, content := range pv {
		testing.ContextLogf(ctx, "Setting proxy value %q to field %q", content.value, fieldName)
		if err := uiauto.Combine(fmt.Sprintf("replace and type text %q to field %q", content.value, fieldName),
			res.ui.EnsureFocused(content.node),
			res.kb.AccelAction("ctrl+a"),
			res.kb.AccelAction("backspace"),
			res.kb.TypeAction(content.value),
		)(ctx); err != nil {
			return err
		}
	}

	saveButton := ossettings.WindowFinder.HasClass("action-button").Name("Save").Role(role.Button)
	return uiauto.Combine("save proxy settings",
		// Save changes.
		res.ui.MakeVisible(saveButton),
		res.ui.LeftClick(saveButton),
	)(ctx)
}

// expandProxyOption expands the proxy option on Settings.
// Settings app must be launched and direct to internet page in advance.
func expandProxyOption(ctx context.Context, res *proxyRetainResource) error {
	if err := res.ui.WaitUntilExists(ossettings.ShowProxySettingsTab)(ctx); err != nil {
		return errors.Wrap(err, "failed to find 'Shared networks' toggle button")
	}

	if err := uiauto.Combine("expand 'Proxy' section",
		res.ui.LeftClick(ossettings.ShowProxySettingsTab),
		res.ui.WaitForLocation(ossettings.SharedNetworksToggleButton),
	)(ctx); err != nil {
		return err
	}

	if toggleInfo, err := res.ui.Info(ctx, ossettings.SharedNetworksToggleButton); err != nil {
		return errors.Wrap(err, "failed to get toggle button info")
	} else if toggleInfo.Checked == checked.True {
		testing.ContextLog(ctx, "'Allow proxies for shared networks' is already turned on")
		return nil
	}

	return uiauto.Combine("turn on 'Allow proxies for shared networks' option",
		res.ui.LeftClick(ossettings.SharedNetworksToggleButton),
		res.ui.LeftClick(ossettings.ConfirmButton),
	)(ctx)
}

// verifyProxy verifies proxy values.
func verifyProxy(ctx context.Context, ui *uiauto.Context, pv proxyValues) (retErr error) {
	for fieldName, content := range pv {
		testing.ContextLogf(ctx, "Verify if the value of the field %q is %q", fieldName, content.value)
		if err := ui.EnsureFocused(content.node)(ctx); err != nil {
			return errors.Wrap(err, "failed to ensure node exists and is shown on the screen")
		}

		if info, err := ui.Info(ctx, content.node); err != nil {
			return errors.Wrap(err, "failed to get node info")
		} else if info.Value != content.value {
			return errors.Errorf("expected value %q for field %q, but got %q", content.value, fieldName, info.Value)
		}
	}
	return nil
}

type proxyRetainTest interface {
	// preparationAtLoginScreen prepares the test environment at the login screen.
	preparationAtLoginScreen(context.Context, *proxyRetainResource, proxyValues) error

	// preparationAfterLoggedIn prepares the test environment after logged in.
	preparationAfterLoggedIn(context.Context, *proxyRetainResource, proxyValues) error
}

// retainAfterLoginTest is a test case structure for the proxy retain after login.
type retainAfterLoginTest struct{}

func (t *retainAfterLoginTest) preparationAtLoginScreen(ctx context.Context, res *proxyRetainResource, pv proxyValues) (retErr error) {
	testing.ContextLog(ctx, "Setting up proxy at login screen")

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := startChrome(ctx, res, true /* isNoLogin */, true /* isKeepState */, chrome.FakeLogin(primaryUser))
	if err != nil {
		return errors.Wrap(err, "failed to sign in")
	}
	defer cr.Close(cleanupCtx)

	if err := launchProxySection(ctx, res); err != nil {
		return errors.Wrap(err, "failed to launch proxy section")
	}
	defer apps.Close(cleanupCtx, res.tconn, apps.Settings.ID)
	defer faillog.DumpUITreeOnErrorToFile(cleanupCtx, res.outDir, func() bool { return retErr != nil }, res.tconn, "before_close_ossettings_ui_dump")

	if err := setupProxy(ctx, res, pv); err != nil {
		return errors.Wrap(err, "failed to setup proxy")
	}
	return nil
}

func (t *retainAfterLoginTest) preparationAfterLoggedIn(ctx context.Context, res *proxyRetainResource, pv proxyValues) error {
	return nil
}

// retainAcrossUsersTest is a test case structure for the proxy retain across users.
type retainAcrossUsersTest struct{}

func (t *retainAcrossUsersTest) preparationAtLoginScreen(ctx context.Context, res *proxyRetainResource, pv proxyValues) error {
	return nil
}

func (t *retainAcrossUsersTest) preparationAfterLoggedIn(ctx context.Context, res *proxyRetainResource, pv proxyValues) (retErr error) {
	testing.ContextLog(ctx, "Setting up proxy after logged in")

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := startChrome(ctx, res, false /* isNoLogin */, true /* isKeepState */, chrome.FakeLogin(primaryUser))
	if err != nil {
		return errors.Wrap(err, "failed to sign in")
	}
	defer cr.Close(cleanupCtx)

	if err := launchProxySection(ctx, res); err != nil {
		return errors.Wrap(err, "failed to launch proxy section")
	}
	defer apps.Close(cleanupCtx, res.tconn, apps.Settings.ID)
	defer faillog.DumpUITreeOnErrorToFile(cleanupCtx, res.outDir, func() bool { return retErr != nil }, res.tconn, "before_close_ossettings_ui_dump")

	if err := expandProxyOption(ctx, res); err != nil {
		return errors.Wrap(err, "failed to expand proxy option")
	}

	if err := setupProxy(ctx, res, pv); err != nil {
		return errors.Wrap(err, "failed to setup proxy")
	}
	return nil
}
