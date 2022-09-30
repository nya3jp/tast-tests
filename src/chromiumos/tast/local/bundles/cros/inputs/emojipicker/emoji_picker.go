// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package emojipicker contains common functions shared by Emoji-picker related tests.
package emojipicker

import (
	"regexp"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// Emoji picker ui node finders.
var (
	RootFinder                    = nodewith.Name("Emoji Picker").Role(role.RootWebArea)
	NodeFinder                    = nodewith.Ancestor(RootFinder)
	SearchFieldFinder             = NodeFinder.NameRegex(regexp.MustCompile(`Search( Emojis)?`)).Role(role.SearchBox)
	RecentUsedHeading             = NodeFinder.Name("Recently used").Role(role.Heading)
	RecentUsedMenu                = nodewith.Role(role.Button).Ancestor(RecentUsedHeading)
	ClearRecentlyUsedButtonFinder = NodeFinder.Name("Clear recently used emojis").Role(role.Button)
)

// NewUICtx creates a new UI context used in emoji picker related tests.
// Emoji picker has too many UI nodes and causes a slow refresh in A11y.
func NewUICtx(tconn *chrome.TestConn) *uiauto.Context {
	return uiauto.New(tconn).WithTimeout(30 * time.Second)
}

// WaitUntilExists returns an action to wait until emoji picker appears.
func WaitUntilExists(tconn *chrome.TestConn) uiauto.Action {
	return NewUICtx(tconn).WaitUntilExists(RootFinder)
}

// WaitUntilGone returns an action to wait until emoji picker disappears.
func WaitUntilGone(tconn *chrome.TestConn) uiauto.Action {
	return NewUICtx(tconn).WaitUntilGone(RootFinder)
}
