package camera

import (
	"context"

	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3Frame,
		Desc:         "Verifies camera frame function with HAL3 interface",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"arc_camera3"},
	})
}

// HAL3Frame verifies camera frame function with HAL3 interface.
func HAL3Frame(ctx context.Context, s *testing.State) {
	hal3.RunTest(ctx, s, hal3.TestConfig{
		GtestFilter: "Camera3FrameTest/*",
	})
}
