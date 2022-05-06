// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ossettings

import (
	"context"
	"fmt"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
)

// AddFakeVPNSetting adds a fake VPN which is not functional.
// It assumes user is currently on the OS setting page.
func (s *OSSettings) AddFakeVPNSetting() uiauto.Action {
	serviceName := "fake"
	serverHostName := "server"
	userName := "user"
	password := "password"

	ui := uiauto.New(s.tconn)
	return uiauto.Combine("add fake VPN",
		ui.LeftClick(nodewith.Name("Add network connection").Role(role.Button)),
		ui.LeftClick(nodewith.NameContaining("Add built-in VPN").Role(role.Button)),
		// Fill necessary VPN settings.
		func(ctx context.Context) error {
			kb, err := input.Keyboard(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to create keyboard event writer")
			}
			defer kb.Close()

			inputTextField := func(name, text string) uiauto.Action {
				return uiauto.Combine(fmt.Sprintf("Type %q into %s field", name, text),
					ui.FocusAndWait(nodewith.Name(name).Role(role.TextField)),
					kb.TypeAction(text),
				)
			}
			return uiauto.Combine("fill VPN settings",
				inputTextField("Service name", serviceName),
				inputTextField("Server hostname", serverHostName),
				inputTextField("Username", userName),
				inputTextField("Password", password),
			)(ctx)
		},
		ui.LeftClick(nodewith.Name("Connect").Role(role.Button)),
	)
}
