// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package network

import (
	"context"
	"reflect"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/network/proxysettings"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
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

// ProxyRetain verifies proxy settings will be retained.
func ProxyRetain(ctx context.Context, s *testing.State) {
	param := s.Param().(*proxyRetainTestParam)

	// proxyValues holds proxy hosts and ports for http, https and socks.
	proxyValues := []*proxysettings.Config{
		{
			Protocol: proxysettings.HTTP,
			Host:     "localhost",
			Port:     "123",
		},
		{
			Protocol: proxysettings.HTTPS,
			Host:     "localhost",
			Port:     "456",
		},
		{
			Protocol: proxysettings.Socks,
			Host:     "socks5://localhost",
			Port:     "8080",
		},
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

	if err := param.test.preparationAtLoginScreen(ctx, cr, resources, proxyValues); err != nil {
		s.Fatalf("Failed to set proxy at login screen for test %q: %v", param.description, err)
	}

	if err := param.test.preparationAfterLoggedIn(ctx, cr, resources, proxyValues); err != nil {
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

			ps, err := proxysettings.Collect(ctx, resources.tconn)
			if err != nil {
				s.Fatal("Failed to launch proxy settings instance: ", err)
			}
			defer ps.Close(cleanupCtx, kb)
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "before_close_ossettings_ui_dump")

			// Verify proxy values.
			for _, pv := range proxyValues {
				if resultPv, err := ps.Content(ctx, pv); err != nil {
					s.Fatalf("Failed to get proxy value for %q: %v", pv.HostName(), err)
				} else if !reflect.DeepEqual(resultPv, pv) {
					s.Fatalf("Failed to verify proxy value for %q: got %q, want %q", pv.HostName(), resultPv, pv)
				}
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

type proxyRetainTest interface {
	// preparationAtLoginScreen prepares the test environment at the login screen.
	preparationAtLoginScreen(context.Context, *chrome.Chrome, *proxyRetainResource, []*proxysettings.Config) error

	// preparationAfterLoggedIn prepares the test environment after logged in.
	preparationAfterLoggedIn(context.Context, *chrome.Chrome, *proxyRetainResource, []*proxysettings.Config) error
}

// retainAfterLoginTest is a test case structure for the proxy retain after login.
type retainAfterLoginTest struct{}

func (t *retainAfterLoginTest) preparationAtLoginScreen(ctx context.Context, cr *chrome.Chrome, res *proxyRetainResource, pvs []*proxysettings.Config) (retErr error) {
	testing.ContextLog(ctx, "Setting up proxy at login screen")

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := startChrome(ctx, res, true /* isNoLogin */, true /* isKeepState */, chrome.FakeLogin(primaryUser))
	if err != nil {
		return errors.Wrap(err, "failed to sign in")
	}
	defer cr.Close(cleanupCtx)

	ps, err := proxysettings.CollectFromSigninScreen(ctx, res.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to create proxy settings instance")
	}
	defer ps.Close(cleanupCtx, res.kb)
	defer faillog.DumpUITreeOnErrorToFile(cleanupCtx, res.outDir, func() bool { return retErr != nil }, res.tconn, "before_close_ossettings_ui_dump")

	for _, pv := range pvs {
		if err := ps.Setup(ctx, cr, res.kb, pv); err != nil {
			return errors.Wrapf(err, "failed to set proxy fields for %s", pv.HostName())
		}
	}

	return nil
}

func (t *retainAfterLoginTest) preparationAfterLoggedIn(ctx context.Context, cr *chrome.Chrome, res *proxyRetainResource, pvs []*proxysettings.Config) error {
	return nil
}

// retainAcrossUsersTest is a test case structure for the proxy retain across users.
type retainAcrossUsersTest struct{}

func (t *retainAcrossUsersTest) preparationAtLoginScreen(ctx context.Context, cr *chrome.Chrome, res *proxyRetainResource, pvs []*proxysettings.Config) error {
	return nil
}

func (t *retainAcrossUsersTest) preparationAfterLoggedIn(ctx context.Context, cr *chrome.Chrome, res *proxyRetainResource, pvs []*proxysettings.Config) (retErr error) {
	testing.ContextLog(ctx, "Setting up proxy after logged in")

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := startChrome(ctx, res, false /* isNoLogin */, true /* isKeepState */, chrome.FakeLogin(primaryUser))
	if err != nil {
		return errors.Wrap(err, "failed to sign in")
	}
	defer cr.Close(cleanupCtx)

	ps, err := proxysettings.Collect(ctx, res.tconn)
	if err != nil {
		return errors.Wrap(err, "failed to create proxy settings instance")
	}
	defer ps.Close(cleanupCtx, res.kb)
	defer faillog.DumpUITreeOnErrorToFile(cleanupCtx, res.outDir, func() bool { return retErr != nil }, res.tconn, "before_close_ossettings_ui_dump")

	for _, pv := range pvs {
		if err := ps.Setup(ctx, cr, res.kb, pv); err != nil {
			return errors.Wrapf(err, "failed to set proxy fields for %s", pv.HostName())
		}
	}

	return nil
}
