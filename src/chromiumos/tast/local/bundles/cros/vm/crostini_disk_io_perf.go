// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package vm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/vm"
	"chromiumos/tast/testing"
)

const (
	containerHomeDir = "/home/testuser"
	testDataFileName = "fio_test_data"
)

// Defines whether to report write and/or read bandwidth for each jobfile.
type reportFlags int

const (
	reportRead reportFlags = 1 << iota
	reportWrite
)

var (
	jobFiles = [...]struct {
		name       string
		reportFlag reportFlags
	}{
		{"disk_io_perf_fio_seq_write.ini", reportWrite},
		{"disk_io_perf_fio_seq_read.ini", reportRead},
		{"disk_io_perf_fio_rand_write.ini", reportWrite},
		{"disk_io_perf_fio_rand_read.ini", reportRead},
		{"disk_io_perf_fio_stress_rw.ini", reportWrite | reportRead},
	}
)

func init() {
	var dataFiles []string
	for _, jobFile := range jobFiles {
		dataFiles = append(dataFiles, jobFile.name)
	}

	testing.AddTest(&testing.Test{
		Func:         CrostiniDiskIOPerf,
		Desc:         "Tests Crostini Disk IO Performance",
		Attr:         []string{"informational", "group:crosbolt", "crosbolt_nightly"},
		Data:         dataFiles,
		Timeout:      30 * time.Minute,
		SoftwareDeps: []string{"chrome_login", "vm_host"},
	})
}

// Describes either a container(guest) environment or native(host) environment on which a performance test is executed.
type runEnv interface {
	// Returns the file name the disk I/O test is performed on.
	getTestDataFileName() string
	// Returns the absolute path of the fio jobfile name.
	getJobfilePath(jobFile string) string
	// Returns the full fio command to execute.
	getFioCommand(ctx context.Context, jobFile string, envArgs []string, extraArgs []string) *testexec.Cmd
}

// A guest environment type which implements runEnv.
type guestEnv struct {
	cont *vm.Container
}

func (g guestEnv) getTestDataFileName() string {
	return filepath.Join(containerHomeDir, testDataFileName)
}

func (g guestEnv) getJobfilePath(jobFile string) string {
	return filepath.Join(containerHomeDir, jobFile)
}

func (g guestEnv) getFioCommand(ctx context.Context, jobFile string, envArgs []string, extraArgs []string) *testexec.Cmd {
	return g.cont.Command(ctx, append(envArgs, append([]string{"fio", jobFile}, extraArgs...)...)...)
}

// A host environment type which implements runEnv.
type hostEnv struct {
	dataPathFunc func(string) string
}

func (h hostEnv) getTestDataFileName() string {
	return filepath.Join("/mnt/stateful_partition", testDataFileName)
}

func (h hostEnv) getJobfilePath(jobFile string) string {
	return h.dataPathFunc(jobFile)
}

func (h hostEnv) getFioCommand(ctx context.Context, jobFile string, envArgs []string, extraArgs []string) *testexec.Cmd {
	cmd := testexec.CommandContext(ctx, "fio", append([]string{jobFile}, extraArgs...)...)
	cmd.Env = envArgs
	return cmd
}

// fio parameters.
type fioSetting struct {
	fileSize, blockSize string
	loopNum             int
}

// runFio runs a fio command and returns the average read/write bandwidth in kB per second.
// |jobFile| is the .ini file to be used by the fio command. The job file may contain variables which
// can be substituted by environment variables passed to fio command. For example, if the .ini file has lines like:
//
// [fio_rand_write]
//   filename=${FILENAME}
//   size=${FILESIZE}
//   bs=${BLOCKSIZE}
//   ...
//
// An example fio command could be like
//     FILESIZE=1G FILENAME=fio_test_data BLOCKSIZE=4m fio fio_seq_write --output-format=json
// |setting| are fio parameters to be passed via the environment variables.
func runFio(ctx context.Context, re runEnv, jobFile string, setting fioSetting) (avgRead float64, avgWrite float64, err error) {
	envArgs := []string{
		"FILENAME=" + re.getTestDataFileName(),
		"FILESIZE=" + setting.fileSize,
		"BLOCKSIZE=" + setting.blockSize,
		fmt.Sprintf("LOOPNUM=%v", setting.loopNum),
	}

	extraArgs := []string{"--output-format=json"}

	cmd := re.getFioCommand(ctx, re.getJobfilePath(jobFile), envArgs, extraArgs)
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		var firstLine string
		firstLineBytes, parseErr := bytes.NewBuffer(out).ReadBytes('\n')
		if parseErr != nil {
			firstLine = parseErr.Error()
		} else {
			firstLine = string(firstLineBytes)
		}
		return 0, 0, errors.Wrap(err, "failed to run fio: "+firstLine)
	}

	var result struct {
		Jobs []struct {
			Read, Write struct {
				Bw float64
			}
		}
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return 0, 0, errors.Wrap(err, "failed to parse fio result")
	}

	var totalRead, totalWrite float64
	for _, job := range result.Jobs {
		totalRead += job.Read.Bw
		totalWrite += job.Write.Bw
	}
	numJobs := len(result.Jobs)
	avgRead = totalRead / float64(numJobs)
	avgWrite = totalWrite / float64(numJobs)

	return avgRead, avgWrite, nil
}

// reportMetric given the metric name |metricName| and perf numbers of the same configuration running in the container as |guestValue|
// and on the native host machine as |hostValue|, reports three metrics:
// - guest_|metricName| : The perf value in the container.
// - host_|metricName| : The perf value on the host machine.
// - ratio_|metricName| : The ratio of |guestValue| divided by |hostValue|.
func reportMetric(ctx context.Context, metricName string, guestValue float64, hostValue float64, perfValues perf.Values) {
	ratio := guestValue / hostValue
	testing.ContextLogf(ctx, "Reporting metric %v: %v %v %v", metricName, guestValue, hostValue, ratio)
	perfValues.Append(perf.Metric{
		Name:      "crosini_disk_io",
		Variant:   "guest_" + metricName,
		Unit:      "KB_per_sec",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}, guestValue)

	perfValues.Append(perf.Metric{
		Name:      "crosini_disk_io",
		Variant:   "host_" + metricName,
		Unit:      "KB_per_sec",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}, hostValue)

	perfValues.Append(perf.Metric{
		Name:      "crosini_disk_io",
		Variant:   "ratio_" + metricName,
		Unit:      "percentage",
		Direction: perf.BiggerIsBetter,
		Multiple:  true,
	}, ratio)
}

func runJobFile(ctx context.Context, guestRe runEnv, hostRe runEnv, jobFile string, reportFlag reportFlags, perfValues perf.Values) {
	testing.ContextLog(ctx, "Running jobfile", jobFile)

	// Delete the test data file on host.
	defer func() {
		hostDataFile := hostRe.getTestDataFileName()
		if _, err := os.Stat(hostDataFile); !os.IsNotExist(err) {
			os.Remove(hostDataFile)
		}
	}()

	for _, setting := range []fioSetting{
		{"300m", "4k", 1},
		{"500m", "64k", 1},
		{"500m", "1m", 2},
		{"500m", "16m", 2},
	} {
		baseMetricName := fmt.Sprintf("%v_bs_%v", jobFile, setting.blockSize)

		const numTries = 3

		for i := 1; i <= numTries; i++ {
			testing.ContextLogf(ctx, "Running %v with bs %v in container (%v/%v)", jobFile, setting.blockSize, i, numTries)
			guestRead, guestWrite, err := runFio(ctx, guestRe, jobFile, setting)
			if err != nil {
				testing.ContextLog(ctx, err)
				continue
			}

			testing.ContextLogf(ctx, "Running %v with bs %v on host (%v/%v)", jobFile, setting.blockSize, i, numTries)
			hostRead, hostWrite, err := runFio(ctx, hostRe, jobFile, setting)
			if err != nil {
				testing.ContextLog(ctx, err)
				continue
			}

			if reportFlag&reportWrite != 0 {
				reportMetric(ctx, baseMetricName+"_write", guestWrite, hostWrite, perfValues)
			}
			if reportFlag&reportRead != 0 {
				reportMetric(ctx, baseMetricName+"_read", guestRead, hostRead, perfValues)
			}
		}
	}
}

// DiskIOPerf Run disk IO performance test by running the tool "fio".
func diskIOPerf(ctx context.Context, cont *vm.Container, dataPathFunc func(string) string) (perf.Values, error) {
	testing.ContextLog(ctx, "Installing fio")
	cmd := cont.Command(ctx, "sudo", "apt-get", "-y", "install", "fio")
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrap(err, "failed to install fio")
	}

	testing.ContextLog(ctx, "Copying job files to container")
	for _, jobFile := range jobFiles {
		containerJobFilePath := filepath.Join(containerHomeDir, jobFile.name)
		if err := cont.PushFile(ctx, dataPathFunc(jobFile.name), containerJobFilePath); err != nil {
			return nil, errors.Wrapf(err, "failed to push %v to container", jobFile.name)
		}
	}

	guestRe := guestEnv{cont}
	hostRe := hostEnv{dataPathFunc}
	perfValues := perf.Values{}
	for _, jobFile := range jobFiles {
		runJobFile(ctx, guestRe, hostRe, jobFile.name, jobFile.reportFlag, perfValues)
	}

	return perfValues, nil
}

func CrostiniDiskIOPerf(ctx context.Context, s *testing.State) {
	// TODO(cylee): Consolidate container creation logic in a util function since it appears in multiple files.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Enabling Crostini preference setting")
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	if err = vm.EnableCrostini(ctx, tconn); err != nil {
		s.Fatal("Failed to enable Crostini preference setting: ", err)
	}

	s.Log("Setting up component ", vm.StagingComponent)
	err = vm.SetUpComponent(ctx, vm.StagingComponent)
	if err != nil {
		s.Fatal("Failed to set up component: ", err)
	}

	s.Log("Creating default container")
	cont, err := vm.CreateDefaultContainer(ctx, cr.User(), vm.StagingImageServer)
	if err != nil {
		s.Fatal("Failed to set up default container: ", err)
	}
	defer func() {
		if err := cont.DumpLog(ctx, s.OutDir()); err != nil {
			s.Error("Failure dumping container log: ", err)
		}
	}()

	dataPathFunc := s.DataPath
	perfValues, err := diskIOPerf(ctx, cont, dataPathFunc)
	if err != nil {
		s.Error("diskIOPerf failed: ", err)
	}
	perfValues.Save(s.OutDir())
}
