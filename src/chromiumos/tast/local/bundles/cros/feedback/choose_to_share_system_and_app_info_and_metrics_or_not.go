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
	fpb "chromiumos/tast/local/bundles/cros/feedback/proto"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

const histograms = "histograms.zip"
const systemLogs = "system_logs.zip"

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChooseToShareSystemAndAppInfoAndMetricsOrNot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify user can share system and app info and metrics or not",
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
		Params: []testing.Param{{
			Name: "share_system_and_app_info_and_metrics",
			Val:  true,
		}, {
			Name: "not_share_system_and_app_info_or_metrics",
			Val:  false,
		}},
	})
}

// ChooseToShareSystemAndAppInfoAndMetricsOrNot verifies user can choose to share system and app info and metrics or not.
func ChooseToShareSystemAndAppInfoAndMetricsOrNot(ctx context.Context, s *testing.State) {
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

	// Clean up in the end.
	defer func() {
		if err := os.RemoveAll(feedbackapp.ReportPath); err != nil {
			s.Log("Failed to remove feedback report: ", err)
		}
	}()

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)
	shareData := s.Param().(bool)

	// Launch feedback app and go to share data page.
	feedbackRootNode, err := feedbackapp.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and navigating to share data page: ", err)
	}

	// Uncheck the share diagnostic data checkbox if needed.
	if !shareData {
		checkboxContainer := nodewith.Name("Send system and app info and metrics").Role(
			role.GenericContainer).Ancestor(feedbackRootNode)
		checkbox := nodewith.Role(role.CheckBox).Ancestor(checkboxContainer)
		if err := uiauto.Combine("Uncheck the share diagnostic data checkbox",
			ui.DoDefault(checkbox),
			ui.WaitUntilExists(checkbox.Attribute("checked", "false")),
		)(ctx); err != nil {
			s.Fatal("Failed to uncheck the share diagnostic data checkbox: ", err)
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

	report := &fpb.ExtensionSubmit{}
	if err = proto.Unmarshal(content, report); err != nil {
		s.Fatal("Failed to parse report: ", err)
	}

	// Verify if system_log.zip and histograms.zip are in the report.
	histogramsExists := false
	systemLogsExists := false
	productSpecificBinaryData := report.GetProductSpecificBinaryData()
	for _, element := range productSpecificBinaryData {
		if element.GetName() == histograms {
			histogramsExists = true
		}
		if element.GetName() == systemLogs {
			systemLogsExists = true
		}
	}

	if shareData {
		if !histogramsExists || !systemLogsExists {
			s.Fatal("Failed to share system and app info and metrics")
		}
	} else {
		if histogramsExists || systemLogsExists {
			s.Fatal("Failed to not share system and app info and metrics")
		}
	}
}
