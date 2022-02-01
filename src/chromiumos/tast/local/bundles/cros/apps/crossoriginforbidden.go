// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/apps/isolatedapp"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Crossoriginforbidden,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks whether chrome refuses to follow cross origin links in isolated web apps",
		Contacts: []string{
			"simonha@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + time.Minute,
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedInIsolatedApp,
		Data: []string{
			"crossoriginforbidden.html",
			"crossoriginforbidden.webmanifest",
			"favicon.ico",
			"cross_origin_forbidden_icon-192x192.png",
			"cross_origin_forbidden_icon-512x512.png",
			"key.pem",
			"certificate.pem",
			"ca-cert.pem",
		},
	})
}

func Crossoriginforbidden(ctx context.Context, s *testing.State) {

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Open a keyboard device.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer kb.Close()

	mainAppFile := "crossoriginforbidden.html"
	totalPath := s.DataPath(mainAppFile)
	httpsServerConfiguration := isolatedapp.HTTPSServer{
		Headers: map[string]string{
			"Cross-Origin-Embedder-Policy": "require-corp",
			"Cross-Origin-Opener-Policy":   "same-origin",
		},
		ServerKeyPath:         s.DataPath("key.pem"),
		ServerCertificatePath: s.DataPath("certificate.pem"),
		HostedFilesBasePath:   totalPath[:len(totalPath)-len(mainAppFile)],
		CaCertificatePath:     s.DataPath("ca-cert.pem"),
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), browser.TypeAsh)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)

	isolatedapp.ConfigureChromeToAcceptCertificate(ctx, httpsServerConfiguration, cr, br, tconn)
	isolatedapp.StartServer(httpsServerConfiguration)

	policies := []policy.Policy{
		&policy.WebAppInstallForceList{
			Val: []*policy.WebAppInstallForceListValue{
				{
					Url:                    "https://localhost/crossoriginforbidden.html",
					DefaultLaunchContainer: "window",
					CreateDesktopShortcut:  false,
					CustomName:             "",
					FallbackAppName:        "",
					CustomIcon: &policy.WebAppInstallForceListValueCustomIcon{
						Hash: "",
						Url:  "",
					},
				},
			},
		},
	}
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, policies); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	// Wait until the PWA is installed.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		const name = "CrossOriginForbidden"
		if err := launcher.SearchAndLaunch(tconn, kb, name)(ctx); err != nil {
			return errors.Wrapf(err, "failed to launch %s", name)
		}

		windows, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(errors.Wrap(err, "failed to get windows"))
		}

		for _, window := range windows {
			if window.Title == name {
				return nil
			}
		}
		return errors.New("failed to find a window with the PWA")
	}, nil); err != nil {
		s.Error("PWA wasn't installed: ", err)
	}

	ui := uiauto.New(tconn)
	navigateButton := nodewith.NameContaining("Navigate").Role(role.Link)
	cannotOpenExpectedWindow := nodewith.NameContaining("Google Chrome OS can't open this page.").First()
	if err := uiauto.Combine("cross_origin_link",
		ui.WaitUntilExists(navigateButton),
		ui.LeftClick(navigateButton),
		// This is the expectation we are waiting for
		ui.WaitUntilExists(cannotOpenExpectedWindow),
	)(ctx); err != nil {
		s.Fatal(errors.Wrap(err, "Cross origin link check failed"))
	}

}
