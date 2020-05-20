// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package meta

import (
	"context"

	"chromiumos/tast/remote/bundles/mtbf/meta/common"
	"chromiumos/tast/remote/bundles/mtbf/meta/tastrun"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:     MTBF054ModesFallback,
		Desc:     "Switch to portrait mode. Verifies after switch camera, mode selector will auto fallback to photo mode",
		Contacts: []string{"xliu@cienet.com"},
		Attr:     []string{"group:mainline", "informational"},
	})
}

// MTBF054ModesFallback test on portrait mode capable device (e.g. nocturne)
//  1. Switch to portrait mode
//  2. Plugin usb external camera
//  3. Switch to external camera while staying in portrait mode
//  4. The portrait mode icon should disappear and mode selector will auto fallback to photo mode.
func MTBF054ModesFallback(ctx context.Context, s *testing.State) {
	flags := common.GetFlags(s)
	retryErrList := []string{"[ERR-4701]"}
	testNames := []string{
		"usbc.USBControl.off",
		"camera.MTBF054ASwitchToPortraitMode",
		"usbc.USBControl.on",
		"camera.MTBF054BSwitchCameraAndVerification",
	}
	if mtbferr := tastrun.RunTestWithRelogin(ctx, s, flags, testNames, retryErrList); mtbferr != nil {
		s.Fatal(mtbferr)
	}
}
