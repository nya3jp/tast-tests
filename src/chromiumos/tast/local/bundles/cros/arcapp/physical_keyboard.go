package arcapp

import (
	"context"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/bundles/cros/arcapp/apptest"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PhysicalKeyboard,
		Desc:         "Checks physical keyboard works on Android",
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"android", "chrome_login"},
		Data:         []string{"ArcKeyboardTest.apk"},
		Timeout:      4 * time.Minute,
	})
}

func PhysicalKeyboard(ctx context.Context, s *testing.State) {
	const (
		apk = "ArcKeyboardTest.apk"
		pkg = "org.chromium.arc.testapp.keyboard"
		cls = "org.chromium.arc.testapp.keyboard.MainActivity"

		fieldID = "org.chromium.arc.testapp.keyboard:id/text"

		keystrokes = "google"
	)

	apptest.Run(ctx, s, apk, pkg, cls, func(a *arc.ARC, d *ui.Device) {
		field := d.Object(ui.ID(fieldID))
		if err := field.WaitForExists(ctx); err != nil {
			s.Fatal("Failed to find field: ", err)
		}
		if err := field.Click(ctx); err != nil {
			s.Fatal("Failed to click field: ", err)
		}
		if err := field.SetText(ctx, ""); err != nil {
			s.Fatal("Failed to empty field: ", err)
		}

		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to find keyboard: ", err)
		}
		defer kb.Close()
		if err := kb.Type(ctx, keystrokes); err != nil {
			s.Fatalf("Failed to type %q: %v", keystrokes, err)
		}

		if actual, err := field.GetText(ctx); err != nil {
			s.Fatal("Failed to get text: ", err)
		} else if actual != keystrokes {
			s.Errorf("Got input %q from field after typing %q", actual, keystrokes)
		}
	})
}
