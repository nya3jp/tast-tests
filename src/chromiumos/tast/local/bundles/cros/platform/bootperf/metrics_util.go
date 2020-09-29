// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package bootperf provides constants and common utilities for test platform.BootPerf.
package bootperf

import (
	"bytes"
	"context"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

const (
	uptimePrefix = "uptime-"
	diskPrefix   = "disk-"

	firmwareTimeFile = "/tmp/firmware-boot-time"

	// The chromeos_shutdown script archives bootstat files under shutdown.TIMESTAMP directory. The timestamp is generated using `date '+%Y%m%d%H%M%S'`.
	bootstatArchiveGlob = "/var/log/metrics/shutdown.[0-9]*"

	// disk usage bootstat numbers are sectors. Convert to bytes by multiplying |sectorSize|.
	sectorSize = 512
)

var (
	// Names of metrics, their associated bootstat events, and 'Required' flag.
	// Test fails if a required event is not found. Each event samples statistics measured since kernel startup at a specific moment on the boot critical path:
	//   pre-startup - The start of the `chromeos_startup` script;
	//     roughly, the time when /sbin/init emits the `startup`
	//     Upstart event.
	//   post-startup - Completion of the `chromeos_startup` script.
	//   chrome-exec - The moment when session_manager exec's the
	//     first Chrome process.
	//   chrome-main - The moment when the first Chrome process
	//     begins executing in main().
	//   kernel_to_signin_start - The moment when LoadPage(loginScreenURL)
	//     is called, i.e. initialization starts.
	//   kernel_to_signin_wait - The moment when UI thread has finished signin
	//     screen initialization and now waits until JS sends "ready" event.
	//   kernel_to_signin_users - The moment when UI thread receives "ready" from
	//     JS code. So V8 is initialized and running, etc...
	//   kernel_to_login - The moment when user can actually see signin UI.
	//   kernel_to_android_start - The moment when Android is started.
	//   kernel_to_cellular_registered - The moment when Shill detects a
	//     cellular device.
	//   kernel_to_wifi_registered - The moment when Shill detects a WiFi device.
	eventMetrics = []struct {
		MetricName string
		EventName  string
		Required   bool
	}{
		{"kernel_to_startup", "pre-startup", true},
		{"kernel_to_startup_done", "post-startup", true},
		{"kernel_to_chrome_exec", "chrome-exec", true},
		{"kernel_to_chrome_main", "chrome-main", true},
		// These two events do not happen if device is in OOBE.
		{"kernel_to_signin_start", "login-start-signin-screen", false},
		{"kernel_to_signin_wait",
			"login-wait-for-signin-state-initialize", false},
		// This event doesn't happen if device has no users.
		{"kernel_to_signin_users", "login-send-user-list", false},
		{"kernel_to_login", "login-prompt-visible", true},
		// Not all boards support ARC.
		{"kernel_to_android_start", "android-start", false},
		// Not all devices have cellular. All should have WiFi, but we
		// still don't want to fail (e.g., if there are hardware
		// issues).
		{"kernel_to_cellular_registered", "network-cellular-registered", false},
		{"kernel_to_wifi_registered", "network-wifi-registered", false},
	}

	uptimeFileGlob = filepath.Join("/tmp", uptimePrefix+"*")
	diskFileGlob   = filepath.Join("/tmp", diskPrefix+"*")

	// The name of this file has changed starting with linux-3.19.
	// Use a glob to snarf up all existing records.
	ramOopsFileGlob = "/sys/fs/pstore/console-ramoops*"
)

// WaitUntilBootComplete is a helper function to wait until boot complete and
// we are ready to collect boot metrics.
func WaitUntilBootComplete(ctx context.Context) error {
	return testing.Poll(ctx, func(context.Context) error {
		// Check that bootstat files are available.
		for _, k := range eventMetrics {
			if !k.Required {
				continue
			}

			for _, prefix := range []string{uptimePrefix, diskPrefix} {
				key := filepath.Join("/tmp", prefix+k.EventName)
				if _, err := os.Stat(key); err != nil {
					return errors.Wrapf(err, "error in waiting for bootstat file %s", key)
				}
			}
		}

		// Check that firmware boot time files is available.
		if _, err := os.Stat(firmwareTimeFile); err != nil {
			return errors.New("waiting for firmware boot time file")
		}

		// Wait until bootchart is stopped.
		job := "bootchart"
		_, state, _, err := upstart.JobStatus(ctx, job)
		if err != nil {
			return errors.Wrapf(err, "failed to get status of job %q", job)
		}
		if state != upstart.WaitingState {
			return errors.Errorf("waiting for %q to stop (current state: %q)", job, state)
		}

		// Wait until system-services is started.
		job = "system-services"
		_, state, _, err = upstart.JobStatus(ctx, job)
		if err != nil {
			return errors.Wrapf(err, "failed to get status of job %q", job)
		}
		if state != upstart.RunningState {
			return errors.Errorf("waiting for %q to start (current state: %q)", job, state)
		}

		return nil
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: time.Second,
	})
}

// parseBootstat reads values from a bootstat event file. Each line of a
// bootstat event file represents one occurrence of the event. Each line is a
// copy of the content of /proc/uptime ("uptime-" files) or
// /sys/block/<dev>/stat ("disk-" files), captured at the time of the
// occurrence. For either kind of file, each line is a blank separated list of
// fields. The given event file can contain either uptime or disk data. This
// function reads all lines (occurrences) in the event file, and returns the
// value of the given field.
func parseBootstat(fileName string, fieldNum int) ([]float64, error) {
	var result []float64
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return nil, errors.Errorf("bootstat file %s is empty", fileName)
	}

	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		f := strings.Fields(line)
		if fieldNum >= len(f) {
			continue
		}
		s, err := strconv.ParseFloat(f[fieldNum], 64)
		if err != nil {
			return nil, errors.Wrapf(err, "malformed bootstat content: %s", line)
		}
		result = append(result, s)
	}

	return result, nil
}

// parseUptime returns time since boot for a bootstat event.
func parseUptime(eventName, bootstatDir string, index int) (float64, error) {
	eventFile := filepath.Join(bootstatDir, uptimePrefix+eventName)
	val, err := parseBootstat(eventFile, 0)
	if err != nil {
		return 0.0, err
	}

	n := len(val)
	// Check for OOB access.
	if index < -n || index > n-1 {
		return 0.0, errors.Errorf("bootstat index out of bound. len=%d, index=%d", n, index)
	}

	if index >= 0 {
		return val[index], nil
	}
	// Like negative index in python.
	return val[n+index], nil
}

// GatherTimeMetrics reads and reports boot time metrics. It reads
// "seconds since kernel startup" from the bootstat files for the events named
// in |eventMetrics|, and stores the values as perf metrics.  The following
// metrics may be recorded:
//   * seconds_kernel_to_startup
//   * seconds_kernel_to_startup_done
//   * seconds_kernel_to_chrome_exec
//   * seconds_kernel_to_chrome_main
//   * seconds_kernel_to_signin_start
//   * seconds_kernel_to_signin_wait
//   * seconds_kernel_to_signin_users
//   * seconds_kernel_to_login
//   * seconds_kernel_to_android_start
//   * seconds_kernel_to_cellular_registered
//   * seconds_kernel_to_wifi_registered
//   * seconds_kernel_to_network
func GatherTimeMetrics(ctx context.Context, results *platform.GetBootPerfMetricsResponse) error {
	for _, k := range eventMetrics {
		key := "seconds_" + k.MetricName
		val, err := parseUptime(k.EventName, "/tmp", 0)
		if err != nil {
			if k.Required {
				return errors.Wrapf(err, "failed in gather time for %s", k.EventName)
			}
			// Failed in getting a non-required metric. Log and skip.
			testing.ContextLog(ctx, "Warning: failed to gather time for non-required event: ", err)
		} else {
			results.Metrics[key] = val
		}
	}

	// Not all 'uptime-network-*-ready' files necessarily exist; probably there's only one.
	// We go through a list of possibilities and pick the earliest one we find.
	// We're not looking for 3G here, so we're not guaranteed to find any file.
	networkReadyEvents := []string{"network-wifi-ready", "network-ethernet-ready"}
	firstNetworkReadyTime := math.MaxFloat64
	for _, e := range networkReadyEvents {
		metricName := "seconds_kernel_to_" + strings.ReplaceAll(e, "-", "_")
		t, err := parseUptime(e, "/tmp", 0)
		if err != nil {
			continue
		}

		results.Metrics[metricName] = t
		if t < firstNetworkReadyTime {
			firstNetworkReadyTime = t
		}
	}

	if firstNetworkReadyTime != math.MaxFloat64 {
		results.Metrics["seconds_kernel_to_network"] = firstNetworkReadyTime
	}

	return nil
}

// parseDiskstat returns sectors read since boot for a bootstat event.
func parseDiskstat(eventName, bootstatDir string, index int) (float64, error) {
	eventFile := filepath.Join(bootstatDir, diskPrefix+eventName)
	val, err := parseBootstat(eventFile, 2)
	if err != nil {
		return 0.0, err
	}
	return val[index], nil
}

// GatherDiskMetrics reads and reports disk read metrics.
// It reads "sectors read since kernel startup" from the bootstat files for the
// events named in |eventMetrics|, converts the values to "bytes read since
// boot", and stores the values as perf metrics. The following metrics are
// recorded:
//   * rdbytes_kernel_to_startup
//   * rdbytes_kernel_to_startup_done
//   * rdbytes_kernel_to_chrome_exec
//   * rdbytes_kernel_to_chrome_main
//   * rdbytes_kernel_to_login
// Disk statistics are reported in units of 512 byte sectors; we convert the
// metrics to bytes so that downstream consumers don't have to ask "How big is
// a sector?".
func GatherDiskMetrics(results *platform.GetBootPerfMetricsResponse) {
	// We expect an error when reading disk statistics for the "chrome-main" event because Chrome (not bootstat) generates that event, and it doesn't include the disk statistics.
	// We get around that by ignoring all errors.
	for _, k := range eventMetrics {
		key := "rdbytes_" + k.MetricName
		val, err := parseDiskstat(k.EventName, "/tmp", 0)
		if err == nil {
			results.Metrics[key] = val * sectorSize
		} // else skip the error and continue.
	}
}

// round is an utility function for rounding |x| to 2 decimal places.
func round(x float64) float64 {
	return math.Round(x*100) / 100
}

// GatherFirmwareBootTime reads and reports firmware startup time. The boot
// process writes the firmware startup time to the file named in
// |firmwareTimeFile|. Read the time from that file, and record it as the metric
// seconds_power_on_to_kernel.
func GatherFirmwareBootTime(results *platform.GetBootPerfMetricsResponse) error {
	b, err := ioutil.ReadFile(firmwareTimeFile)
	for err != nil {
		return errors.Wrapf(err, "failed to open file %s", firmwareTimeFile)
	}

	l := strings.Split(string(b), "\n")[0]

	fw, err := strconv.ParseFloat(l, 64)
	if err != nil {
		return errors.Wrapf(err, "failed to parse firmware time %s", l)
	}

	bootTime := results.Metrics["seconds_kernel_to_login"]
	results.Metrics["seconds_power_on_to_kernel"] = round(fw)
	results.Metrics["seconds_power_on_to_login"] = round(fw + bootTime)
	return nil
}

// calculateTimeval estimates the absolute time of a time since boot. Input
// values |event| and |tUptime| are times measured as seconds since boot (for
// the same boot event, as from /proc/uptime). The input values |t0| and |t1|
// are two values measured as seconds since the epoch. The three "t" values
// were sampled in the order |t0|, |tUptime|, |t1|.
// This function estimates the time of |event| measured as seconds since the
// epoch and also estimates the worst-case error based on the time elapsed
// between `t0` and `t1`.
// All values are floats.  The precision of |event| and `tUptime` is expected to
// be kernel jiffies (i.e. one centisecond). The output result is rounded to the
// nearest jiffy.
func calculateTimeval(event, t0, t1, tUptime float64) (float64, float64) {
	bootTimeval := round((t0+t1)/2 - tUptime)
	// |error| should be close to 0 so is not rounded.
	error := (t1 - t0) / 2
	return bootTimeval + event, error
}

// GatherRebootMetrics reads and reports shutdown and reboot times. The shutdown
// process saves all bootstat files in /var/log, plus it saves a timestamp file
// that can be used to convert "time since boot" into times in UTC.  Read the
// saved files from the most recent shutdown, and use them to calculate the time
// spent from the start of that shutdown until the completion of the most recent
// boot.
// This function records these metrics:
//   * seconds_shutdown_time
//   * seconds_reboot_time
//   * seconds_reboot_error
func GatherRebootMetrics(results *platform.GetBootPerfMetricsResponse) error {
	bootstatArchives, _ := filepath.Glob(bootstatArchiveGlob) // filepath.Glob() only returns error on malformed glob patterns.
	if bootstatArchives == nil {
		return errors.New("failed to list bootstat archive directories")
	}

	// max() for string comparison.
	max := func(in []string) string {
		m := ""
		for _, s := range in {
			if s > m {
				m = s
			}
		}
		return m
	}
	// It's safe using string > operator in max() to find the entry with the largest timestamp value because the timestamp is generated using the command `date '+%Y%m%d%H%M%S'`.
	bootstatDir := max(bootstatArchives)

	bootID, err := ioutil.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return errors.Wrap(err, "failed to read boot_id")
	}

	didrunPath := filepath.Join(bootstatDir, "bootperf_ran")
	_, err = os.Stat(didrunPath)
	if err == nil {
		// File exists. Compare with the current boot ID. Proceed only if the boot ID matches.
		b, err := ioutil.ReadFile(didrunPath)
		if err != nil {
			return errors.Wrap(err, "failed to read from bootperf_ran")
		}
		if !bytes.Equal(b, bootID) {
			// Returns an error on boot id mismatch
			return errors.Errorf("boot id mismatch: %s != %s", string(b), string(bootID))
		}
	} else if os.IsNotExist(err) {
		if err := ioutil.WriteFile(didrunPath, bootID, 0644); err != nil {
			return errors.Wrapf(err, "failed to write boot ID to %s", didrunPath)
		}
	} else {
		return errors.Wrap(err, "failed in getting the information of bootperf_ran")
	}

	timestampPath := filepath.Join(bootstatDir, "timestamp")

	b, err := ioutil.ReadFile(timestampPath)
	if err != nil {
		return errors.Wrap(err, "failed to read timestamp")
	}
	archiveTs := strings.Split(string(b), "\n")
	archiveUptime, err := parseUptime("archive", bootstatDir, 0)
	if err != nil {
		return errors.Wrap(err, "failed in parsing uptime of event archive")
	}
	shutdownUptime, err := parseUptime("ui-post-stop", bootstatDir, -1)
	if err != nil {
		return errors.Wrap(err, "failed in parsing uptime of event archive")
	}
	archiveT0, err := strconv.ParseFloat(archiveTs[0], 64)
	if err != nil {
		return errors.Wrap(err, "error in parsing timestamp value")
	}
	archiveT1, err := strconv.ParseFloat(archiveTs[1], 64)
	if err != nil {
		return errors.Wrapf(err, "error in parsing timestamp value %s", archiveTs[1])
	}

	shutdownTimeval, shutdownError := calculateTimeval(shutdownUptime, archiveT0, archiveT1, archiveUptime)

	bootT0 := time.Now().Unix()
	uptime, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		return errors.Wrap(err, "failed to read system uptime")
	}
	bootT1 := time.Now().Unix()
	uptimeF, err := strconv.ParseFloat(strings.Fields(string(uptime))[0], 64)
	if err != nil {
		return errors.Wrapf(err, "failed to parse system uptime %s", string(uptime))
	}
	bootTimeval, bootError := calculateTimeval(results.Metrics["seconds_kernel_to_login"], float64(bootT0), float64(bootT1), uptimeF)

	rebootTime := round(bootTimeval - shutdownTimeval)
	poweronTime := results.Metrics["seconds_power_on_to_login"]
	shutdownTime := round(rebootTime - poweronTime)

	results.Metrics["seconds_reboot_time"] = rebootTime
	results.Metrics["seconds_reboot_error"] = shutdownError + bootError
	results.Metrics["seconds_shutdown_time"] = shutdownTime

	return nil
}

// CalculateDiff generates metrics from existing ones. Metrics of time and
// rdbytes are calculated from kernel startup. For example, metric
// "seconds_startup_to_chrome_exec" that represents time from the "startup"
// stage to Chrome execution begins, is calculated from subtracting
// "seconds_kernel_to_chrome_exec" to "seconds_kernel_to_startup".
func CalculateDiff(results *platform.GetBootPerfMetricsResponse) {
	barriers := []string{"startup", "chrome_exec", "login"}
	types := []string{"seconds", "rdbytes"}
	for i, b := range barriers[:len(barriers)-1] {
		for _, t := range types {
			begin := t + "_kernel_to_" + b
			end := t + "_kernel_to_" + barriers[i+1]

			rb, ok1 := results.Metrics[begin]
			if re, ok2 := results.Metrics[end]; ok1 && ok2 {
				diffName := t + "_" + b + "_to_" + barriers[i+1]
				results.Metrics[diffName] = round(re - rb)
			}
		}
	}
}

// GatherMetricRawDataFiles gathers content of raw data files to be returned to
// the client.
func GatherMetricRawDataFiles(raw map[string][]byte) error {
	files := []string{firmwareTimeFile}
	for _, glob := range []string{uptimeFileGlob, diskFileGlob} {
		list, _ := filepath.Glob(glob) // filepath.Glob() only returns error on malformed glob patterns.
		files = append(files, list...)
	}

	for _, f := range files {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return errors.Wrapf(err, "failed to read from %s", f)
		}
		raw[filepath.Base(f)] = b
	}

	return nil
}

// GatherConsoleRamoops gathers console_ramoops from previous reboot.
func GatherConsoleRamoops(raw map[string][]byte) error {
	list, _ := filepath.Glob(ramOopsFileGlob) // filepath.Glob() only returns error on malformed glob patterns.
	for _, f := range list {
		b, err := ioutil.ReadFile(f)
		if err != nil {
			return errors.Wrapf(err, "failed to read from %s", f)
		}
		raw[filepath.Base(f)] = b
	}

	return nil
}
