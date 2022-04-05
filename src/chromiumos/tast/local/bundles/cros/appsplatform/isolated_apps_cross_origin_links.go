// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package appsplatform

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/https"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

const (
	mainAppFile              = "isolated_apps_cross_origin_links_index.html"
	serverKeyFile            = "isolated_apps_cross_origin_links_key.pem"
	serverCertificateFile    = "isolated_apps_cross_origin_links_certificate.pem"
	certificateAuthorityFile = "isolated_apps_cross_origin_links_ca_cert.pem"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         IsolatedAppsCrossOriginLinks,
		LacrosStatus: testing.LacrosVariantNeeded,
		// TODO(crbug.com/1292633): Update test to check that cross origin links are opened in a separate browser window
		Desc: "Checks whether Chrome refuses to follow cross origin links in isolated web apps",
		Contacts: []string{
			"simonha@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Timeout:      chrome.GAIALoginTimeout + time.Minute,
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedInIsolatedApp,
		Data: []string{
			mainAppFile,
			serverKeyFile,
			serverCertificateFile,
			certificateAuthorityFile,
			"isolated_apps_cross_origin_links.webmanifest",
			"isolated_apps_cross_origin_links_icon-192x192.png",
			"isolated_apps_cross_origin_links_icon-512x512.png",
		},
	})
}

func IsolatedAppsCrossOriginLinks(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Open a keyboard device.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer kb.Close()

	// TODO(crbug.com/1298550): Don't rely on all files being in same directory.
	baseDirectory, fileName := filepath.Split(s.DataPath(mainAppFile))
	ServerConfiguration := https.ServerConfiguration{
		Headers: map[string]string{
			"Cross-Origin-Embedder-Policy": "require-corp",
			"Cross-Origin-Opener-Policy":   "same-origin",
		},
		ServerKeyPath:         s.DataPath(serverKeyFile),
		ServerCertificatePath: s.DataPath(serverCertificateFile),
		HostedFilesBasePath:   baseDirectory,
		CaCertificatePath:     s.DataPath(certificateAuthorityFile),
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

	https.ConfigureChromeToAcceptCertificate(ctx, ServerConfiguration, cr, br, tconn)
	server := https.StartServer(ServerConfiguration)
	if server.Error != nil {
		s.Fatal("Could not start https server: ", err)
	}
	defer server.Close()

	policies := []policy.Policy{
		&policy.WebAppInstallForceList{
			Val: []*policy.WebAppInstallForceListValue{
				{
					Url:                    server.Address + "/" + fileName,
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
		const name = "IsolatedAppsCrossOriginLinks"
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
