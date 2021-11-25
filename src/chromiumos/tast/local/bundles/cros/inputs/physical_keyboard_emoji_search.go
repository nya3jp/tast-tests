// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"

	"chromiumos/tast/local/bundles/cros/inputs/pre"
	"chromiumos/tast/local/bundles/cros/inputs/testserver"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/testing"
)

func init() {

	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboardEmojiSearch,
		Desc:         "Check that emoji search works well",
		Contacts:     []string{"jopalmer@chromium.org", "essential-inputs-team@google.com"},
		Attr:         []string{"group:input-tools"},
		SoftwareDeps: []string{"chrome"},
		Pre:          pre.NonVKClamshell,
		Params: []testing.Param{{
			ExtraAttr: []string{"group:input-tools-upstream"},
			//ExtraHardwareDeps: hwdep.D(hwdep.Model(pre.StableModels...), hwdep.SkipOnModel("kodama", "kefka")),
		}}})
}

func PhysicalKeyboardEmojiSearch(ctx context.Context, s *testing.State) {
	stopRecording := uiauto.RecordVNCVideo(ctx, s)
	defer stopRecording()

	cr := s.PreValue().(pre.PreData).Chrome
	tconn := s.PreValue().(pre.PreData).TestAPIConn
	uc := s.PreValue().(pre.PreData).UserContext

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	its, err := testserver.Launch(ctx, cr, tconn)
	if err != nil {
		s.Fatal("Failed to launch inputs test server: ", err)
	}
	defer its.Close()

	if err := its.InputEmojiWithEmojiPickerSearch(uc, testserver.TextAreaInputField, "melting face", "🫠").Run(ctx); err != nil {
		s.Fatal("Failed to verify emoji picker: ", err)
	}
}
