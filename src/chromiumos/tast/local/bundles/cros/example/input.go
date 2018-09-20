package example

import (
	"time"

	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Input,
		Desc: "Generates input",
		Attr: []string{"informational"},
	})
}

func Input(s *testing.State) {
	ew := input.Keyboard(s.Context())
	ew.Event(time.Now(), input.EV_KEY, input.KEY_LEFTCTRL, 1)
	ew.Sync(time.Now())
	ew.Event(time.Now(), input.EV_KEY, input.KEY_T, 1)
	ew.Sync(time.Now())
	ew.Event(time.Now(), input.EV_KEY, input.KEY_T, 0)
	ew.Sync(time.Now())
	ew.Event(time.Now(), input.EV_KEY, input.KEY_LEFTCTRL, 0)
	ew.Sync(time.Now())
	if err := ew.Close(); err != nil {
		s.Fatal("Failed to inject events: ", err)
	}
}
