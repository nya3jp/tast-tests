// Copyright 2022 The ChromiumOS Authors.
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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/localstate"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

const reportPath = "/tmp/feedback-report/feedback-report"

// reportContainsEmailParam contains all the data needed to run a single test iteration.
type reportContainsEmailParam struct {
	includeEmail bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ReportContainsEmail,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify feedback report contains user email",
		Contacts: []string{
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "include_email",
			Val: reportContainsEmailParam{
				includeEmail: true,
			},
		}, {
			Name: "not_include_email",
			Val: reportContainsEmailParam{
				includeEmail: false,
			},
		}},
	})
}

// ReportContainsEmail verifies feedback report contains user email.
func ReportContainsEmail(ctx context.Context, s *testing.State) {
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
		if err := os.RemoveAll(reportPath); err != nil {
			s.Log("Failed to remove feedback report")
		}
	}()

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	// Launch feedback app and go to share data page.
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and navigating to share data page: ", err)
	}

	// Find default selected email address.
	var localState struct {
		Emails []string `json:"LoggedInUsers"`
	}

	if err := localstate.Unmarshal(browser.TypeAsh, &localState); err != nil {
		s.Fatal("Failed to extract Local State: ", err)
	}

	var selectedEmail string
	for _, email := range localState.Emails {
		if err := ui.WaitUntilExists(nodewith.NameContaining(email).First())(
			ctx); err == nil {
			selectedEmail = email
		}
	}

	// Choose not to include email if needed.
	if !s.Param().(reportContainsEmailParam).includeEmail {
		for i := 0; i < 4; i++ {
			if err := kb.Accel(ctx, "Tab"); err != nil {
				s.Fatal("Failed pressing tab key: ", err)
			}
		}

		if err := kb.Accel(ctx, "Down"); err != nil {
			s.Fatal("Failed pressing tab key: ", err)
		}
	}

	// Find send button and then submit the feedback.
	sendButton := nodewith.Name("Send").Role(role.Button).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(sendButton)(ctx); err != nil {
		s.Fatal("Failed to submit feedback: ", err)
	}

	// Verify confirmation page title exists.
	confirmationPageTitle := nodewith.Name("Thanks for your feedback").Role(
		role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(confirmationPageTitle)(ctx); err != nil {
		s.Fatal("Failed to find confirmation page title: ", err)
	}

	s.Log("Waiting enough time for feedback report to be written")
	testing.Sleep(ctx, time.Minute)

	// Read feedback report content.
	content, err := ioutil.ReadFile(reportPath)
	if err != nil {
		s.Fatal("Failed to read report content: ", err)
	}

	actualContent := strings.ToValidUTF8(string(content), "")
	expectedEmail := selectedEmail

	// Verify feedback report contains email based on user selection.
	if s.Param().(reportContainsEmailParam).includeEmail {
		if !strings.Contains(actualContent, expectedEmail) {
			s.Fatalf("Expected email %s does not exist", expectedEmail)
		}
	} else {
		if strings.Contains(actualContent, expectedEmail) {
			s.Fatalf("Unxpected email %s does exist", expectedEmail)
		}
	}

}
