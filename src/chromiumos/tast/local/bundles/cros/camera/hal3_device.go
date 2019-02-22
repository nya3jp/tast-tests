package camera

import (
	"context"
	"strings"

	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3Device,
		Desc:         "Verifies camera device function with HAL3 interface",
		Contacts:     []string{"shik@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"arc_camera3"},
	})
}

// HAL3Device verifies camera device function with HAL3 interface.
func HAL3Device(ctx context.Context, s *testing.State) {
	// TODO(shik): Replace this by some tast utility function that returns board
	// name directly when available.
	cmd := testexec.CommandContext(ctx, "mosys", "platform", "name")
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to get platform name: ", err)
	}
	name := strings.ToLower(strings.TrimSpace(string(out[:])))

	gtestFilter := "Camera3DeviceTest/*"
	switch name {
	case "scarlet", "nocturne":
		// Skip sensor orientation test for tablets.
		gtestFilter += "-*SensorOrientationTest/*"
	}

	hal3.RunTest(ctx, s, hal3.TestConfig{
		GtestFilter: gtestFilter,
	})
}
