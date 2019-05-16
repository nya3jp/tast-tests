package trace

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"

	"chromiumos/tast/errors"
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
		s.Log("Command `glxinfo` failed: ", err)
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

// RunAPITrace run trace file and return the result.
func RunAPITrace(ctx context.Context, s *testing.State, cont *vm.Container, traceFileName string) (result APITraceResult) {
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
