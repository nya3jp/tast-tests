// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/protobuf/proto"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	fpb "chromiumos/tast/local/bundles/cros/feedback/proto"
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
		Fixture:      "chromeLoggedInWithOsFeedbackSaveReportToLocalForE2ETesting",
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
	})
}

const tabLink = "chrome://newtab/"

// ChooseToSharePageURLOrNot verifies user can choose to share page url or not.
func ChooseToSharePageURLOrNot(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Test API: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr,
		"ui_dump")

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	// Open chrome browser.
	if err := apps.Launch(ctx, tconn, apps.Chrome.ID); err != nil {
		s.Fatal("Failed to launch chrome app: ", err)
	}

	if err := ash.WaitForApp(ctx, tconn, apps.Chrome.ID, time.Minute); err != nil {
		s.Fatal("Chrome app did not appear in shelf after launch: ", err)
	}

	cleanUp := func() {
		if err := os.RemoveAll(feedbackapp.ReportPath); err != nil {
			s.Error("Failed to remove feedback report: ", err)
		}
	}

	for _, tc := range []struct {
		name         string
		sharePageURL bool
	}{
		{
			name:         "share_page_url",
			sharePageURL: true,
		},
		{
			name:         "not_share_page_url",
			sharePageURL: false,
		},
	} {
		s.Run(ctx, tc.name, func(ctx context.Context, s *testing.State) {
			// Clean up at both the beginning and the end to make sure
			// there is no file in the report path.
			cleanUp()
			defer cleanUp()

			// Launch feedback app and go to share data page.
			feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
			if err != nil {
				s.Fatal(
					"Failed to launch feedback app and go to share data page: ", err)
			}

			// Verify url text exists.
			urlText := nodewith.NameContaining(tabLink).Role(
				role.Link).Ancestor(feedbackRootNode)
			if err := ui.WaitUntilExists(urlText)(ctx); err != nil {
				s.Fatal("Failed to find element: ", err)
			}

			// Uncheck the share page url checkbox if needed.
			if !tc.sharePageURL {
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
			sendButton := nodewith.Name("Send").Role(
				role.Button).Ancestor(feedbackRootNode)
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

			report := &fpb.ExtensionSubmit{}
			if err = proto.Unmarshal(content, report); err != nil {
				s.Fatal("Failed to parse report: ", err)
			}
			reportURL := report.GetWebData().GetUrl()

			// Verify if report contains the page url.
			if tc.sharePageURL {
				if reportURL != tabLink {
					s.Fatal("Failed to verify can choose to share page url")
				}
			} else {
				if reportURL != "" {
					s.Fatal("Failed to verify can choose not to share page url")
				}
			}

			// Click done button and verify feedback window is closed.
			doneButton := nodewith.Name("Done").Role(
				role.Button).Ancestor(feedbackRootNode)

			if err := uiauto.Combine("Verify feedback window is closed",
				ui.DoDefault(doneButton),
				ui.WaitUntilGone(feedbackRootNode),
			)(ctx); err != nil {
				s.Fatal("Failed to verify feedback window is closed: ", err)
			}
		})
	}
}
