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

	must := func(err error) {
		if err != nil {
			s.Fatal(err)
		}
	}

	apptest.Run(ctx, s, apk, pkg, cls, func(a *arc.ARC, d *ui.Device) {
		field := d.Object(ui.ID(fieldID))
		must(field.WaitForExists(ctx))
		must(field.Click(ctx))
		must(field.SetText(ctx, ""))

		kb, err := input.Keyboard(ctx)
		if err != nil {
			s.Fatal("Failed to find keyboard", err)
		}
		defer kb.Close()
		must(kb.Type(ctx, keystrokes))

		actual, err := field.GetText(ctx)
		must(err)
		if actual != keystrokes {
			s.Fatal("Expected", keystrokes, "got", actual)
		}
	})
}
