package ui

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BluetoothToggle,
		Desc: "Toggles Bluetooth setting from the login screen",
		Contacts: []string{
			"billyzhao@chromium.org",
		},
		SoftwareDeps: []string{"chrome"},
		Vars:         []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func BluetoothToggle(ctx context.Context, s *testing.State) {
	s.Fatal(s.RequiredVar("ui.signinProfileTestExtensionManifestKey"))
	cr, err := chrome.New(
		ctx,
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)
	tLoginConn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer tLoginConn.Close()
	//tconn, err := cr.TestAPIConn(ctx)
	const pauseDuration = 5 * time.Second
	if err := testing.Sleep(ctx, pauseDuration); err != nil {
		s.Fatal("fail2sleep")
	}

	// Find and click the StatusArea via UI. Clicking it opens the Ubertray.
	params := ui.FindParams{
		ClassName: "ash/StatusAreaWidgetDelegate",
	}
	statusArea, err := ui.FindWithTimeout(ctx, tLoginConn, params, 10*time.Second)
	if err != nil {
		s.Fatal("Failed to find the status area (time, battery, etc.): ", err)
	}
	defer statusArea.Release(ctx)
	if err := statusArea.LeftClick(ctx); err != nil {
		s.Fatal("Failed to click status area: ", err)
	}

	// Confirm that the system tray is open by checking for the "CollapseButton".
	params = ui.FindParams{
		ClassName: "CollapseButton",
	}
	if err := ui.WaitUntilExists(ctx, tLoginConn, params, 10*time.Second); err != nil {
		s.Fatal("Waiting for system tray open failed: ", err)
	}
	if err := testing.Sleep(ctx, pauseDuration); err != nil {
		s.Fatal("fail2sleep")
	}
	// Find the collapse button view bounds.
	bluetoothButton, err := ui.Find(ctx, tLoginConn, ui.FindParams{Name: "Toggle Bluetooth. Bluetooth is on", ClassName: "FeaturePodIconButton"})
	if err != nil {
		bluetoothButton, err = ui.Find(ctx, tLoginConn, ui.FindParams{Name: "Toggle Bluetooth. Bluetooth is off", ClassName: "FeaturePodIconButton"})
		if err != nil {
			s.Fatal("Failed to find the collapse button: ", err)
		}
	}
	defer bluetoothButton.Release(ctx)
	if err := bluetoothButton.LeftClick(ctx); err != nil {
		s.Fatal(err, "failed to click collapse button")
	}

	//if err != nil {
	//	s.Fatal("Failed to create Test API connection: ", err)
	//}
	////defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	//defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tLoginConn)

	//s.Fatal("I would like a UI dump")
}
