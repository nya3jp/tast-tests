// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mtbfutil

import (
	"context"
	"time"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ClickSystemTray,
		Desc:         "Click SystemTray",
		Contacts:     []string{"xliu@cienet.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Pre:          chrome.LoginReuse(),
	})
}

// ClickSystemTray open notification center
func ClickSystemTray(ctx context.Context, s *testing.State) {
	var cr = s.PreValue().(*chrome.Chrome)
	conn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeTestConn, err))
	}
	defer conn.Close()

	// System tray maybe covered by app, mouse move to bottom could open system tray
	mouse, err := input.Mouse(ctx)
	if err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeMouse, err))
	}
	defer mouse.Close()
	mouse.Move(0, 2000)
	testing.Sleep(ctx, 1*time.Second)
	clickSystrayJs :=
		`chrome.automation.getDesktop(function cb(desktop) {
			var sysTray = desktop.find({attributes: {className:"UnifiedSystemTray"}});
			sysTray.doDefault();
		});`

	if err := conn.Exec(ctx, clickSystrayJs); err != nil {
		s.Fatal(mtbferrors.New(mtbferrors.ChromeClickSystemTray, err))
	}
	s.Log("Click system tray")
}
