// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardFloat,
		Desc:         "Validity check on floating virtual keyboard",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
		Timeout:      5 * 60 * time.Minute,
	})
}

func VirtualKeyboardFloat(ctx context.Context, s *testing.State) {
	// Give 5 seconds to clean up and dump out UI tree.
	for i := 0; i < 10000; i++ {
		testing.ContextLogf(ctx, "TESTING: %d", i)
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
		defer cancel()

		cr, err := chrome.New(ctx, chrome.VKEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view"))
		if err != nil {
			s.Fatal("Failed to start Chrome: ", err)
		}
		defer cr.Close(cleanupCtx)

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			s.Fatal("Failed to create test API connection: ", err)
		}
		defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)
		vkbCtx := vkb.NewContext(cr, tconn)

		testing.Sleep(ctx, 10*time.Second)

		if err := vkbCtx.ShowVirtualKeyboard()(ctx); err != nil {
			s.Fatal("Failed to show VK: ", err)
		}
	}
}
