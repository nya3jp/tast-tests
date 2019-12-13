package patrace

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/testing"
)

// RunTrace replays a PATrace (GLES) (https://github.com/ARM-software/patrace)
// in android. APK and trace data are specified by apkFile and traceFile.
func RunTrace(ctx context.Context, s *testing.State, apkFile, traceFile string) {
	const (
		pkgName      = "com.arm.pa.paretrace"
		activityName = ".Activities.RetraceActivity"
	)

	// Reuse existing ARC and Chrome session.
	a := s.PreValue().(arc.PreData).ARC

	s.Log("Pushing trace file")

	out, err := a.Command(ctx, "mktemp", "-d", "-p", "/sdcard").Output()
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	tmpDir := strings.TrimSpace(string(out))
	defer a.RemoveAll(ctx, tmpDir)

	s.Log("Temp dir: ", tmpDir)

	tracePath := filepath.Join(tmpDir, traceFile)
	resultPath := filepath.Join(tmpDir, traceFile+".result.json")

	if err := a.PushFile(ctx, s.DataPath(traceFile), tracePath); err != nil {
		s.Fatal("Failed to push the trace file: ", err)
	}

	if err := a.Install(ctx, s.DataPath(apkFile)); err != nil {
		s.Fatal("failed installing %s", s.DataPath(apkFile), err)
	}

	act, err := arc.NewActivity(a, pkgName, activityName)
	if err != nil {
		s.Fatal("Failed to create new activity: ", err)
	}
	defer act.Close()

	if err := act.StartWithArgs(ctx, []string{"-W", "-S", "-n"}, []string{"--es", "fileName", tracePath, "--es", "resultFile", resultPath}); err != nil {
		s.Fatal("Cannot start retrace: ", err)
	}

	if err := act.WaitForFinish(ctx, 0); err != nil {
		s.Fatal("waitForFinish failed: ", err)
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

func setPerf(ctx context.Context, a *arc.ARC, perfValues *perf.Values, resultPath string) error {
	buf, err := a.ReadFile(ctx, resultPath)
	if err != nil {
		return errors.Errorf("Failed to read result file: %s. paretrace did not finish successfully", resultPath)
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
