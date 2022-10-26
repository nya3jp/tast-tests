// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/proto"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	fpb "chromiumos/tast/local/bundles/cros/feedback/proto"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/localstate"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	fa "chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

const histogramsZip = "histograms.zip"
const systemLogsZip = "system_logs.zip"

func init() {
	testing.AddTest(&testing.Test{
		Func:         VerifyEssentialData,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify if essential data is in the report in different conditions",
		Contacts: []string{
			"wangdanny@google.com",
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Fixture:      "chromeLoggedInWithOsFeedbackSaveReportToLocalForE2ETesting",
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{fa.PngFile},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "share_data",
			Val:  true,
		}, {
			Name: "not_share_data",
			Val:  false,
		}},
	})
}

// VerifyEssentialData verifies if essential data is
// in the report in different conditions. Description should always be
// included in the report. Screenshot should be included in the report
// when the screenshot checkbox is checked. Diagnostic data should be included
// in the report when the Diagnostic data checkbox is checked. Email should be
// included in the report by default. Attachment should be included in the report
// when the attach file checkbox is checked.
func VerifyEssentialData(ctx context.Context, s *testing.State) {
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

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}

	cleanUp := func() {
		if err := os.RemoveAll(fa.ReportPath); err != nil {
			s.Error("Failed to remove feedback report: ", err)
		}

		files, err := ioutil.ReadDir(downloadsPath)
		if err != nil {
			s.Error("Failed to read files in Downloads: ", err)
		} else {
			for _, f := range files {
				path := filepath.Join(downloadsPath, f.Name())
				if err := os.RemoveAll(path); err != nil {
					s.Errorf("Failed to RemoveAll(%v): %v", path, err)
				}
			}
		}
	}

	// Clean up at both the beginning and the end to make sure
	// the path is clean.
	cleanUp()
	defer cleanUp()

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)
	shareData, castResult := s.Param().(bool)
	if !castResult {
		s.Fatal("Failed to cast param val to bool")
	}

	// Copy the file to Downloads for uploading purpose.
	fileName := fa.PngFile
	if err := fsutil.CopyFile(
		s.DataPath(fileName), filepath.Join(downloadsPath, fileName)); err != nil {
		s.Fatal("Failed to copy file to Downloads: ", err)
	}

	// Launch feedback app and go to share data page.
	feedbackRootNode, err := fa.LaunchAndGoToShareDataPage(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch feedback app and navigating to share data page: ", err)
	}

	// Find add file button and click.
	addFileButton := nodewith.NameContaining("Add file").Role(
		role.Button).Ancestor(feedbackRootNode)
	if err := ui.DoDefault(addFileButton)(ctx); err != nil {
		s.Fatal("Failed to click add file button: ", err)
	}

	// Open Downloads dir and select the png file to upload.
	if err := uiauto.Combine("Open Downloads dir and select PNG file",
		ui.LeftClick(nodewith.Name("Downloads").Role(role.TreeItem)),
		ui.LeftClick(nodewith.NameContaining(fileName).Role(role.StaticText).First()),
		ui.LeftClick(nodewith.Name("Open").Role(role.Button)),
	)(ctx); err != nil {
		s.Fatal("Failed to open Downloads dir and select PNG file: ", err)
	}

	// Verify the uploaded png file exists.
	fileFinder := nodewith.NameContaining(fileName).Role(
		role.StaticText).Ancestor(feedbackRootNode)
	if err := ui.WaitUntilExists(fileFinder)(ctx); err != nil {
		s.Fatal("Failed to find png file: ", err)
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

	if shareData {
		// Check the screenshot checkbox.
		checkbox := nodewith.Role(role.CheckBox).Ancestor(feedbackRootNode).First()
		if err := uiauto.Combine("Verify checkbox is unchecked and click it",
			ui.WaitUntilExists(checkbox.Attribute("checked", "false")),
			ui.DoDefault(checkbox),
			ui.WaitUntilExists(checkbox.Attribute("checked", "true")),
		)(ctx); err != nil {
			s.Fatal("Failed to change the checkbox state: ", err)
		}
	} else {
		// Uncheck the share diagnostic data checkbox.
		diagnosticDataCheckboxContainer := nodewith.Name("Send system and app info and metrics").Role(
			role.GenericContainer).Ancestor(feedbackRootNode)
		diagnosticDataCheckbox := nodewith.Role(role.CheckBox).Ancestor(diagnosticDataCheckboxContainer)
		if err := uiauto.Combine("Uncheck the share diagnostic data checkbox",
			ui.DoDefault(diagnosticDataCheckbox),
			ui.WaitUntilExists(diagnosticDataCheckbox.Attribute("checked", "false")),
		)(ctx); err != nil {
			s.Fatal("Failed to uncheck the share diagnostic data checkbox: ", err)
		}

		// Choose not to include email.
		emailDropdown := nodewith.Name("Select email").Role(role.ListBox)
		dontIncludeEmailOption := nodewith.Name("anonymous user").Role(role.ListBoxOption)
		if err := uiauto.Combine("choose not to include Email",
			ui.LeftClickUntil(emailDropdown, ui.WithTimeout(
				2*time.Second).WaitUntilExists(dontIncludeEmailOption)),
			ui.LeftClick(dontIncludeEmailOption),
		)(ctx); err != nil {
			s.Fatal("Failed to choose not include Email: ", err)
		}

		// Uncheck the attach file checkbox.
		attachFileCheckboxContainer := nodewith.Name("Attach file").Role(role.GenericContainer)
		attachFileCheckbox := nodewith.Role(role.CheckBox).Ancestor(attachFileCheckboxContainer)
		if err := uiauto.Combine("Uncheck the attach file checkbox",
			ui.DoDefault(attachFileCheckbox),
			ui.WaitUntilExists(attachFileCheckbox.Attribute("checked", "false")),
		)(ctx); err != nil {
			s.Fatal("Failed to uncheck the attach file checkbox: ", err)
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
		content, err = ioutil.ReadFile(fa.ReportPath)
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

	// Verify if description is in the report.
	reportDescription := report.GetCommonData().GetDescription()
	if reportDescription != fa.IssueText {
		s.Fatal("Failed to get correct report description")
	}

	// Verify feedback report contains screenshot based on user selection.
	reportScreenshot := report.GetScreenshot()
	if shareData {
		if reportScreenshot == nil {
			s.Fatal("Failed to include the screenshot in the report")
		}
	} else {
		if reportScreenshot != nil {
			s.Fatal("Failed to not include the screenshot in the report")
		}
	}

	// Verify feedback report contains system_log.zip and histograms.zip based on user selection.
	histogramsExists := false
	systemLogsExists := false
	productSpecificBinaryData := report.GetProductSpecificBinaryData()
	for _, element := range productSpecificBinaryData {
		if element.GetName() == histogramsZip {
			histogramsExists = true
		}
		if element.GetName() == systemLogsZip {
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

	// Verify feedback report contains email based on user selection.
	userEmail := report.GetCommonData().GetUserEmail()
	expectedEmail := selectedEmail
	if shareData {
		if userEmail != selectedEmail {
			s.Fatalf("Expected email %s does not exist", expectedEmail)
		}
	} else {
		if userEmail != "" {
			s.Fatalf("Unexpected email %s does exist", expectedEmail)
		}
	}

	// Verify feedback report contains selected file based on user selection.
	// File will be compressed to a zip file.
	zipFile := fileName + ".zip"
	fileExist := false
	for _, element := range productSpecificBinaryData {
		if element.GetName() == zipFile {
			fileExist = true
			break
		}
	}
	if shareData {
		if !fileExist {
			s.Fatal("Failed to verify selected file is in the report")
		}
	} else {
		if fileExist {
			s.Fatal("Failed to verify selected file is not in the report")
		}
	}
}
