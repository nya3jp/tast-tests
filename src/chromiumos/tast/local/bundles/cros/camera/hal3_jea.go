package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/bundles/cros/video/lib/caps"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3JEA,
		Desc:         "Verifies Jpeg encode accelerator works in USB HALv3",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"camera_hal3", caps.USBCamera, caps.HWEncodeJPEG},
	})
}

// HAL3JEA verifies Jpeg encode accelerator works in USB HALv3.
func HAL3JEA(ctx context.Context, s *testing.State) {
	hal3.RunTest(ctx, s, hal3.TestConfig{
		CameraHals:     []string{"usb"},
		GtestFilter:    "*/Camera3SimpleStillCaptureTest.TakePictureTest/0",
		ForceJpegHwEnc: true,
	})
}
