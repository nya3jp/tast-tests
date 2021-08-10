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
	"sort"
	"strconv"
	"strings"
	"time"

	upstartcommon "chromiumos/tast/common/upstart"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

const (
	uptimePrefix = "uptime-"
	diskPrefix   = "disk-"

	firmwareTimeFile = "/tmp/firmware-boot-time"

	// Directory where the current statistics are stored
	// TODO(b:182094511): Move the statistics to a subdirectory of /tmp
	bootstatCurrentDir = "/tmp"

	// The chromeos_shutdown script archives bootstat files under shutdown.TIMESTAMP directory. The timestamp is generated using `date '+%Y%m%d%H%M%S'`.
	bootstatArchiveGlob = "/var/log/metrics/shutdown.[0-9]*"

	// disk usage bootstat numbers are sectors. Convert to bytes by multiplying |sectorSize|.
	sectorSize = 512
)

type metricRequirement int

const (
	// An Optional metric may be collected, but we will not wait for it.
	metricOptional metricRequirement = iota
	// We will wait a reasonable amount of time for a Recommended metric, but won't abort if it's not found.
	metricRecommended
	// We will wait for a Required metric, and abort if it's not found.
	metricRequired
)

var (
	// Names of metrics, their associated bootstat events, and their recommendation status.
	// The test fails if a Required event is not found.
	// A Recommended event will cause the test to wait for some reasonable time to allow the event to be recorded,
	// but not fail the test if it doesn't show up. This can allow for some level of flake in an underlying event
	// (e.g., WiFi hardware failure) without making it completely optional (which would otherwise fail to report if
	// the event is slower than the slowest Required event).
	// Each event samples statistics measured since kernel startup at a specific moment on the boot critical path:
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
		MetricName  string
		EventName   string
		Requirement metricRequirement
	}{
		{"kernel_to_startup", "pre-startup", metricRequired},
		{"kernel_to_startup_done", "post-startup", metricRequired},
		{"kernel_to_chrome_exec", "chrome-exec", metricRequired},
		{"kernel_to_chrome_main", "chrome-main", metricRequired},
		// These two events do not happen if device is in OOBE.
		{"kernel_to_signin_start", "login-start-signin-screen", metricOptional},
		{"kernel_to_signin_wait", "login-wait-for-signin-state-initialize", metricOptional},
		// This event doesn't happen if device has no users.
		{"kernel_to_signin_users", "login-send-user-list", metricOptional},
		{"kernel_to_login", "login-prompt-visible", metricRequired},
		// Not all boards support ARC.
		{"kernel_to_android_start", "android-start", metricOptional},
		// Not all devices have cellular.
		{"kernel_to_cellular_registered", "network-cellular-registered", metricOptional},
		// All should have WiFi, but we still don't want to fail (e.g., if there are hardware issues).
		{"kernel_to_wifi_registered", "network-wifi-registered", metricRecommended},
	}

	uptimeFileGlob = filepath.Join(bootstatCurrentDir, uptimePrefix+"*")
	diskFileGlob   = filepath.Join(bootstatCurrentDir, diskPrefix+"*")

	// The name of this file has changed starting with linux-3.19.
	// Use a glob to snarf up all existing records.
	ramOopsFileGlob = "/sys/fs/pstore/console-ramoops*"
)

// WaitUntilBootComplete is a helper function to wait until boot complete and
// we are ready to collect boot metrics.
func WaitUntilBootComplete(ctx context.Context) error {
	pollOnce := func(ctx context.Context, recommendation metricRequirement) error {
		// Check that bootstat files are available.
		for _, k := range eventMetrics {
			if k.Requirement < recommendation {
				continue
			}

			for _, prefix := range []string{uptimePrefix, diskPrefix} {
				key := filepath.Join(bootstatCurrentDir, prefix+k.EventName)
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
		if state != upstartcommon.WaitingState {
			return errors.Errorf("waiting for %q to stop (current state: %q)", job, state)
		}

		// Wait until system-services is started.
		job = "system-services"
		_, state, _, err = upstart.JobStatus(ctx, job)
		if err != nil {
			return errors.Wrapf(err, "failed to get status of job %q", job)
		}
		if state != upstartcommon.RunningState {
			return errors.Errorf("waiting for %q to start (current state: %q)", job, state)
		}

		return nil
	}

	// Wait for Recommended and Required events for the first 60 seconds.
	if err := testing.Poll(ctx, func(context.Context) error {
		return pollOnce(ctx, metricRecommended)
	}, &testing.PollOptions{
		Timeout:  60 * time.Second,
		Interval: time.Second,
	}); err == nil {
		return nil
	}

	// Try one last time with only Required metrics, in case a Recommended metric wasn't found.
	return pollOnce(ctx, metricRequired)
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
	var missingNonRequiredEvennts []string
	for _, k := range eventMetrics {
		key := "seconds_" + k.MetricName
		val, err := parseUptime(k.EventName, bootstatCurrentDir, 0)
		if err != nil {
			if k.Requirement == metricRequired {
				return errors.Wrapf(err, "failed in gather time for %s", k.EventName)
			}
			// Failed in getting a non-required metric. Log and skip.
			missingNonRequiredEvennts = append(missingNonRequiredEvennts, k.EventName)
		} else {
			results.Metrics[key] = val
		}
	}
	if len(missingNonRequiredEvennts) != 0 {
		testing.ContextLogf(ctx, "Skip gathering time metrics for non-required event: %s", strings.Join(missingNonRequiredEvennts, ", "))
	}

	// Not all 'uptime-network-*-ready' files necessarily exist; probably there's only one.
	// We go through a list of possibilities and pick the earliest one we find.
	// We're not looking for 3G here, so we're not guaranteed to find any file.
	networkReadyEvents := []string{"network-wifi-ready", "network-ethernet-ready"}
	firstNetworkReadyTime := math.MaxFloat64
	for _, e := range networkReadyEvents {
		metricName := "seconds_kernel_to_" + strings.ReplaceAll(e, "-", "_")
		t, err := parseUptime(e, bootstatCurrentDir, 0)
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
		val, err := parseDiskstat(k.EventName, bootstatCurrentDir, 0)
		if err == nil {
			results.Metrics[key] = val * sectorSize
		} // else skip the error and continue.
	}
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
	results.Metrics["seconds_power_on_to_kernel"] = fw
	results.Metrics["seconds_power_on_to_login"] = fw + bootTime
	return nil
}

// calculateTimeOffset calculates the time offset between 2 different clock
// sources (e.g. RTC and uptime), assumed to have no drift (just a fixed
// offset).
//
// The input values |t0| and |t1| are two values read from clock source A,
// |tx| is read from clock source B.
// The three "t" values were sampled in the order |t0|, |tx|, |t1|.
//
// The first return value (`offset`) is the offset between clock A and B, adding
// this offset to a measurement in clock source B will convert it to an
// estimated measurement in clock source A (i.e. `tx + offset = (t0 + t1) / 2`).
// Conversely, subtracting the offset from a measurement in clock source A will
// convert it to an estimated measurement in clock source B.
// The second return value estimates the worst-case error based on the time
// elapsed between `t0` and `t1`.
// All values are floats.
func calculateTimeOffset(t0, t1, tx float64) (float64, float64) {
	offset := (t0+t1)/2 - tx
	error := (t1 - t0) / 2
	return offset, error
}

// parseSyncRtc parses a sync-rtc-* file, which has this format:
// `uptime0 uptime1 RTCDate RTCTime`, where uptimeT* are uptime measurements
// before and after the RTC measurement.
// For example: `7.558581153 7.559699999 2021-02-09 11:42:10`
// Returns `uptime0`, `uptime1` as floats, RTC time as a Unix timestamp.
func parseSyncRtc(rtcPath string) (float64, float64, int64, error) {
	c, err := ioutil.ReadFile(rtcPath)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "failed to read timestamp")
	}
	// In rare cases the sync-rtc-* files contain multiple entries (likely because tlsdated was restarted), parse the most recent entry (the last line) of the file.
	lines := strings.Split(strings.TrimSpace(string(c)), "\n")
	lastLine := strings.TrimSpace(lines[len(lines)-1])
	timesStr := strings.Split(lastLine, " ")
	uptime0, err := strconv.ParseFloat(timesStr[0], 64)
	if err != nil {
		return 0, 0, 0, errors.Wrapf(err, "error in parsing timestamp value %s", timesStr[0])
	}
	uptime1, err := strconv.ParseFloat(timesStr[1], 64)
	if err != nil {
		return 0, 0, 0, errors.Wrapf(err, "error in parsing timestamp value %s", timesStr[1])
	}
	const rtcTimeFormat = "2006-01-02 15:04:05"
	rtcTimeStr := timesStr[2] + " " + timesStr[3]
	rtcTime, err := time.Parse(rtcTimeFormat, rtcTimeStr)
	if err != nil {
		return 0, 0, 0, errors.Wrapf(err, "failed in parsing RTC time %s", rtcTimeStr)
	}
	return uptime0, uptime1, rtcTime.Unix(), nil
}

// findMostRecentBootstatArchivePath returns the path of the bootstat archive
// generated from the most recent successful shutdown.
func findMostRecentBootstatArchivePath() (string, error) {
	bootstatArchives, _ := filepath.Glob(bootstatArchiveGlob) // filepath.Glob() only returns error on malformed glob patterns.
	if len(bootstatArchives) == 0 {
		return "", errors.New("failed to list bootstat archive directories")
	}

	// Sort |bootstatArchives| using string comparison. This works in finding the entry with the largest timestamp value because the timestamp is generated using the command `date '+%Y%m%d%H%M%S'` during shutdown.
	sort.Strings(bootstatArchives)
	for i := len(bootstatArchives) - 1; i >= 0; i-- {
		bootstatDir := bootstatArchives[i]
		// Check that this is a valid archive: in a successful shutdown, the timestamp file should contain 2 entries written by bootstat_archive.
		timestampPath := filepath.Join(bootstatDir, "timestamp")
		b, err := ioutil.ReadFile(timestampPath)
		if err != nil && !os.IsNotExist(err) {
			// Shouldn't have any error other than timestamp not existent.
			return "", errors.Wrapf(err, "unexpected error in checking bootstat archive file: %s", timestampPath)
		}
		if err == nil && len(strings.Split(string(b), "\n")) > 1 {
			return bootstatDir, nil
		}
	}
	return "", errors.New("failed to find the bootstat archive for the latest shutdown")
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
	bootstatDir, err := findMostRecentBootstatArchivePath()
	if err != nil {
		return err
	}

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

	// Time values can come from 3 different sources. To reduce confusion, we suffix the variables as follows:
	//  - *Uptime: uptime in seconds (a.k.a. clock_gettime with CLOCK_BOOTTIME)
	//  - *SystemTime: seconds since epoch, system/wall clock time (a.k.a. clock_gettime with CLOCK_REALTIME, time.Now().Unix())
	//  - *RtcTime: seconds since epoch, but obtained from an RTC source

	shutdownUptime, err := parseUptime("ui-post-stop", bootstatDir, -1)
	if err != nil {
		return errors.Wrap(err, "failed in parsing uptime of event ui-post-stop")
	}

	// Compute reboot time using system time (quite inaccurate).
	// TODO(b:181084968): Remove this once we are convinced RTC code works better.
	timestampPath := filepath.Join(bootstatDir, "timestamp")
	b, err := ioutil.ReadFile(timestampPath)
	if err != nil {
		return errors.Wrap(err, "failed to read timestamp")
	}
	archiveSystemTimeStr := strings.Split(string(b), "\n")
	archiveUptime, err := parseUptime("archive", bootstatDir, 0)
	if err != nil {
		return errors.Wrap(err, "failed in parsing uptime of event archive")
	}
	archiveSystemTime0, err := strconv.ParseFloat(archiveSystemTimeStr[0], 64)
	if err != nil {
		return errors.Wrapf(err, "error in parsing timestamp value %s", archiveSystemTimeStr[0])
	}
	archiveSystemTime1, err := strconv.ParseFloat(archiveSystemTimeStr[1], 64)
	if err != nil {
		return errors.Wrapf(err, "error in parsing timestamp value %s", archiveSystemTimeStr[1])
	}

	archiveSystemTimeOffset, archiveError := calculateTimeOffset(archiveSystemTime0, archiveSystemTime1, archiveUptime)
	shutdownSystemTime := shutdownUptime + archiveSystemTimeOffset

	nowSystemTime0 := time.Now().Unix()
	nowUptimeStr, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		return errors.Wrap(err, "failed to read system uptime")
	}
	nowSystemTime1 := time.Now().Unix()
	nowUptime, err := strconv.ParseFloat(strings.Fields(string(nowUptimeStr))[0], 64)
	if err != nil {
		return errors.Wrapf(err, "failed to parse system uptime %s", string(nowUptimeStr))
	}
	nowSystemTimeOffset, nowError := calculateTimeOffset(float64(nowSystemTime0), float64(nowSystemTime1), nowUptime)
	bootSystemTime := results.Metrics["seconds_kernel_to_login"] + nowSystemTimeOffset

	rebootTime := bootSystemTime - shutdownSystemTime
	poweronTime := results.Metrics["seconds_power_on_to_login"]
	shutdownTime := rebootTime - poweronTime

	results.Metrics["seconds_reboot_time"] = rebootTime
	results.Metrics["seconds_reboot_error"] = archiveError + nowError
	// TODO(b:181084548): This is somewhat inaccurate, as this excludes power sequencing.
	results.Metrics["seconds_shutdown_time"] = shutdownTime

	// Compute reboot time using RTC (much better accuracy)
	rtcStopPath := filepath.Join(bootstatDir, "sync-rtc-tlsdated-stop")
	stopUptime0, stopUptime1, stopRtcTime, err := parseSyncRtc(rtcStopPath)
	if err != nil {
		return errors.Wrap(err, "failed to parse RTC sync stop time")
	}
	rtcStartPath := filepath.Join(bootstatCurrentDir, "sync-rtc-tlsdated-start")
	startUptime0, startUptime1, startRtcTime, err := parseSyncRtc(rtcStartPath)
	if err != nil {
		return errors.Wrap(err, "failed to parse RTC sync start time")
	}

	stopRtcTimeOffset, stopError := calculateTimeOffset(stopUptime0, stopUptime1, float64(stopRtcTime))
	shutdownTimeRtc := shutdownUptime - stopRtcTimeOffset
	startRtcTimeOffset, startError := calculateTimeOffset(startUptime0, startUptime1, float64(startRtcTime))
	bootTimeRtc := results.Metrics["seconds_kernel_to_login"] - startRtcTimeOffset

	rebootTimeRtc := bootTimeRtc - shutdownTimeRtc

	// TODO(b:181084548): Drop the "_rtc" suffix once we remove the system time metrics above.
	results.Metrics["seconds_reboot_time_rtc"] = rebootTimeRtc
	results.Metrics["seconds_reboot_error_rtc"] = stopError + startError

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
				results.Metrics[diffName] = re - rb
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
