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
		Params: []testing.Param{
			{Name: "1"},
			{Name: "2"},
			{Name: "3"},
			{Name: "4"},
			{Name: "5"},
			{Name: "6"},
			{Name: "7"},
			{Name: "8"},
			{Name: "9"},
			{Name: "10"},
			{Name: "11"},
			{Name: "12"},
			{Name: "13"},
			{Name: "14"},
			{Name: "15"},
			{Name: "16"},
			{Name: "17"},
			{Name: "18"},
			{Name: "19"},
			{Name: "20"},
			{Name: "21"},
			{Name: "22"},
			{Name: "23"},
			{Name: "24"},
			{Name: "25"},
			{Name: "26"},
			{Name: "27"},
			{Name: "28"},
			{Name: "29"},
			{Name: "30"},
			{Name: "31"},
			{Name: "32"},
			{Name: "33"},
			{Name: "34"},
			{Name: "35"},
			{Name: "36"},
			{Name: "37"},
			{Name: "38"},
			{Name: "39"},
			{Name: "40"},
			{Name: "41"},
			{Name: "42"},
			{Name: "43"},
			{Name: "44"},
			{Name: "45"},
			{Name: "46"},
			{Name: "47"},
			{Name: "48"},
			{Name: "49"},
			{Name: "50"},
		},
	})
}

func VirtualKeyboardFloat(ctx context.Context, s *testing.State) {
	// Give 5 seconds to clean up and dump out UI tree.
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

	if err := vkbCtx.ShowVirtualKeyboard()(ctx); err != nil {
		s.Fatal("Failed to show VK: ", err)
	}

}
