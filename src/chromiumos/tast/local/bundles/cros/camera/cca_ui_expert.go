// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCAUIExpert,
		Desc:         "Opens CCA and verifies the expert options",
		Contacts:     []string{"kaihsien@google.com", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", caps.BuiltinCamera},
		Data:         []string{"cca_ui.js"},
		Pre:          chrome.LoggedIn(),
	})
}

func CCAUIExpert(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*chrome.Chrome)

	app, err := cca.New(ctx, cr, []string{s.DataPath("cca_ui.js")})
	if err != nil {
		s.Fatal("Failed to open CCA: ", err)
	}
	defer app.Close(ctx)

	if err := app.WaitForVideoActive(ctx); err != nil {
		s.Fatal("Preview is inactive after launching app: ", err)
	}
	s.Log("Preview started")

	if ok, err := app.ActivateExpertMode(ctx); err != nil {
		s.Error("Expert mode button cannot be activated: ", err)
	} else if !ok {
		s.Error("Expert mode button not showing")
	}
}
