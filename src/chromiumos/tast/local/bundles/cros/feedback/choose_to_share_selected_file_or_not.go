// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package feedback

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	fa "chromiumos/tast/local/chrome/uiauto/feedbackapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ChooseToShareSelectedFileOrNot,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verify user can choose to share selected file or not",
		Contacts: []string{
			"wangdanny@google.com",
			"zhangwenyu@google.com",
			"xiangdongkong@google.com",
			"cros-feedback-app@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{fa.PngFile},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Name: "share_selected_file",
			Val:  true,
		}, {
			Name: "not_share_selected_file",
			Val:  false,
		}},
	})
}

// ChooseToShareSelectedFileOrNot verifies user can choose to share selected file
// or not by checking or unchecking the attach file checkbox.
func ChooseToShareSelectedFileOrNot(ctx context.Context, s *testing.State) {
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

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}

	// Clean up in the end.
	defer func() {
		if err := os.RemoveAll(fa.ReportPath); err != nil {
			s.Log("Failed to remove feedback report: ", err)
		}

		files, err := ioutil.ReadDir(downloadsPath)
		if err != nil {
			s.Log("Failed to read files in Downloads: ", err)
		} else {
			for _, f := range files {
				path := filepath.Join(downloadsPath, f.Name())
				if err := os.RemoveAll(path); err != nil {
					s.Logf("Failed to RemoveAll(%q)", path)
				}
			}
		}
	}()

	ui := uiauto.New(tconn).WithTimeout(20 * time.Second)
	shareSelectedFile := s.Param().(bool)

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

	// Uncheck the attach file checkbox if needed.
	if !shareSelectedFile {
		checkboxContainer := nodewith.Name("Attach file").Role(role.GenericContainer)
		checkbox := nodewith.Role(role.CheckBox).Ancestor(checkboxContainer)
		if err := uiauto.Combine("Uncheck the attach file checkbox",
			ui.DoDefault(checkbox),
			ui.WaitUntilExists(checkbox.Attribute("checked", "false")),
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

	actualContent := strings.ToValidUTF8(string(content), "")
	fileExist := strings.Contains(actualContent, fileName)

	// Verify feedback report contains selected file based on user selection.
	if shareSelectedFile {
		if !fileExist {
			s.Fatal("Failed to verify selected file is in the report")
		}
	} else {
		if fileExist {
			s.Fatal("Failed to verify selected file is not in the report")
		}
	}
}
