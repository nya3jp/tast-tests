// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"image/color"
	"path/filepath"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/colorcmp"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/https"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebAppInstallForceList,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of WebAppInstallForceList policy",
		Contacts: []string{
			"alexanderhartl@google.com", // Test author
			"phweiss@google.com",        // Test author of customization test cases
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:commercial_limited"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		Data: []string{"web_app_install_force_list_index.html",
			"web_app_install_force_list_manifest.json",
			"web_app_install_force_list_service-worker.js",
			"web_app_install_force_list_icon-192x192.png",
			"web_app_install_force_list_icon-512x512.png",
			"web_app_install_force_list_icon-192x192-red.png",
			"web_app_install_force_list_icon-192x192-green.png",
			"web_app_install_force_list_no_manifest.html",
			"certificate.pem",
			"key.pem",
			"ca-cert.pem"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.WebAppInstallForceList{}, pci.VerifiedFunctionalityUI),
		},
	})
}

func WebAppInstallForceList(ctx context.Context, s *testing.State) {
	const (
		colorMaxDiff = 32
		// Offset of icon from the app name on the app's settings page, in dip.
		iconOffsetX = -30
		iconOffsetY = +10
	)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeAsh)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get TestConn: ", err)
	}

	// TODO(crbug.com/1298550): Don't rely on all files being in same directory.
	baseDirectory, _ := filepath.Split(s.DataPath("certificate.pem"))
	ServerConfiguration := https.ServerConfiguration{
		ServerKeyPath:         s.DataPath("key.pem"),
		ServerCertificatePath: s.DataPath("certificate.pem"),
		CaCertificatePath:     s.DataPath("ca-cert.pem"),
		HostedFilesBasePath:   baseDirectory,
	}

	https.ConfigureChromeToAcceptCertificate(ctx, ServerConfiguration, cr, br, tconn)
	server := https.StartServer(ServerConfiguration)
	if server.Error != nil {
		s.Fatal("Could not start https server: ", err)
	}
	defer server.Close()

	for _, param := range []struct {
		// name is the subtest name.
		name string
		// appName is an expected substring of the app's name.
		// This is because the full name for placeholder-apps contains the test-server's IP.
		appName   string
		iconColor color.RGBA
		// value is the policy value.
		value *policy.WebAppInstallForceList
	}{
		{
			name:    "one_pwa_no_customization",
			appName: `Test PWA`,
			// The PWA has a white icon.
			iconColor: color.RGBA{255, 255, 255, 255},
			value: &policy.WebAppInstallForceList{
				Val: []*policy.WebAppInstallForceListValue{
					{
						Url:                    server.Address + "/web_app_install_force_list_index.html",
						DefaultLaunchContainer: "window",
						CreateDesktopShortcut:  false,
					},
				},
			},
		},
		{
			name:    "one_pwa_custom_name_and_icon",
			appName: "CUSTOM",
			// The custom icon is red.
			iconColor: color.RGBA{255, 0, 0, 255},
			value: &policy.WebAppInstallForceList{
				Val: []*policy.WebAppInstallForceListValue{
					{
						Url:                    server.Address + "/web_app_install_force_list_index.html",
						DefaultLaunchContainer: "window",
						CreateDesktopShortcut:  false,
						CustomName:             "CUSTOM",
						CustomIcon: &policy.WebAppInstallForceListValueCustomIcon{
							Hash: "d8fb0f842189a95200f422452ea648ab545f2817f07a389393b63ed93aeba797",
							Url:  server.Address + "/web_app_install_force_list_icon-192x192-red.png",
						},
					},
				},
			},
		},
		{
			name: "one_website_no_manifest_no_customization",
			// <title> of the website.
			appName: "Test Website",
			// Color of the favicon (green).
			iconColor: color.RGBA{0, 255, 0, 255},
			value: &policy.WebAppInstallForceList{
				Val: []*policy.WebAppInstallForceListValue{
					{
						Url:                    server.Address + "/web_app_install_force_list_no_manifest.html",
						DefaultLaunchContainer: "window",
						CreateDesktopShortcut:  false,
					},
				},
			},
		},
		{
			name:    "one_website_no_manifest_custom_name_and_icon",
			appName: "foobar",
			// The custom icon is red.
			iconColor: color.RGBA{255, 0, 0, 255},
			value: &policy.WebAppInstallForceList{
				Val: []*policy.WebAppInstallForceListValue{
					{
						Url:                    server.Address + "/web_app_install_force_list_no_manifest.html",
						DefaultLaunchContainer: "window",
						CreateDesktopShortcut:  false,
						CustomName:             "foobar",
						CustomIcon: &policy.WebAppInstallForceListValueCustomIcon{
							Hash: "d8fb0f842189a95200f422452ea648ab545f2817f07a389393b63ed93aeba797",
							Url:  server.Address + "/web_app_install_force_list_icon-192x192-red.png",
						},
					},
				},
			},
		},
		{
			name: "app_does_not_exist_custom_icon",
			// The real name is <IP-address of test-server>/does_not_exist.html, but we are only checking for a substring.
			appName: "does_not_exist.html",
			// The custom icon is red.
			iconColor: color.RGBA{255, 0, 0, 255},
			value: &policy.WebAppInstallForceList{
				Val: []*policy.WebAppInstallForceListValue{
					{
						Url:                    server.Address + "/does_not_exist.html",
						DefaultLaunchContainer: "window",
						CreateDesktopShortcut:  false,
						CustomIcon: &policy.WebAppInstallForceListValueCustomIcon{
							Hash: "d8fb0f842189a95200f422452ea648ab545f2817f07a389393b63ed93aeba797",
							Url:  server.Address + "/web_app_install_force_list_icon-192x192-red.png",
						},
					},
				},
			},
		},
		{
			name:    "app_does_not_exist_custom_name_and_updated_icon",
			appName: "barfoo",
			// The updated custom icon is green.
			iconColor: color.RGBA{0, 255, 0, 255},
			value: &policy.WebAppInstallForceList{
				Val: []*policy.WebAppInstallForceListValue{
					{
						Url:                    server.Address + "/does_not_exist.html",
						DefaultLaunchContainer: "window",
						CreateDesktopShortcut:  false,
						CustomName:             "barfoo",
						CustomIcon: &policy.WebAppInstallForceListValueCustomIcon{
							Hash: "5a5483b279df898b75ef36b82a2953cee3e93dcf267c7dde3f5ecaad867be902",
							Url:  server.Address + "/web_app_install_force_list_icon-192x192-green.png",
						},
					},
				},
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Navigate to the app's settings page.
			conn, err := apps.LaunchOSSettings(ctx, cr, "chrome://os-settings/app-management")
			if err != nil {
				s.Fatal("Failed to open OS app management page: ", err)
			}
			defer conn.Close()

			ui := uiauto.New(tconn)
			pwaButton := nodewith.NameContaining(param.appName).Role(role.Link)
			if err := ui.WaitUntilExists(pwaButton)(ctx); err != nil {
				s.Fatal("Failed to wait for the app to be installed: ", err)
			}

			if err := ui.LeftClick(pwaButton)(ctx); err != nil {
				s.Fatal("Clicking app in settings failed: ", err)
			}

			// Wait for the page to open and check app name and app icon in the heading.
			heading := nodewith.NameContaining(param.appName).Role(role.Heading)
			if err := ui.WaitUntilExists(heading)(ctx); err != nil {
				s.Fatal("Failed to wait for the app's settings page to open: ", err)
			}
			headingInfo, err := ui.Info(ctx, heading)
			if err != nil {
				s.Fatal("Failed to get heading info: ", err)
			}

			// The remaining code checks the icon's color.

			img, err := screenshot.GrabScreenshot(ctx, cr)
			if err != nil {
				s.Fatal("Failed to grab screenshot: ", err)
			}

			// Get Display Scale Factor to use it to convert bounds in dip to pixels.
			displayInfo, err := display.GetPrimaryInfo(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get the primary display info: ", err)
			}
			displayMode, err := displayInfo.GetSelectedMode()
			if err != nil {
				s.Fatal("Failed to get the selected display mode of the primary display: ", err)
			}
			deviceScaleFactor := displayMode.DeviceScaleFactor

			// The icon is slightly to the left of the heading (which contains just the app name).
			// This aims exactly for the center of the icon.
			loc := headingInfo.Location
			loc.Left += iconOffsetX
			loc.Top += iconOffsetY
			samplePt := coords.ConvertBoundsFromDPToPX(loc, deviceScaleFactor).TopLeft()
			sampleColor := img.At(samplePt.X, samplePt.Y)

			if !colorcmp.ColorsMatch(sampleColor, param.iconColor, colorMaxDiff) {
				s.Errorf("Icon colors did not match, got %v, expected %v", sampleColor, param.iconColor)
			}
		})
	}
}
