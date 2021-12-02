// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeOSPageInfo,
		Desc: "Check the chrome OS page shows enough information to user",
		Contacts: []string{
			"cienet-development@googlegroups.com",
			"chromeos-sw-engprod@google.com",
			"lyle.lai@cienet.com",
			"ting.chen@cienet.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      3 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

// ChromeOSPageInfo checks chromeOS version info and online help available to user.
func ChromeOSPageInfo(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	ui := uiauto.New(tconn)

	s.Log("Open setting page and starting test")
	settings, err := ossettings.LaunchAtPage(ctx, tconn, ossettings.AboutChromeOS)
	if err != nil {
		s.Fatal("Failed to open setting page: ", err)
	}
	defer settings.Close(cleanupCtx)
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(cleanupCtx, cr, s.OutDir(), s.HasError)

	s.Log("Check Chrome OS version")
	if err := checkVersion(settings)(ctx); err != nil {
		s.Fatal("Failed to check Chrome OS version: ", err)
	}

	s.Log("Check update")
	if err := checkUpdate(settings)(ctx); err != nil {
		s.Fatal("Failed to check update to Chrome OS: ", err)
	}

	s.Log("Check online help")
	if err := checkOnlineHelp(ui, settings)(ctx); err != nil {
		s.Fatal("Failed to check online help: ", err)
	}

	s.Log("Check report issue")
	if err := checkReportIssue(ui, settings)(ctx); err != nil {
		s.Fatal("Failed to check report issue: ", err)
	}

	s.Log("Check detailed build informations")
	if err := checkDetail(ui, settings)(ctx); err != nil {
		s.Fatal("Failed to check detailed build informations: ", err)
	}

	s.Log("Check open source links")
	if err := checkOpensources(ui, cr, settings)(ctx); err != nil {
		s.Fatal("Failed to check open source links: ", err)
	}

	s.Log("Check term of services")
	if err := checkTermsOfServiceLinks(ui, settings)(ctx); err != nil {
		s.Fatal("Failed to check term of services: ", err)
	}
}

func checkVersion(settings *ossettings.OSSettings) uiauto.Action {
	return settings.WaitUntilExists(ossettings.VersionInfo)
}

func checkUpdate(settings *ossettings.OSSettings) uiauto.Action {
	return settings.WaitUntilExists(ossettings.CheckUpdateBtn)
}

func checkOnlineHelp(ui *uiauto.Context, settings *ossettings.OSSettings) uiauto.Action {
	helpRoot := nodewith.Name(apps.Help.Name).HasClass("BrowserFrame").Role(role.Window)

	return uiauto.Combine("check get help",
		settings.LaunchHelpApp(),
		ui.WaitUntilExists(nodewith.Name("Welcome to your Chromebook").Role(role.StaticText).Ancestor(helpRoot)),
		ui.LeftClick(nodewith.Name("Close").Ancestor(helpRoot)),
	)
}

func checkReportIssue(ui *uiauto.Context, settings *ossettings.OSSettings) uiauto.Action {
	feedbackRoot := nodewith.Name("Feedback").Role(role.RootWebArea)

	return uiauto.Combine("check report issue",
		settings.LeftClick(ossettings.ReportIssue),
		ui.WaitUntilExists(nodewith.Name("Send feedback to Google").Role(role.StaticText).Ancestor(feedbackRoot)),
		ui.LeftClick(nodewith.Name("Close").Ancestor(feedbackRoot)),
	)
}

func checkDetail(ui *uiauto.Context, settings *ossettings.OSSettings) uiauto.Action {
	detailRoot := nodewith.Name("Chrome - About Version").HasClass("BrowserFrame").Role(role.Window)

	return uiauto.Combine("click details",
		settings.LeftClick(ossettings.AdditionalDetails),
		func(ctx context.Context) error {
			arr, err := ui.Info(ctx, ossettings.ChangeChannelBtn)
			if err != nil {
				return err
			}
			if arr.HTMLAttributes["aria-disabled"] == "true" {
				return errors.New("change channel button diabled")
			}
			return nil
		},
		settings.LeftClick(ossettings.BuildDetailsBtn),
		ui.WaitUntilExists(nodewith.Name("Platform").Role(role.StaticText).Ancestor(detailRoot)),
		ui.LeftClick(nodewith.Name("Close").HasClass("FrameCaptionButton").Ancestor(detailRoot)),
		settings.LeftClick(ossettings.BackArrowBtn),
		settings.WaitUntilGone(ossettings.BackArrowBtn),
	)
}

func checkOpensources(ui *uiauto.Context, cr *chrome.Chrome, settings *ossettings.OSSettings) uiauto.Action {
	return func(ctx context.Context) error {
		if err := settings.FocusAndWait(ossettings.OpenSourceSoftwares.First())(ctx); err != nil {
			return errors.Wrap(err, "failed to focus on node")
		}

		infos, err := ui.NodesInfo(ctx, ossettings.OpenSourceSoftwares)
		if err != nil {
			return errors.Wrap(err, "failed to get opensource nodes info")
		}
		if len(infos) != 2 {
			return errors.Errorf("unexpected UI result: %+v", infos)
		}

		matchTargetCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		for _, opensource := range []struct {
			node uiauto.NodeInfo
			url  string
		}{
			{node: infos[0], url: "chrome://credits/"},
			{node: infos[1], url: "chrome://os-credits/"},
		} {
			if err := ui.MouseClickAtLocation(0, opensource.node.Location.CenterPoint())(ctx); err != nil {
				return errors.Wrap(err, "failed to click opensource link")
			}

			conn, err := cr.NewConnForTarget(matchTargetCtx, chrome.MatchTargetURL(opensource.url))
			if err != nil {
				return errors.Wrap(err, "failed to find expected page")
			}

			if err := conn.CloseTarget(ctx); err != nil {
				return errors.Wrap(err, "failed to close target")
			}
			if err := conn.Close(); err != nil {
				return errors.Wrap(err, "failed to close connection")
			}
		}
		return nil
	}
}

func checkTermsOfServiceLinks(ui *uiauto.Context, settings *ossettings.OSSettings) uiauto.Action {
	title := "Google Chrome and Chrome OS Additional Terms of Service"
	termsRoot := nodewith.Name("Chrome - " + title).HasClass("BrowserFrame").Role(role.Window)
	termsOfServiceTitle := nodewith.Name(title).Role(role.Heading).Ancestor(termsRoot)

	return uiauto.Combine("click term of service",
		settings.FocusAndWait(ossettings.TermsOfService),
		settings.LeftClick(ossettings.TermsOfService),
		ui.WaitUntilExists(termsOfServiceTitle),
		ui.LeftClick(nodewith.Name("Close").HasClass("FrameCaptionButton").Ancestor(termsRoot)),
	)
}
