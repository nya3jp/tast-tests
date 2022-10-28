// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/checked"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/https"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BlockThirdPartyCookies,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test the behavior of BlockThirdPartyCookies policy: check if third party cookies are allowed based on policy value",
		Contacts: []string{
			"snijhara@google.com", // Test author
			"phweiss@google.com",  // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Data: []string{
			"third_party_cookies.html",
			"third_party_cookies.js",
			"certificate.pem",
			"key.pem",
			"cert_for_127.0.0.1.pem",
			"key_for_127.0.0.1.pem",
			"ca-cert.pem",
		},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.BlockThirdPartyCookies{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func BlockThirdPartyCookies(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// radioButtonNames is a list of UI element names in the cookies settings page.
	// The order of the strings should follow the order in the settings page.
	// wantRestriction and wantChecked entries are expected to follow this order as well.
	radioButtonNames := []string{
		"Allow all cookies",
		"Block third-party cookies",
	}

	// TODO(crbug.com/1298550): Don't rely on all files being in same directory.
	baseDirectory, _ := filepath.Split(s.DataPath("certificate.pem"))
	localhostConfiguration := https.ServerConfiguration{
		ServerKeyPath:         s.DataPath("key.pem"),
		ServerCertificatePath: s.DataPath("certificate.pem"),
		CaCertificatePath:     s.DataPath("ca-cert.pem"),
		HostedFilesBasePath:   baseDirectory,
	}

	// This certificate is signed with the same CA key, but specifies 127.0.0.1 instead of
	// localhost as Subject Alternative Name.
	IPConfiguration := https.ServerConfiguration{
		ServerKeyPath:         s.DataPath("key_for_127.0.0.1.pem"),
		ServerCertificatePath: s.DataPath("cert_for_127.0.0.1.pem"),
		CaCertificatePath:     s.DataPath("ca-cert.pem"),
		HostedFilesBasePath:   baseDirectory,
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get TestConn: ", err)
	}

	// For third_party_cookies.html, we need to insert the right port number to the other server into the HTML.
	path := s.DataPath("third_party_cookies.html")
	htmlBuffer, err := ioutil.ReadFile(path)
	if err != nil {
		s.Fatal("Couldn't read .html template file")
	}
	htmlContent := string(htmlBuffer)

	// Read third_party_cookies.js, because we need to write it in our custom handler that also sets the cookies.
	jsPath := s.DataPath("third_party_cookies.js")
	jsBuffer, err := ioutil.ReadFile(jsPath)
	if err != nil {
		s.Fatal("Couldn't read .js file")
	}
	jsContent := string(jsBuffer)

	// Serve the modified HTML content instead of the original file.
	IPConfiguration.HandleFunc("/third_party_cookies.html", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(200)
		fmt.Fprintf(w, htmlContent)
	})

	// Serve the unmodified js content and set the third-party cookie.
	localhostConfiguration.HandleFunc("/third_party_cookies.js", func(w http.ResponseWriter, req *http.Request) {
		// SameSite and Secure are mandatory for third-party cookies, that's why we are using https.
		cookie := &http.Cookie{
			Name:     "token",
			Value:    "some_token",
			MaxAge:   3000,
			Domain:   "localhost",
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		}
		http.SetCookie(w, cookie)
		w.WriteHeader(200)
		fmt.Fprintf(w, jsContent)
	})

	// Start both servers.
	// First party.
	ipServer := https.StartServer(IPConfiguration)
	if ipServer.Error != nil {
		s.Fatal("Could not start https server: ", err)
	}
	defer ipServer.Close()

	// Third party.
	localhostServer := https.StartServer(localhostConfiguration)
	if localhostServer.Error != nil {
		s.Fatal("Could not start https server: ", err)
	}
	defer localhostServer.Close()

	ipPort := ipServer.Address[strings.LastIndex(ipServer.Address, ":")+1:]
	localhostPort := localhostServer.Address[strings.LastIndex(localhostServer.Address, ":")+1:]

	// Provide the third-party server's port in the html file.
	htmlContent = strings.Replace(htmlContent, "LOCALHOST_PORT", localhostPort, 1)

	for _, param := range []struct {
		name            string                    // name is the name of the test case.
		wantRestriction []restriction.Restriction // The expected restriction states of the radio buttons in
		// radioButtonNames.
		wantChecked []checked.Checked // The expected checked states of the radio buttons in
		// radioButtonNames.
		wantCookie bool                           // Whether the third-party cookie should be successfully set.
		policy     *policy.BlockThirdPartyCookies // policy is the policy we test.
	}{
		{
			name:            "unset",
			wantRestriction: []restriction.Restriction{restriction.None, restriction.None},
			wantChecked:     []checked.Checked{checked.False, checked.False},
			wantCookie:      true,
			policy:          &policy.BlockThirdPartyCookies{Stat: policy.StatusUnset},
		},
		{
			name:            "allow",
			wantRestriction: []restriction.Restriction{restriction.None, restriction.Disabled},
			wantChecked:     []checked.Checked{checked.True, checked.False},
			wantCookie:      true,
			policy:          &policy.BlockThirdPartyCookies{Val: false},
		},
		{
			name: "block",
			// The radio button for "Block third-party cookies" is not disabled in this case as the user can switch
			// between blocking only third party cookies or all cookies for which there is another radio button on
			// the cookies settings page.
			wantRestriction: []restriction.Restriction{restriction.Disabled, restriction.None},
			wantChecked:     []checked.Checked{checked.False, checked.True},
			wantCookie:      false,
			policy:          &policy.BlockThirdPartyCookies{Val: true},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.policy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := https.ConfigureChromeToAcceptCertificate(ctx, localhostConfiguration, cr, br, tconn); err != nil {
				s.Fatal("Failed to set certificate: ", err)
			}

			// Open cookies settings page.
			conn, err := br.NewConn(ctx, "chrome://settings/cookies")
			if err != nil {
				s.Fatal("Failed to open cookies settings: ", err)
			}
			defer conn.Close()

			// Open cookies settings page and check the state of the radio buttons.
			for i, radioButtonName := range radioButtonNames {
				if err := policyutil.CurrentPage(cr).
					SelectNode(ctx, nodewith.
						Role(role.RadioButton).
						Name(radioButtonName)).
					Restriction(param.wantRestriction[i]).
					Checked(param.wantChecked[i]).
					Verify(); err != nil {
					s.Errorf("Unexpected settings state for the %q button: %v", radioButtonName, err)
				}
			}

			// Load page that sets one 127.0.0.1 first-party cookie and one localhost third-party cookie.
			conn2, err := br.NewConn(ctx, "https://127.0.0.1:"+ipPort+"/third_party_cookies.html")
			if err != nil {
				s.Fatal("Failed to connect to the browser: ", err)
			}
			defer conn2.Close()

			// Check which cookies got created.
			conn3, err := br.NewConn(ctx, "chrome://settings/content/all")
			if err != nil {
				s.Fatal("Failed to connect to the browser: ", err)
			}
			defer conn3.Close()

			ui := uiauto.New(tconn)
			localhostText := nodewith.NameStartingWith("localhost").Role(role.StaticText)
			ipText := nodewith.NameStartingWith("127.0.0.1").Role(role.StaticText)
			expandButton := nodewith.NameStartingWith("127.0.0.1").Role(role.Button).ClassName("icon-expand-more")
			removeAllButton := nodewith.Name("Clear All Data").Role(role.Button)
			confirmRemoveAllButton := nodewith.Name("Clear all").Role(role.Button)

			// The removeAllButton is labeled "Clear all data" for non-Google Chromium
			// builds. Use this to tell the difference between the two cases.
			if err := ui.WaitUntilExists(removeAllButton)(ctx); err != nil {
				removeAllButton = nodewith.Name("Clear all data").Role(role.Button)
				confirmRemoveAllButton = nodewith.Name("Clear").Role(role.Button)
				// The expand button only exists in non-Google branded builds, and only
				// if first and third party cookies exist. Click it to reveal third
				// party cookies, but ignore errors.
				ui.LeftClick(expandButton)(ctx)
			}

			checkCookieExistence := ui.WaitUntilExists(localhostText)
			if !param.wantCookie {
				checkCookieExistence = ui.Gone(localhostText)
			}

			if err := uiauto.Combine("check_and_clear_cookies",
				ui.WaitUntilExists(ipText),
				checkCookieExistence,
				ui.LeftClick(removeAllButton),
				ui.WaitUntilExists(confirmRemoveAllButton),
				ui.LeftClick(confirmRemoveAllButton),
			)(ctx); err != nil {
				s.Error("Failed to check and clean up cookies: ", err)
			}
		})
	}
}
