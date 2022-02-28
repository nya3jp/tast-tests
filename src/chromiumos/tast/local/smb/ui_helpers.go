// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package smb

import (
	"context"

	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
)

// AddFileShareAction returns a ui.Action that enters all the inputs required
// for an SMB file share in the corresponding dialog.
// This assumes the SMB file share dialog is already open.
func AddFileShareAction(ui *uiauto.Context, kb *input.KeyboardEventWriter, rememberPassword bool, shareName, username, password string) uiauto.Action {
	return func(ctx context.Context) error {
		UncheckRememberMyPasswordIfRequired := func(ctx context.Context) error {
			if !rememberPassword {
				return uiauto.Combine("uncheck Remember my password",
					kb.AccelAction("Tab"),
					kb.AccelAction("Enter"),
					kb.AccelAction("Tab"),
					kb.AccelAction("Tab"),
				)(ctx)
			}
			return nil
		}

		fileShareURLTextBox := nodewith.Name("File share URL").Role(role.TextField)
		return uiauto.Combine("add secureshare via Files context menu",
			ui.WaitForLocation(fileShareURLTextBox),
			ui.LeftClick(fileShareURLTextBox),
			kb.TypeAction(`\\localhost\`+shareName),
			kb.AccelAction("Tab"), // Tab past share name to Username box.
			kb.AccelAction("Tab"),
			kb.TypeAction(username),
			kb.AccelAction("Tab"), // Tab to the password box.
			kb.TypeAction(password),
			UncheckRememberMyPasswordIfRequired,
			kb.AccelAction("Enter"), // Add the Samba share.
		)(ctx)
	}
}
