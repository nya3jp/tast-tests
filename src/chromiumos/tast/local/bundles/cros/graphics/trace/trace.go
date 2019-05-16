package trace

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

// APITraceResult holds the performance metrics reported by apitrace.
type APITraceResult struct {
	Fps      float64
	Duration float64
	Frames   uint64
}

// InstallApitrace will install/check apitrace in the given vm container.
func InstallApitrace(ctx context.Context, s *testing.State, cont *vm.Container) (err error) {
	// Log glxinfo so that we can give some insight on how the crostini is set up.
	cmd := cont.Command(ctx, "glxinfo")
	if err := cmd.Run(); err != nil {
		s.Fatal("Command `glxinfo` failed: ", err)
	} else {
		cmd.DumpLog(ctx)
	}

	// Stop the apt-daily systemd timers since they may end up running while we
	// are executing the tests and cause failures due to resource contention.
	for _, t := range []string{"apt-daily", "apt-daily-upgrade"} {
		cmd := cont.Command(ctx, "sudo", "systemctl", "stop", t+".timer")
		if err := cmd.Run(); err != nil {
			cmd.DumpLog(ctx)
			s.Logf("Failed to stop %s timer: %v", t, err)
		}
	}

	// TODO(pwang): Install it in container image.
	s.Log("Installing apitrace")
	cmd = cont.Command(ctx, "sudo", "apt", "-y", "install", "apitrace")
	if tmpErr := cmd.Run(); tmpErr != nil {
		cmd.DumpLog(ctx)
		err = tmpErr
	}
	return err
}

// RunAPITraceTest start an VM and runs the trace from the traceNameMap, which traceNameMap mapping from local file name to trace name.
func RunAPITraceTest(ctx context.Context, s *testing.State, traceNameMap map[string]string) {
	cr := s.PreValue().(*chrome.Chrome)

	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}
	defer vm.UnmountComponent(ctx)

	s.Log("Creating default container")
	cont, err := vm.CreateDefaultContainer(ctx, s.OutDir(), cr.User(), vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer func() {
		if err := cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failed to dump container log: ", err)
		}
		vm.StopConcierge(ctx)
	}()

	s.Log("Verifying pwd command works")
	cmd := cont.Command(ctx, "pwd")
	if err = cmd.Run(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to run pwd: ", err)
	}

	shortCtx, shortCancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer shortCancel()
	if err := InstallApitrace(shortCtx, s, cont); err != nil {
		s.Fatal("Failed to get Apitrace: ", err)
	}
	for traceFileName, traceName := range traceNameMap {
		result := runAPITrace(shortCtx, s, cont, traceFileName)
		perfValues := result.GeneratePerfs(traceName)
		for _, perf := range perfValues {
			if err := perf.Save(s.OutDir()); err != nil {
				s.Fatal("Failed saving perf data: ", err)
			}
		}
	}
}

// GeneratePerfs generate the perf metrics for chromeperf upload.
func (result *APITraceResult) GeneratePerfs(testName string) (values []*perf.Values) {
	time := perf.NewValues()
	time.Set(perf.Metric{
		Name:      testName + "/time",
		Unit:      "sec",
		Direction: perf.SmallerIsBetter,
	}, result.Duration)
	frame := perf.NewValues()
	frame.Set(perf.Metric{
		Name:      testName + "/frames",
		Unit:      "frame",
		Direction: perf.BiggerIsBetter,
	}, float64(result.Frames))
	fps := perf.NewValues()
	fps.Set(perf.Metric{
		Name:      testName + "/fps",
		Unit:      "fps",
		Direction: perf.BiggerIsBetter,
	}, result.Fps)

	values = []*perf.Values{time, frame, fps}
	return values
}

// runAPITrace run trace file and return the result.
func runAPITrace(ctx context.Context, s *testing.State, cont *vm.Container, traceFileName string) (result APITraceResult) {
	s.Log("Copy trace file to container")
	containerPath := filepath.Join("/home/testuser", traceFileName)
	if err := cont.PushFile(ctx, s.DataPath(traceFileName), containerPath); err != nil {
		s.Fatal("Failed copying trace file to container: ", err)
	}

	s.Log("Replay trace file")
	cmd := cont.Command(ctx, "apitrace", "replay", containerPath)
	traceOut, err := cmd.CombinedOutput()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to replay apitrace: ", err)
	}

	//Write trace ouptut
	outputFile := filepath.Join(s.OutDir(), traceFileName)
	s.Logf("Dump trace output to file: %s", outputFile)
	if err := ioutil.WriteFile(outputFile, traceOut, 0644); err != nil {
		s.Fatal("Error writing tracing output: ", err)
	}
	result, err = parseAPIAPITraceResult(traceOut)
	if err != nil {
		s.Fatal("Failed to parse the result: ", err)
	}
	return result
}

func parseAPIAPITraceResult(output []byte) (result APITraceResult, err error) {
	re := regexp.MustCompile(`Rendered (\d+) frames in (\d*\.?\d*) secs, average of (\d*\.?\d*) fps`)
	match := re.FindSubmatch(output)
	if match == nil {
		err = errors.New("result line can't be located")
		return result, err
	}
	var tmp = string(bytes.Join(match, []byte{' '}))
	_, err = fmt.Sscanf(tmp, "%d %f %f", &result.Frames, &result.Duration, &result.Fps)
	if err != nil {
		err = errors.New("result line can't be located")
		return result, err
	}
	return result, err
}
