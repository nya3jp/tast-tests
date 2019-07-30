// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/perf"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	var dataFiles []string
	dataFiles = append(dataFiles, crostini.ImageArtifact)
	for _, job := range fioJobs {
		dataFiles = append(dataFiles, job.fileName)
	}

	testing.AddTest(&testing.Test{
		Func:     DiskIOPerf,
		Desc:     "Tests Crostini Disk IO Performance",
		Contacts: []string{"cylee@chromium.org", "cros-containers-dev@google.com"},
		// TODO(cylee): A presubmit check enforce "informational". Confirm if we should remove the checking.
		Attr:         []string{"informational", "group:crosbolt", "crosbolt_nightly"},
		Timeout:      30 * time.Minute,
		Data:         dataFiles,
		Pre:          crostini.StartedByArtifact(),
		SoftwareDeps: []string{"chrome", "vm_host"},
	})
}

// Defines whether to report write and/or read bandwidth for each fioJob.
type reportFlags int

const (
	reportRead reportFlags = 1 << iota
	reportWrite
)

type fioJob struct {
	fileName, name string
	reportFlag     reportFlags
}

var (
	fioJobs = []fioJob{
		{"disk_io_perf_fio_seq_write.ini", "seq_write", reportWrite},
		{"disk_io_perf_fio_seq_read.ini", "seq_read", reportRead},
		{"disk_io_perf_fio_rand_write.ini", "rand_write", reportWrite},
		{"disk_io_perf_fio_rand_read.ini", "rand_read", reportRead},
		{"disk_io_perf_fio_stress_rw.ini", "stress_rw", reportWrite | reportRead},
	}
)

// Describes either a container(guest) environment or native(host) environment on which a performance test is executed.
type runEnv struct {
	// File name the disk I/O test is performed on.
	testDataPath string
	// Returns the absolute path of the fio jobfile name.
	jobFilePath func(jobFile string) string
	// Returns the full fio command to execute.
	fioCmd func(ctx context.Context, jobFile string, envArgs []string, args []string) *testexec.Cmd
}

type fioSettings struct {
	fileSize  string // fio size description, e.g., "1G", or "4m".
	blockSize string // Ditto.
	runTime   string // fio time description, e.g., "10s", "1m".
}

type logFunc func(title string, content []byte) error

// runFIO runs a fio command and returns the average read/write bandwidth in kB per second.
// |jobFile| is the .ini file to be used by the fio command. The job file may contain variables which
// can be substituted by environment variables passed to fio command. For example, if the .ini file has lines like:
//
//   [fio_rand_write]
//   filename=${FILENAME}
//   size=${FILESIZE}
//   bs=${BLOCKSIZE}
//   ...
//
// An example fio command could be like
//     FILESIZE=1G FILENAME=fio_test_data BLOCKSIZE=4m fio fio_seq_write --output-format=json
// |settings| are fio parameters to be passed via the environment variables.
func runFIO(ctx context.Context, re runEnv, jobFile string, settings fioSettings, writeError logFunc) (avgRead, avgWrite float64, err error) {
	envArgs := []string{
		"FILENAME=" + re.testDataPath,
		"FILESIZE=" + settings.fileSize,
		"BLOCKSIZE=" + settings.blockSize,
		"RUNTIME=" + settings.runTime,
	}

	extraArgs := []string{"--output-format=json", "--end_fsync=1"}

	cmd := re.fioCmd(ctx, re.jobFilePath(jobFile), envArgs, extraArgs)
	out, err := cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		if err := writeError("Run fio failure", out); err != nil {
			testing.ContextLog(ctx, "Failed to write fio running error to log file: ", err)
		}
		// Only append the first line of the output to the error.
		if idx := bytes.IndexByte(out, '\n'); idx != -1 {
			out = out[:idx]
		}
		return 0, 0, errors.Wrap(err, "failed to run fio: "+string(out))
	}

	var result struct {
		Jobs []struct {
			Read, Write struct {
				Bw float64
			}
		}
	}
	if err := json.Unmarshal(out, &result); err != nil {
		if err := writeError("Parse fio failure", out); err != nil {
			testing.ContextLog(ctx, "Failed to write fio parsing error to log file: ", err)
		}
		return 0, 0, errors.Wrap(err, "failed to parse fio result")
	}

	var totalRead, totalWrite float64
	for _, job := range result.Jobs {
		totalRead += job.Read.Bw
		totalWrite += job.Write.Bw
	}
	if numJobs := len(result.Jobs); numJobs > 0 {
		avgRead = totalRead / float64(numJobs)
		avgWrite = totalWrite / float64(numJobs)
	}

	return avgRead, avgWrite, nil
}

// reportMetric given the metric name |metricName| and perf numbers of the same configuration running in the container as |guestValue|
// and on the native host machine as |hostValue|, reports three metrics:
// - guest_|metricName| : The perf value in the container.
// - host_|metricName| : The perf value on the host machine.
// - ratio_|metricName| : The ratio of |guestValue| divided by |hostValue|.
func reportMetric(ctx context.Context, metricName string, guestValue, hostValue float64, perfValues *perf.Values) {
	ratio := guestValue / hostValue
	testing.ContextLogf(ctx, "Reporting metric %v: %.1f %.1f %.2f", metricName, guestValue, hostValue, ratio)
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

func runFIOJob(ctx context.Context, s *testing.State, guestEnv, hostEnv runEnv, job fioJob, perfValues *perf.Values, writeError logFunc) {
	testing.ContextLog(ctx, "Running job ", job.name)

	// Delete the test data file on host.
	defer os.RemoveAll(hostEnv.testDataPath)

	for _, settings := range []fioSettings{
		{"300m", "4k", "10s"},
		{"400m", "16k", "10s"},
		{"500m", "64k", "10s"},
	} {
		baseMetricName := fmt.Sprintf("%v_bs_%v", job.name, settings.blockSize)

		const numTries = 3

		for i := 1; i <= numTries; i++ {
			testing.ContextLogf(ctx, "Running %v with block size %v in container (%v/%v)", job.name, settings.blockSize, i, numTries)
			guestRead, guestWrite, err := runFIO(ctx, guestEnv, job.fileName, settings, writeError)
			if err != nil {
				s.Errorf("%v with block size %v failed: %v", job.name, settings.blockSize, err)
				continue
			}

			testing.ContextLogf(ctx, "Running %v with bs %v on host (%v/%v)", job.name, settings.blockSize, i, numTries)
			hostRead, hostWrite, err := runFIO(ctx, hostEnv, job.fileName, settings, writeError)
			if err != nil {
				s.Errorf("%v with block size %v failed: %v", job.name, settings.blockSize, err)
				continue
			}

			if job.reportFlag&reportWrite != 0 {
				reportMetric(ctx, baseMetricName+"_write", guestWrite, hostWrite, perfValues)
			}
			if job.reportFlag&reportRead != 0 {
				reportMetric(ctx, baseMetricName+"_read", guestRead, hostRead, perfValues)
			}
		}
	}
}

// DiskIOPerf runs disk IO performance tests by running the tool "fio".
func DiskIOPerf(ctx context.Context, s *testing.State) {
	cont := s.PreValue().(crostini.PreData).Container

	testing.ContextLog(ctx, "Installing fio")
	if err := cont.Command(ctx, "sudo", "apt-get", "-y", "install", "fio").Run(); err != nil {
		s.Fatal("Failed to install fio: ", err)
	}

	const (
		containerHomeDir = "/home/testuser"
		testDataFileName = "fio_test_data"
	)

	testing.ContextLog(ctx, "Copying job files to container")
	for _, job := range fioJobs {
		containerJobFilePath := filepath.Join(containerHomeDir, job.fileName)
		if err := cont.PushFile(ctx, s.DataPath(job.fileName), containerJobFilePath); err != nil {
			s.Fatalf("Failed to push %v to container: %v", job.fileName, err)
		}
	}

	guestEnv := runEnv{
		testDataPath: filepath.Join(containerHomeDir, testDataFileName),
		jobFilePath: func(jobFile string) string {
			return filepath.Join(containerHomeDir, jobFile)
		},
		fioCmd: func(ctx context.Context, jobFile string, envArgs []string, args []string) *testexec.Cmd {
			cmdLine := append(envArgs, "fio", jobFile)
			cmdLine = append(cmdLine, args...)
			return cont.Command(ctx, cmdLine...)
		},
	}

	hostEnv := runEnv{
		testDataPath: filepath.Join("/mnt/stateful_partition", testDataFileName),
		jobFilePath:  s.DataPath,
		fioCmd: func(ctx context.Context, jobFile string, envArgs []string, args []string) *testexec.Cmd {
			cmd := testexec.CommandContext(ctx, "fio", append([]string{jobFile}, args...)...)
			cmd.Env = envArgs
			return cmd
		},
	}

	errFile, err := os.Create(filepath.Join(s.OutDir(), "error_log.txt"))
	if err != nil {
		s.Fatal("Failed to create error log: ", err)
	}
	defer errFile.Close()
	writeError := func(title string, content []byte) error {
		const logTemplate = "========== START %s ==========\n%s\n========== END ==========\n"
		if _, err := fmt.Fprintf(errFile, logTemplate, title, content); err != nil {
			return err
		}
		return nil
	}

	perfValues := perf.NewValues()
	for _, job := range fioJobs {
		runFIOJob(ctx, s, guestEnv, hostEnv, job, perfValues, writeError)
	}
	perfValues.Save(s.OutDir())
}
