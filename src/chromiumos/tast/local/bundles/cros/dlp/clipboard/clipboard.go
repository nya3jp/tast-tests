// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package clipboard contains functionality shared by tests that
// exercise Clipboard restrictions of DLP.
package clipboard

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
)

// Upper and lower bounds of the number of words to check between the pasted and copied strings.
const (
	CheckWordsMax = 10
	CheckWordsMin = 1
)

// CheckGreyPasteNode checks if greyed paste node exists.
func CheckGreyPasteNode(ctx context.Context, ui *uiauto.Context) error {
	pasteNode := nodewith.Name("Paste Ctrl+V").Role(role.MenuItem)
	pasteActiveNode := nodewith.Name("Paste Ctrl+V").Role(role.MenuItem).State("focusable", true)

	if err := uiauto.Combine("Check paste node greyed ",
		ui.WaitUntilExists(pasteNode),
		ui.WaitUntilGone(pasteActiveNode))(ctx); err != nil {
		return errors.Wrap(err, "failed to check paste node greyed")
	}

	return nil
}

// CheckClipboardBubble checks if clipboard restriction bubble exists.
func CheckClipboardBubble(ctx context.Context, ui *uiauto.Context, url string) error {
	// Message name - IDS_POLICY_DLP_CLIPBOARD_BLOCKED_ON_PASTE
	bubbleClass := nodewith.ClassName("ClipboardBlockBubble")
	bubbleButton := nodewith.Name("Got it").Role(role.Button).Ancestor(bubbleClass)
	messageBlocked := "Pasting from " + url + " to this location is blocked by administrator policy. Learn more"
	bubble := nodewith.Name(messageBlocked).Role(role.StaticText).Ancestor(bubbleClass)

	if err := uiauto.Combine("find bubble ",
		ui.WaitUntilExists(bubbleButton),
		ui.WaitUntilExists(bubble))(ctx); err != nil {
		return errors.Wrap(err, "failed to check for notification bubble's existence")
	}

	return nil
}

// WarnBubble gets the clipboard warn bubble if it exists.
func WarnBubble(ctx context.Context, ui *uiauto.Context, url string) (*nodewith.Finder, error) {
	// Message name - IDS_POLICY_DLP_CLIPBOARD_WARN_ON_PASTE
	bubbleClass := nodewith.ClassName("ClipboardWarnBubble")
	cancelButton := nodewith.Name("Cancel").Role(role.Button).Ancestor(bubbleClass)
	pasteButton := nodewith.Name("Paste anyway").Role(role.Button).Ancestor(bubbleClass)
	message := "Pasting from " + url + " to this location is not recommended by administrator policy. Learn more"
	bubble := nodewith.Name(message).Role(role.StaticText).Ancestor(bubbleClass)

	if err := uiauto.Combine("find bubble ",
		ui.WaitUntilExists(cancelButton),
		ui.WaitUntilExists(pasteButton),
		ui.WaitUntilExists(bubble))(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to check for notification bubble's existence")
	}

	return bubbleClass, nil
}

// GetClipboardContent retrieves the current clipboard content.
func GetClipboardContent(ctx context.Context, tconn *chrome.TestConn) (string, error) {
	var clipData string
	if err := tconn.Eval(ctx, `tast.promisify(chrome.autotestPrivate.getClipboardTextData)()`, &clipData); err != nil {
		return "", errors.Wrap(err, "failed to get clipboard content")
	}
	return clipData, nil
}

func checkNumWordsAndRetrieveContentNode(ctx context.Context, ui *uiauto.Context, content string) (*nodewith.Finder, error) {
	// Slicing the string to get the first 10 words or less in a single line.
	// Since pasted string in search box will be in single line format.
	words := strings.Fields(content)
	numSampleWords := len(words)
	if numSampleWords > CheckWordsMax {
		numSampleWords = CheckWordsMax
	} else if numSampleWords < CheckWordsMin {
		return nil, errors.New("sample text has too few words")
	}
	content = strings.Join(words[:numSampleWords], " ")

	contentNode := nodewith.NameStartingWith(content).Role(role.InlineTextBox).State(state.Editable, true).First()

	return contentNode, nil
}

// CheckPastedContent checks if a certain string appears in the search box.
func CheckPastedContent(ctx context.Context, ui *uiauto.Context, content string) error {

	contentNode, err := checkNumWordsAndRetrieveContentNode(ctx, ui, content)

	if err != nil {
		return err
	}

	if err := ui.WaitUntilExists(contentNode)(ctx); err != nil {
		return errors.Wrap(err, "failed to check for pasted content")
	}

	return nil
}

// CheckContentIsNotPasted checks that a certain strings do not appear in the search box
func CheckContentIsNotPasted(ctx context.Context, ui *uiauto.Context, content string) error {

	contentNode, err := checkNumWordsAndRetrieveContentNode(ctx, ui, content)

	if err != nil {
		return err
	}

	if err := ui.EnsureGoneFor(contentNode, 5*time.Second)(ctx); err != nil {
		return errors.Wrap(err, "failed to check for pasted content")
	}

	return nil

}

// WarnPolicy returns a clipboard dlp policy warning when clipboard content is copied and pasted from source to destination.
func WarnPolicy(source, destination string) []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Warn about copy and paste of confidential content in restricted destination",
				Description: "User should be warned when coping and pasting confidential content in restricted destination",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						source,
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListValueDestinations{
					Urls: []string{
						destination,
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "CLIPBOARD",
						Level: "WARN",
					},
				},
			},
		},
	},
	}
}

// BlockPolicy returns a clipboard dlp policy warning when clipboard content is copied and pasted from source to destination.
func BlockPolicy(source, destination string) []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable copy and paste of confidential content in restricted destination",
				Description: "User should not be able to copy and paste confidential content in restricted destination",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						source,
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListValueDestinations{
					Urls: []string{
						destination,
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "CLIPBOARD",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}
}
