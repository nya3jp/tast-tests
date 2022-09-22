// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChooseToSharePageURLOrNot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify user can choose to share page url or not",
		Contacts: []string{
			"wangdanny@google.com",
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "share_page_url",
			Val:  true,
		}, {
			Name: "not_share_page_url",
			Val:  false,
		}},
	})
}

const tabLink = "chrome://newtab/"

// ChooseToSharePageURLOrNot verifies user can choose to share page url or not.
func ChooseToSharePageURLOrNot(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	s.Log("Setting up chrome")
	cr, err := chrome.New(ctx, chrome.EnableFeatures(
		"OsFeedback", "OsFeedbackSaveReportToLocalForE2ETesting"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr,
		"ui_dump")

	// Clean up in the end.
	defer func() {
		if err := os.RemoveAll(feedbackapp.ReportPath); err != nil {
			s.Log("Failed to remove feedback report: ", err)
		}
	}()

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)
	sharePageURL := s.Param().(bool)

	// Open chrome browser.
	if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
		s.Fatal("Failed to launch chrome app: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Chrome.ID, time.Minute); err != nil {
		s.Fatal("Chrome app did not appear in shelf after launch: ", err)
	}

	// Launch feedback app and go to share data page.
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and navigating to share data page: ", err)
	}

	// Verify url text exists.
	urlText := nodewith.NameContaining(tabLink).Role(
		role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(urlText)(ctx); err != nil {
		s.Fatal("Failed to find element: ", err)
	}

	// Uncheck the share page url checkbox if needed.
	if !sharePageURL {
		checkboxAncestor := nodewith.NameContaining("Share URL").Role(
			role.GenericContainer).Ancestor(feedbackRootNode)
		checkbox := nodewith.Role(role.CheckBox).Ancestor(checkboxAncestor)
		if err := uiauto.Combine("Uncheck the share url checkbox",
			ui.DoDefault(checkbox),
			ui.WaitUntilExists(checkbox.Attribute("checked", "false")),
		)(ctx); err != nil {
			s.Fatal("Failed to uncheck the share url checkbox: ", err)
		}
	}

	// Submit the feedback and verify confirmation page title exists.
	sendButton := nodewith.Name("Send").Role(role.Button).Ancestor(feedbackRootNode)
	confirmationPageTitle := nodewith.Name("Thanks for your feedback").Role(
		role.StaticText).Ancestor(feedbackRootNode)

	if err := uiauto.Combine("Submit feedback and verify",
		ui.DoDefault(sendButton),
		ui.WaitUntilExists(confirmationPageTitle),
	)(ctx); err != nil {
		s.Fatal("Failed to submit feedback and verify: ", err)
	}

	// Read feedback report content.
	var content []byte

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		content, err = ioutil.ReadFile(feedbackapp.ReportPath)
		if err != nil {
			return errors.Wrap(err, "failed to read report content")
		}

		return nil
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		s.Fatal("Failed to read report content: ", err)
	}

	actualContent := strings.ToValidUTF8(string(content), "")
	reportContainsURL := strings.Contains(actualContent, tabLink)

	// Verify if report contains the page url.
	if sharePageURL {
		if !reportContainsURL {
			s.Fatal("Failed to verify user chooses to share page url")
		}
	} else {
		if reportContainsURL {
			s.Fatal("Failed to verify user chooses not to share page url")
		}
	}
}
