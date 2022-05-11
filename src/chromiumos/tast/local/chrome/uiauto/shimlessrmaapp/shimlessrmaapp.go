// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// TODO: Refactor the file name. b:228780486

// Package shimlessrmaapp contains drivers for controlling the ui of Shimless RMA SWA.
package shimlessrmaapp

import (
	"context"
	"os"
	"os/user"
	"strconv"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

var rootNode = nodewith.Name(apps.ShimlessRMA.Name).Role(role.Window)

var nextButton = nodewith.Name("Next >").Role(role.Button)
var cancelButton = nodewith.Name("Cancel").Role(role.Button)

const pollingInterval = 2 * time.Second
const pollingTimeout = 15 * time.Second
const waitUITimeout = 20 * time.Second

// Titles of pages.
// role.Heading nodes with this text are found to confirm a page loaded.
const (
	welcomePageTitle  = "Chromebook repair"
	updateOSPageTitle = "Make sure Chrome OS is up to date"
)

const (
	stateFile = "/mnt/stateful_partition/unencrypted/rma-data/state"
)

// RMAApp represents an instance of the Shimless RMA App.
type RMAApp struct {
	ui    *uiauto.Context
	tconn *chrome.TestConn
	// TODO(gavinwill): launched and all support for running the app manually
	// should be removed once the app launch at boot cls land.
	launched bool
}

// CreateEmptyStateFile creates a valid empty state file.
func CreateEmptyStateFile() error {
	return CreateStateFile("{}\n")
}

// CreateStateFile creates a state file with contents |state|.
func CreateStateFile(state string) error {
	uid, err := getRmadUID()
	if err != nil {
		return err
	}

	// Deletes the state file, if it exists.
	RemoveStateFile()

	f, err := os.Create(stateFile)
	if err != nil {
		return err
	}
	defer f.Close()
	l, err := f.WriteString(state)
	if err != nil || l < 3 {
		return err
	}
	if err := f.Chown(uid, -1); err != nil {
		return err
	}
	return nil
}

// RemoveStateFile deletes the state file, if it exists.
func RemoveStateFile() error {
	return os.Remove(stateFile)
}

// Launch launches the Shimless RMA App and returns it.
// An error is returned if the app fails to launch.
// TODO(gavinwill): This method and all support for running the app manually
// should be removed once the app launch at boot cls land.
func Launch(ctx context.Context, tconn *chrome.TestConn) (*RMAApp, error) {
	// Launch the Shimless RMA App.
	if err := apps.Launch(ctx, tconn, apps.ShimlessRMA.ID); err != nil {
		return nil, err
	}
	r, err := App(ctx, tconn)
	if err != nil {
		return r, err
	}
	// Find the main Shimless RMA window
	if err := r.ui.WithTimeout(waitUITimeout).WaitUntilExists(rootNode)(ctx); err != nil {
		return nil, err
	}
	r.launched = true
	return r, nil
}

// App returns an existing instance of the Shimless RMA app.
// An error is returned if the app cannot be found.
func App(ctx context.Context, tconn *chrome.TestConn) (*RMAApp, error) {
	// Create a uiauto.Context with default timeout.
	ui := uiauto.New(tconn)
	return &RMAApp{tconn: tconn, ui: ui, launched: false}, nil
}

// Close closes the Shimless RMA App.
func (r *RMAApp) Close(ctx context.Context) error {
	// Close the Shimless RMA App.
	if err := apps.Close(ctx, r.tconn, apps.ShimlessRMA.ID); err != nil {
		return err
	}

	// Wait for window to close.
	return r.ui.WithTimeout(time.Minute).WaitUntilGone(rootNode)(ctx)
}

// WaitForStateFileDeleted returns a function that waits for the state file to be deleted.
func (r *RMAApp) WaitForStateFileDeleted() uiauto.Action {
	return r.waitForFileDeleted(stateFile)
}

// WaitForWelcomePageToLoad returns a function that waits for the Welcome state page to load.
func (r *RMAApp) WaitForWelcomePageToLoad() uiauto.Action {
	return r.WaitForPageToLoad(welcomePageTitle, waitUITimeout)
}

// WaitForUpdateOSPageToLoad returns a function that waits for the Update OS state page to load.
func (r *RMAApp) WaitForUpdateOSPageToLoad() uiauto.Action {
	return r.WaitForPageToLoad(updateOSPageTitle, waitUITimeout)
}

// WaitForPageToLoad returns a function that waits for the a page with title |pageTitle| to load.
func (r *RMAApp) WaitForPageToLoad(pageTitle string, timeout time.Duration) uiauto.Action {
	title := nodewith.Name(pageTitle).Role(role.Heading)
	return r.ui.WithTimeout(timeout).WaitUntilExists(title)
}

// LeftClickNextButton returns a function that clicks the next button.
func (r *RMAApp) LeftClickNextButton() uiauto.Action {
	return r.leftClickButton(nextButton.Visible())
}

// LeftClickCancelButton returns a function that clicks the cancel button.
func (r *RMAApp) LeftClickCancelButton() uiauto.Action {
	return r.leftClickButton(cancelButton.Visible())
}

// LeftClickButton returns a function that clicks a button.
func (r *RMAApp) LeftClickButton(label string) uiauto.Action {
	return r.leftClickButton(nodewith.Name(label).Role(role.Button).Visible())
}

// WaitUntilButtonEnabled returns a function that waits |timeout| for a button to be enabled.
func (r *RMAApp) WaitUntilButtonEnabled(label string, timeout time.Duration) uiauto.Action {
	return r.waitUntilEnabled(nodewith.Name(label).Role(role.Button).Visible(), timeout)
}

// LeftClickRadioButton returns a function that clicks a radio button.
func (r *RMAApp) LeftClickRadioButton(label string) uiauto.Action {
	// TODO(b/230692945): Can we add RadioButton as role?
	radioGroup := nodewith.Role(role.RadioGroup)
	return r.ui.LeftClick(nodewith.Name(label).Ancestor(radioGroup).First())
}

// LeftClickLink returns a function that clicks a link.
func (r *RMAApp) LeftClickLink(label string) uiauto.Action {
	return r.ui.LeftClick(nodewith.Name(label).Role(role.Link).Visible())
}

// RetrieveTextByPrefix returns a text which has a cerntian prefix.
func (r *RMAApp) RetrieveTextByPrefix(ctx context.Context, prefix string) (*uiauto.NodeInfo, error) {
	return r.ui.Info(ctx, nodewith.NameStartingWith(prefix).Role(role.StaticText))
}

// EnterIntoTextInput enters text into text input.
func (r *RMAApp) EnterIntoTextInput(ctx context.Context, textInputName, content string) uiauto.Action {
	keyboard, _ := input.Keyboard(ctx)
	var textInputFinder = nodewith.Role(role.TextField)
	return uiauto.Combine("type keyword to enter content to text input",
		r.ui.LeftClickUntil(textInputFinder, r.ui.WaitUntilExists(textInputFinder.Focused())),
		keyboard.TypeAction(content),
	)
}

func getRmadUID() (int, error) {
	u, err := user.Lookup("rmad")
	if err != nil {
		return -1, err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return -1, err
	}
	return uid, nil
}

func (r *RMAApp) waitUntilEnabled(button *nodewith.Finder, timeout time.Duration) uiauto.Action {
	if r.launched {
		button = button.Ancestor(rootNode)
	}
	return uiauto.Combine("waiting for enabled button",
		r.ui.WithTimeout(waitUITimeout).WaitUntilExists(button),
		func(ctx context.Context) error {
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := r.ui.CheckRestriction(button, restriction.Disabled)(ctx); err == nil {
					return errors.Errorf("Button state %s", restriction.Disabled)
				}
				return nil
			}, &testing.PollOptions{Timeout: timeout, Interval: pollingInterval}); err != nil {
				return errors.Wrap(err, "Button failed to become enabled")
			}
			return nil
		})
}

func (r *RMAApp) leftClickButton(button *nodewith.Finder) uiauto.Action {
	if r.launched {
		button = button.Ancestor(rootNode)
	}
	return uiauto.Combine("waiting to trigger left click on button",
		r.ui.WithTimeout(waitUITimeout).WaitUntilExists(button),
		r.ui.FocusAndWait(button),
		func(ctx context.Context) error {
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := r.ui.CheckRestriction(button, restriction.Disabled)(ctx); err == nil {
					return errors.Errorf("Button state %s", restriction.Disabled)
				}
				return nil
			}, &testing.PollOptions{Timeout: pollingTimeout, Interval: pollingInterval}); err != nil {
				return errors.Wrap(err, "Button failed to become enabled")
			}
			return nil
		},
		r.ui.LeftClick(button),
	)
}

func (r *RMAApp) waitForFileDeleted(fileName string) uiauto.Action {
	return func(ctx context.Context) error {
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if _, err := os.Stat(fileName); err == nil {
				return errors.Errorf("File %s was not deleted", fileName)
			} else if !os.IsNotExist(err) {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: pollingTimeout, Interval: pollingInterval}); err != nil {
			return errors.Wrap(err, "File was not deleted")
		}
		return nil
	}
}
