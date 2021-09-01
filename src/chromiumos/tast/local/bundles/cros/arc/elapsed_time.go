package arc

import (
  "context"
  //"encoding/json"
  //"math"

  //"chromiumos/tast/errors"
  //"chromiumos/tast/local/android/ui"
  "chromiumos/tast/local/arc"
  //"chromiumos/tast/local/input"
  "chromiumos/tast/testing"
)

func init() {
  testing.AddTest(&testing.Test{
    Func:         ElapsedTime,
    Desc:        "Checks gamepad support works on Android",
    Contacts:     []string{"tetsui@chromium.org", "arc-framework+tast@google.com"},
    Attr:         []string{"group:mainline"},
    SoftwareDeps: []string{"chrome"},
    Fixture:      "arcBooted",
    Params: []testing.Param{{
      ExtraSoftwareDeps: []string{"android_p"},
    }, {
      Name:              "vm",
      ExtraSoftwareDeps: []string{"android_vm"},
    }},
  })
}

func ElapsedTime(ctx context.Context, s *testing.State) {
  a := s.FixtValue().(*arc.PreData).ARC
  cr := s.FixtValue().(*arc.PreData).Chrome
  //d := s.FixtValue().(*arc.PreData).UIDvice
  tconn, err := cr.TestAPIConn(ctx)
  const (
    apk = "ArcElapsedTimeTest.apk"
    pkg = "org.chromium.arc.testapp.elapsedtime"
    cls = "org.chromium.arc.testapp.elapsedtime.MainActivity"
  )

  if err := a.Install(ctx, arc.APKPath(apk)); err != nil {
    s.Fatal("Failed installing app: ", err)
  }

  act, err := arc.NewActivity(a, pkg, cls)
  if err != nil {
    s.Fatal("Failed to create a new activity: ", err)
  }
  defer act.Close()

  if err := act.Start(ctx, tconn); err != nil {
    s.Fatal("Failed to start the activity: ", err)
  }
  defer act.Close()
}


