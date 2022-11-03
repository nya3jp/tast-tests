// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"bufio"
	"context"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BasicThermalParticipants,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checking the availability of all basic thermal participants",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Timeout:      5 * time.Minute,
		Fixture:      "chromeLoggedIn",
	})
}

func BasicThermalParticipants(ctx context.Context, s *testing.State) {
	const sysfsThermalPath = "/sys/class/thermal"
	files, err := ioutil.ReadDir(sysfsThermalPath)
	if err != nil {
		s.Fatal("Failed to read sysfs dir: ", err)
	}
	coolingDeviceCount := 0
	var allParticipants []string
	mandatoryParticipants := []string{"x86_pkg_temp", "INT3400 Thermal", "TCPU", "TSR0"}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), "thermal_zone") {
			devPath := path.Join(sysfsThermalPath, file.Name())
			if temp, err := readInt64(path.Join(devPath, "temp")); err != nil || temp == 0 {
				s.Fatal("Failed, temperature of the participants should be displayed and should be non-zero values: ", err)
			}

			participant, err := readFirstLine(path.Join(devPath, "type"))
			if err != nil {
				s.Fatalf("Failed to read path %v: %v", devPath, err)
			}
			allParticipants = append(allParticipants, participant)

		}

		if strings.HasPrefix(file.Name(), "cooling_device") {
			devPath := path.Join(sysfsThermalPath, file.Name())
			participant, err := readFirstLine(path.Join(devPath, "type"))
			if err != nil {
				s.Fatalf("Failed to read path %v: %v", devPath, err)
			}
			if participant == "Processor" {
				coolingDeviceCount++
			}
		}
	}

	cpuCount, err := getNumberOfCPU(ctx)
	if err != nil {
		s.Fatal("Failed to get number of CPU(s): ", err)
	}

	if cpuCount != coolingDeviceCount {
		s.Fatalf("Failed to validate CPU(s) count; got %d, want %d", coolingDeviceCount, cpuCount)
	}
	if !containsAll(allParticipants, mandatoryParticipants) {
		s.Fatal("Failed, thermal_zone don't have all required participants")
	}
}

// getNumberOfCPU returns total online CPU count from lsusb.
func getNumberOfCPU(ctx context.Context) (int, error) {
	lscpu := testexec.CommandContext(ctx, "lscpu")
	out, err := lscpu.Output()
	if err != nil {
		return -1, errors.Wrap(err, "lscpu failed")
	}
	cpuRe := regexp.MustCompile(`^CPU\(s\):\s*(.*)$`)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if !cpuRe.MatchString(line) {
			continue
		}
		cpus := cpuRe.FindStringSubmatch(line)
		ret, err := strconv.Atoi(cpus[1])
		if err != nil {
			return -1, errors.Wrap(err, "failed parsing number of CPUs")
		}
		return ret, nil
	}
	return -1, errors.New("can't find CPU(s) info in lscpu")
}

// containsAll checks that sliceToQuery is a superset of sliceToMatch.
func containsAll(sliceToQuery, sliceToMatch []string) bool {
	for _, item := range sliceToMatch {
		if !contains(sliceToQuery, item) {
			return false
		}
	}
	return true
}

// contains checks that sliceToQuery contains an instance of toFind.
func contains(sliceToQuery []string, toFind string) bool {
	for _, item := range sliceToQuery {
		if item == toFind {
			return true
		}
	}
	return false
}

// readInt64 reads a line from a file and converts it into int64.
func readInt64(filePath string) (int64, error) {
	str, err := readFirstLine(filePath)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read first line")
	}
	return strconv.ParseInt(str, 10, 64)
}

// readFirstLine reads the first line from a file.
// Line feed character will be removed to ease converting the string
// into other types.
func readFirstLine(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to open file")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	if err := scanner.Err(); err != nil {
		return "", errors.Wrap(err, "failed to read file")
	}
	return "", errors.Errorf("found no content in %q", filePath)
}
