package arc

import (
  "context"
  //"encoding/json"
  //"math"
  "strconv"
  "chromiumos/tast/common/testexec"
  //"chromiumos/tast/errors"
  "chromiumos/tast/local/android/ui"
  "chromiumos/tast/local/arc"
  "chromiumos/tast/common/perf"
  //"chromiumos/tast/local/input"
  "chromiumos/tast/testing"
)

func init() {
  testing.AddTest(&testing.Test{
    Func:         ElapsedTime,
    Desc:         "Checks elapsed time during full-volume media scan",
    Contacts:     []string{"risan@chromium.org", "youkichihosoi@chromium.org", "arc-storage@google.com"},
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

func getElapsedTimeData(ctx context.Context, d *ui.Device) (float64, error) {
  view := d.Object(ui.ID("org.chromium.arc.testapp.elapsedtime:id/file_content"))
  text, err := view.GetText(ctx)
  if err != nil {
    return 0, err
  }
  time, err := strconv.ParseFloat(text, 64)
  if err != nil {
    return 0, err
  }
  return time, nil
}

func ElapsedTime(ctx context.Context, s *testing.State) {
  a := s.FixtValue().(*arc.PreData).ARC
  cr := s.FixtValue().(*arc.PreData).Chrome
  d := s.FixtValue().(*arc.PreData).UIDevice
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

  if err := a.Command(ctx, "sm", "unmount", "emulated;0").Run(testexec.DumpLogOnError); err != nil {
    s.Fatal("Failed to unmount sdcard: ", err)
  }
  defer act.Close()

  if err := a.Command(ctx, "sm", "mount", "emulated;0").Run(testexec.DumpLogOnError); err != nil {
    s.Fatal("Failed to mount sdcard: ", err)
  }
  defer act.Close()

  time, err := getElapsedTimeData(ctx, d)
  if err != nil {
    s.Fatal("Failed to get data from app UI: ", err)
  }
  defer act.Close()

  perfValues := perf.NewValues()
  perfValues.Set(perf.Metric{
    Name: "mediaScanTime",
    Unit: "msec",
    Direction: perf.SmallerIsBetter,
  }, time)

  if err := perfValues.Save(s.OutDir()); err != nil {
    s.Fatal("Failed saving perf data: ", err)
  }
}


