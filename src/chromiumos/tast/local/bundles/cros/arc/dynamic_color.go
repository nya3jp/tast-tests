// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DynamicColor,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ArcSystemUIService changes Settings.Secure",
		Contacts:     []string{"arc-app-dev@google.com, ttefera@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "arcBootedWithoutUIAutomator",
		ServiceDeps:  []string{"tast.cros.nearbyservice.NearbyShareService"}, // Need to find out the arcsystemuiservice equivalent
		Params: []testing.Param{
			{
				Name:              "vm",
				ExtraSoftwareDeps: []string{"android_vm"},
			},
		},
	})
}

// DynamicColor alls function in ArcSystemUIService and checks to see if Android Settings.Secure changed.
func DynamicColor(ctx context.Context, s *testing.State) {
	// fixtData := s.FixtValue().(*assistant.FixtData)
	// cr := fixtData.Chrome

	cr := s.FixtValue().(*arc.PreData).Chrome

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	// s.Fatal("I literally give up: ", err)
	// if err := tconn.Call(ctx, nil, "tast.promisify(chrome.autotestPrivate.sendOverlayColor)"); err != nil {
	var ret bool
	if err := tconn.Eval(ctx, "tast.promisify(chrome.autotestPrivate.sendOverlayColor)()", &ret); err != nil {
		s.Fatal(err, " --> Failed to send overlay color.")
	}
	s.Log(ret, " ~~~Geez this took so long to get")

	a := s.FixtValue().(*arc.PreData).ARC

	cmd := a.Command(ctx, "settings", "get", "secure", "theme_customization_overlay_packages")
	output, err := cmd.Output()
	if err != nil {
		s.Fatal(err, " --> Failed to get secure settings.")
	}
	s.Log(string(output), " ~~~Oh my goodness we did it")
}
