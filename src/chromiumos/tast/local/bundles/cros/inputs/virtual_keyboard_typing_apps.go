package inputs

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/vkb"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         VirtualKeyboardTypingApps,
		Desc:         "Checks that the virtual keyboard works in apps",
		Contacts:     []string{"essential-inputs-team@google.com"},
		Attr:         []string{"group:mainline", "group:input-tools", "informational"},
		SoftwareDeps: []string{"chrome", "google_virtual_keyboard"},
	})
}

func VirtualKeyboardTypingApps(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.VKEnabled(), chrome.ExtraArgs("--force-tablet-mode=touch_view"))
	if err != nil {
		s.Fatal(err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	vkbCtx := vkb.NewContext(cr, tconn)

	if err := uiauto.Combine("Show VKB", vkbCtx.ClickUntilVKShown(nodewith.Role(role.TextField)), vkbCtx.WaitForDecoderEnabled(true), vkbCtx.TapKey("a"))(ctx); err != nil {
		s.Fatal(err)
	}
}
