package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3JDA,
		Desc:         "Verifies Jpeg decode accelerator works in USB HALv3",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"camera_hal3", caps.USBCamera, caps.HWDecodeJPEG},
	})
}

// HAL3JDA verifies Jpeg decode accelerator works in USB HALv3.
func HAL3JDA(ctx context.Context, s *testing.State) {
	hal3.RunTest(ctx, s, hal3.TestConfig{
		CameraHals:     []string{"usb"},
		GtestFilter:    "*/Camera3SingleFrameTest.GetFrame/0",
		ForceJpegHwDec: true,
	})
}
