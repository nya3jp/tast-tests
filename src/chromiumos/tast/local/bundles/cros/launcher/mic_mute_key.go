// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: MicMuteKey,
		Desc: "Tests to ensure that the microphone mute key properly mutes/unmutes the microphone",
		Contacts: []string{
			"cros-system-ui-eng@google.com",
			"chromeos-sw-engprod@google.com",
			"jamescook@chromium.org",
		},
		// TODO(https://crbug.com/1255265): Remove "informational" once stable.
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

func MicMuteKey(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("ProductivityLauncher"))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	//tconn, err := cr.TestAPIConn(ctx)
	//if err != nil {
	//	s.Fatal("Failed to connect Test API: ", err)
	//}
	//defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	//ui := uiauto.New(tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to connect to cras: ", err)
	}

	// There is no way to query which device is used by CRAS now. However, the
	// PCM name of internal mic is still correct, we can always run test on the
	// internal mic until there is a method to get the correct device name.
	// See b/142910355 for more details.
	if err := cras.SetActiveNodeByType(ctx, "INTERNAL_MIC"); err != nil {
		s.Fatal("Failed to set internal mic active: ", err)
	}

	s.Log("Muting the microphone")
	//if err := kb.Accel(ctx, "VolumeUp"); err != nil {
	//if err := kb.Accel(ctx, "Search"); err != nil {
	if err := kb.Accel(ctx, "Shift+Ctrl+Alt+M"); err != nil {
		s.Fatal("Failed to mute the microphone: ", err)
	}
	//testing.Sleep(ctx, time.Second)
	//s.Fatal("Dump")

	// Set timeout to duration + 1s, which is the time buffer to complete the normal execution.
	const duration = 5 // second
	runCtx, cancel := context.WithTimeout(ctx, (duration+1)*time.Second)
	defer cancel()

	out, err := testexec.CommandContext(
		runCtx, "cras_test_client").Output()
	s.Log("out = ", out)
	//if regexp.Match("*Mut*", out) {
	//	s.Log(out)
	//}

	//if err := ui.WaitUntilExists(nodewith.Name("Microphone is off")); err != nil {
	//if err := ui.WaitUntilExists(nodewith.Role(role.Slider)); err != nil {
	//if err := ui.WaitUntilExists(nodewith.Role("MicGainSliderView")); err != nil {
	//slider := nodewith.ClassName("MicGainSliderView")
	//s.Log("slider = ", slider)
	//endPoint := coords.NewPoint(0, 0)
	//if err := ui.WaitForEvent(slider, event.Alert, mouse.Move(tconn, endPoint.Add(coords.Point{X: 1, Y: 0}), time.Second))(ctx); err != nil {
	//	s.Fatal("Failed to mute the microphone: ", err)
	//}
	//defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree")

	/* 	s.Log("Un-muting the microphone");
	   	if err := kb.Accel(ctx, "Shift+Ctrl+Alt+X"); err != nil {
	   		s.Fatal("Failed to un-mute the microphone: ", err)
	   	} */
}
