// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clipboardhistory

import (
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
)

// FindMenu returns a finder which locates the clipboard history menu.
func FindMenu() *nodewith.Finder {
	return nodewith.NameStartingWith("Clipboard history").Role(role.MenuBar)
}
