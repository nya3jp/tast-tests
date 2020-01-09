package example

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/local/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Instrument,
		Desc:         "Runs Android instrumentation tests",
		Attr:         []string{"disabled"},
		SoftwareDeps: []string{"chrome", "android_all_both"},
		Timeout:      3 * time.Minute,
		Data:         []string{"LocalAttachedEspressoTests.apk"},
		Pre:          arc.Booted(),
	})
}

func Instrument(ctx context.Context, s *testing.State) {
	a := s.PreValue().(arc.PreData).ARC

	s.Log("Installing APK")
	if err := a.Install(ctx, s.DataPath("LocalAttachedEspressoTests.apk")); err != nil {
		s.Fatal("Failed to install APK: ", err)
	}

	s.Log("Running test")
	f, err := os.Create(filepath.Join(s.OutDir(), "out.txt"))
	if err != nil {
		s.Fatal("Failed to create output file: ", err)
	}
	defer f.Close()
	cmd := a.Command(ctx, "am", "instrument", "-w", "com.google.android.apps.play.store.espressotests.external.core.navigate.apps/com.google.android.apps.common.testing.testrunner.Google3InstrumentationTestRunner")
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(); err != nil {
		s.Error("am instrument failed (see out.txt): ", err)
	}
}

