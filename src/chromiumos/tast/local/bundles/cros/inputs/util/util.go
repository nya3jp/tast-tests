// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// CandidatesFinder is the finder for candidates in the IME candidates window.
var CandidatesFinder = nodewith.Role(role.ImeCandidate)

// InputEval is a data structure to define common input function and expected out.
type InputEval struct {
	TestName     string
	InputFunc    uiauto.Action
	ExpectedText string
}

// FieldInputEval is a data structure to define common input function and expected out on certain input field.
type FieldInputEval struct {
	InputField   testserver.InputField
	InputFunc    uiauto.Action
	ExpectedText string
}

// WaitForFieldTextToBe returns an action checking whether the input field value equals given text.
func WaitForFieldTextToBe(tconn *chrome.TestConn, finder *nodewith.Finder, expectedText string) uiauto.Action {
	ui := uiauto.New(tconn)
	return ui.Retry(10, func(ctx context.Context) error {
		nodeInfo, err := ui.Info(ctx, finder)
		if err != nil {
			return err
		} else if nodeInfo.Value != expectedText {
			return errors.Errorf("unexpected user name: got %s; want %s", nodeInfo.Value, expectedText)
		}
		return nil
	})
}

// GetCandidateText returns a function that returns the candidate text in the specified position in the candidates window.
func GetCandidateText(ctx context.Context, tconn *chrome.TestConn, index int) func() (string, error) {
	return func() (string, error) {
		ui := uiauto.New(tconn)

		if err := ui.WaitUntilExists(CandidatesFinder.First())(ctx); err != nil {
			return "", err
		}

		candidates, err := ui.NodesInfo(ctx, CandidatesFinder)
		if err != nil {
			return "", err
		}

		return candidates[index].Name, nil
	}
}
