package patrace

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

const (
	pkgName      = "com.arm.pa.paretrace"
	activityName = ".Activities.RetraceActivity"
)

// RunTrace replays a PATrace (GLES) (https://github.com/ARM-software/patrace)
// in android
//
// apkName: Filename of the apk data
// traceFile: Filename of the trace data
// timeout: Adjust according to the trace file
func RunTrace(ctx context.Context, s *testing.State, apkName string, traceFile string, timeout time.Duration) {
	// Reuse existing ARC and Chrome session.
	a := s.PreValue().(arc.PreData).ARC
	cr := s.PreValue().(arc.PreData).Chrome

	// Create Test API connection.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Switch to Clamshell mode
	tabletModeEnabled, err := ash.TabletModeEnabled(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get tablet mode: ", err)
	}
	if tabletModeEnabled {
		// Be nice and restore tablet mode to its original state on exit.
		defer ash.SetTabletModeEnabled(ctx, tconn, tabletModeEnabled)
		if err := ash.SetTabletModeEnabled(ctx, tconn, false); err != nil {
			s.Fatal("Failed to set tablet mode disabled: ", err)
		}
		// TODO(crbug.com/1002958): Wait for "tablet mode animation is finished" in a reliable way.
		// If an activity is launched while the tablet mode animation is active, the activity
		// will be launched in un undefined state, making the test flaky.
		if err := testing.Sleep(ctx, 5*time.Second); err != nil {
			s.Fatal("Failed to wait until tablet-mode animation finished: ", err)
		}
	}

	s.Log("Pushing trace file")

	out, err := a.Command(ctx, "mktemp", "-d", "-p", "/sdcard").Output()
	if err != nil {
		s.Fatal("Failed to create tmp dir: ", err)
	}
	tmpDir := strings.TrimSpace(string(out))
	defer a.RemoveAll(ctx, tmpDir)

	s.Log("Temp dir: ", tmpDir)

	tracePath := filepath.Join(tmpDir, traceFile)
	resultPath := filepath.Join(tmpDir, traceFile+".result.json")

	if err := a.PushFile(ctx, s.DataPath(traceFile), tracePath); err != nil {
		s.Fatal("Failed to push the trace file: ", err)
	}

	if err := startRetrace(ctx, a, s.DataPath(apkName), tracePath, resultPath); err != nil {
		s.Fatal("Cannot start retrace: ", err)
	}

	if err := waitForResult(ctx, a, resultPath, timeout); err != nil {
		s.Fatal("Timeout: ", err)
	}

	perfValues := perf.NewValues()
	defer func() {
		if err := perfValues.Save(s.OutDir()); err != nil {
			s.Error("Cannot save perf data: ", err)
		}
	}()
	if err := setPerf(ctx, a, perfValues, resultPath); err != nil {
		s.Fatal("Failed to set perf values: ", err)
	}
}

// startRetrace launches retrace of patrace
//
// apkPath: Local path to eglretrace-release.apk of patrace
// tracePath: Local path to the trace file
// resultPath: Path to save the result file to
func startRetrace(ctx context.Context, a *arc.ARC, apkPath string, tracePath string, resultPath string) error {
	if err := a.Install(ctx, apkPath); err != nil {
		return errors.Wrapf(err, "failed installing %s", apkPath)
	}

	cmd := a.Command(ctx, "am", "start", "-n", pkgName+"/"+activityName, "--es", "fileName", tracePath, "--es", "resultFile", resultPath)
	output, err := cmd.Output()
	if err != nil {
		return errors.Wrap(err, "failed to start activity")
	}

	// "adb shell" doesn't distinguish between a failed/successful run. For that we have to parse the output.
	// Looking for:
	//  Starting: Intent { act=android.intent.action.MAIN cat=[android.intent.category.LAUNCHER] cmp=com.example.name/.ActvityName }
	//  Error type 3
	//  Error: Activity class {com.example.name/com.example.name.ActvityName} does not exist.
	re := regexp.MustCompile(`(?m)^Error:\s*(.*)$`)
	groups := re.FindStringSubmatch(string(output))
	if len(groups) == 2 {
		testing.ContextLog(ctx, "Failed to start activity: ", groups[1])
		return errors.New("failed to start activity")
	}
	return nil
}

// waitForResult waits for the result file to appear
func waitForResult(ctx context.Context, a *arc.ARC, resultPath string, timeout time.Duration) error {
	return testing.Poll(ctx, func(ctx context.Context) error {
		_, err := a.ReadFile(ctx, resultPath)
		if err != nil {
			// It is OK if it does not exist yet
			return err
		}
		return nil
	}, &testing.PollOptions{Timeout: timeout, Interval: 5 * time.Second})
}

func setPerf(ctx context.Context, a *arc.ARC, perfValues *perf.Values, resultPath string) error {
	buf, err := a.ReadFile(ctx, resultPath)
	if err != nil {
		return err
	}

	var m struct {
		Results []struct {
			Time float64 `json:"time"`
			FPS  float64 `json:"fps"`
		} `json:"result"`
	}
	if err := json.Unmarshal(buf, &m); err != nil {
		return err
	}

	perfValues.Set(
		perf.Metric{
			Name:      "trace",
			Unit:      "s",
			Direction: perf.SmallerIsBetter,
			Multiple:  false,
		}, m.Results[0].Time)
	perfValues.Set(
		perf.Metric{
			Name:      "trace",
			Unit:      "fps",
			Direction: perf.BiggerIsBetter,
			Multiple:  false,
		}, m.Results[0].FPS)

	return nil
}
