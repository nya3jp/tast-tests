// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/uuid"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/services/cros/platform"
	"chromiumos/tast/testing"
)

const (
	uptimePrefix = "uptime-"
	diskPrefix   = "disk-"

	firmwareTimeFile = "/tmp/firmware-boot-time"

	bootstatArchiveGlob = "/var/log/metrics/shutdown.[0-9]*"
	bootchartKConfig    = "cros_bootchart"

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
	//   kernel_to_signin_start - The moment when LoadPage(loginSceenURL)
	//     is called, i.e. initialization starts.
	//   kernel_to_signin_wait - The moment when UI thread has finished signin
	//     screen initialization and now waits until JS sends "ready" event.
	//   kernel_to_signin_users - The moment when UI thread receives "ready" from
	//     JS code. So V8 is initialized and running, etc...
	//   kernel_to_login - The moment when user can actually see signin UI.
	//   kernel_to_android_start - The moment when Android is started.
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
	}

	uptimeFileGlob = fmt.Sprintf("/tmp/%s*", uptimePrefix)
	diskFileGlob   = fmt.Sprintf("/tmp/%s*", diskPrefix)

	// The name of this file has changed starting with linux-3.19.
	// Use a glob to snarf up all existing records.
	ramOopsFileGlob = "/sys/fs/pstore/console-ramoops*"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			platform.RegisterBootPerfServiceServer(srv, &BootPerfService{s})
		},
	})
}

// BootPerfService implements tast.cros.platform.BootPerfService
type BootPerfService struct {
	s *testing.ServiceState
}

// waitUntilBootComplete is a helper function to wait until boot complete and
// we are ready to collect boot metrics.
func waitUntilBootComplete(ctx *context.Context) error {
	return testing.Poll(*ctx, func(context.Context) error {
		// Check that bootstat files are available.
		for _, k := range eventMetrics {
			key := "/tmp/" + uptimePrefix + k.EventName
			_, err := os.Stat(key)
			if k.Required && os.IsNotExist(err) {
				return errors.Errorf("waiting for bootstat file %s", key)
			}

			key = "/tmp/" + diskPrefix + k.EventName
			_, err = os.Stat(key)
			if k.Required && os.IsNotExist(err) {
				return errors.Errorf("waiting for bootstat file %s", key)
			}
		}

		// Check that bootchart has stopped.
		cmd := testexec.CommandContext(*ctx, "/sbin/initctl", "status", "bootchart")
		out, err := cmd.Output()
		if err != nil {
			return errors.Wrap(err, "failed to get status of bootchart")
		}
		if !strings.Contains(string(out), "stop/waiting") {
			return errors.New("bootchart running")
		}

		// Check that system-services is started.
		cmd = testexec.CommandContext(*ctx, "/sbin/initctl", "status", "system-services")
		out, err = cmd.Output()
		if err != nil {
			return errors.Wrap(err, "failed to get status of system-services")
		}
		if !strings.Contains(string(out), "start/running") {
			return errors.New("system-services not started")
		}

		return nil
	}, &testing.PollOptions{
		Timeout:  30 * time.Second,
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
	for _, l := range lines {
		f := strings.Fields(l)
		if fieldNum >= len(f) {
			continue
		}
		if s, err := strconv.ParseFloat(f[fieldNum], 64); err == nil {
			result = append(result, s)
		}
	}

	return result, err
}

// parseUptime returns time since boot for a bootstat event.
func parseUptime(eventName, bootstatDir string, index int) (float64, error) {
	eventFile := bootstatDir + uptimePrefix + eventName
	if val, err := parseBootstat(eventFile, 0); err != nil {
		return 0.0, err
	}
	if index >= 0 {
		return val[index], nil
	}
	// Like negative index in python.
	return val[len(val)+index], nil

}

// gatherTimeMetrics reads and reports boot time metrics. It reads
// "seconds since kernel startup" from the bootstat files for the events named
// in |eventMetrics|, and stores the values as perf metrics.  The following
// metrics are recorded:
//   * seconds_kernel_to_startup
//   * seconds_kernel_to_startup_done
//   * seconds_kernel_to_chrome_exec
//   * seconds_kernel_to_chrome_main
//   * seconds_kernel_to_login
//   * seconds_kernel_to_network
// All of these metrics are considered mandatory, except for
// seconds_kernel_to_network.
func gatherTimeMetrics(ctx *context.Context, results *platform.GetBootPerfMetricsResponse) error {
	for _, k := range eventMetrics {
		key := "seconds_" + k.MetricName
		val, err := parseUptime(k.EventName, "/tmp/", 0)
		if err != nil {
			if k.Required {
				return errors.Wrapf(err, "failed in gather time for %s", k.EventName)
			}
			// Failed in getting the non-required metric. Skip.
		} else {
			//	testing.ContextLogf(*ctx, "got %s: %f", key, val)
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
		t, err := parseUptime(e, "/tmp/", 0)
		if err != nil {
			continue
		}

		results.Metrics[metricName] = t
		if t < firstNetworkReadyTime {
			firstNetworkReadyTime = t
			results.Metrics["seconds_kernel_to_network"] = firstNetworkReadyTime
		}
	}
	return nil
}

// parseDiskstat returns sectors read since boot for a bootstat event.
func parseDiskstat(eventName, bootstatDir string, index int) (float64, error) {
	eventFile := bootstatDir + diskPrefix + eventName
	if val, err := parseBootstat(eventFile, 2); err != nil {
		return 0.0, err
	}
	return val[index], nil
}

// gatherDiskMetrics reads and reports disk read metrics.
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
func gatherDiskMetrics(results *platform.GetBootPerfMetricsResponse) {
	// We expect an error when reading disk statistics for the "chrome-main" event because Chrome (not bootstat) generates that event, and it doesn't include the disk statistics.
	// We get around that by ignoring all errors.
	for _, k := range eventMetrics {
		key := "rdbytes_" + k.MetricName
		val, err := parseDiskstat(k.EventName, "/tmp/", 0)
		if err == nil {
			results.Metrics[key] = val * sectorSize
		} // else skip the error and continue.
	}
}

// round is an utility function for rounding |x| to 2 decimal places.
func round(x float64) float64 {
	return math.Round(x*100) / 100
}

// gatherFirmwareBootTime reads and reports firmware startup time. The boot
// process writes the firmware startup time to the file named in
// |firmwareTimeFile|. Read the time from that file, and record it as the metric
// seconds_power_on_to_kernel.
func gatherFirmwareBootTime(results *platform.GetBootPerfMetricsResponse) error {
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
	bootTimeval := round((t0+t1)/2) - tUptime
	error := (t1 - t0) / 2
	return bootTimeval + event, error
}

// gatherRebootMetrics reads and reports shutdown and reboot times. The shutdown
// process saves all bootstat files in /var/log, plus it saves a timestamp file
// that can be used to convert "time since boot" into times in UTC.  Read the
// saved files from the most recent shutdown, and use them to calculate the time
// spent from the start of that shutdown until the completion of the most recent
// boot.
// This function records these metrics:
//   * seconds_shutdown_time
//   * seconds_reboot_time
//   * seconds_reboot_error
func gatherRebootMetrics(results *platform.GetBootPerfMetricsResponse) error {
	bootstatArchives, err := filepath.Glob(bootstatArchiveGlob)
	if err != nil {
		return nil
	}

	// max() for string comparison.
	max := func(in []string) string {
		m := ""
		for _, s := range in {
			if strings.Compare(s, m) > 0 {
				m = s
			}
		}
		return m
	}
	bootstatDir := max(bootstatArchives) + string(os.PathSeparator)

	bootID, err := ioutil.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return errors.Wrap(err, "failed to read boot_id")
	}

	didrunPath := bootstatDir + "bootperf_ran"
	_, err = os.Stat(didrunPath)
	if os.IsNotExist(err) {
		_ = ioutil.WriteFile(didrunPath, bootID, 0644)
	} else {
		b, err := ioutil.ReadFile(didrunPath)
		if err == nil && strings.Compare(string(b), string(bootID)) != 0 {
			// Returns an error on boot id mismatch
			return errors.Errorf("boot id mismatch: %s != %s", string(b), string(bootID))
		}
	}

	timestampPath := bootstatDir + "timestamp"

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
		return errors.Wrapf(err, "error in parsing timestamp value %s", archiveTs[0])
	}
	archiveT1, err := strconv.ParseFloat(archiveTs[1], 64)
	if err != nil {
		return errors.Wrapf(err, "error in parsing timestamp value %s", archiveTs[1])
	}

	shutdownTimeval, shutdownError := calculateTimeval(shutdownUptime, archiveT0, archiveT1, archiveUptime)

	bootT0 := time.Now().Unix()
	uptime, _ := ioutil.ReadFile("/proc/uptime")
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

func calculateDiff(results *platform.GetBootPerfMetricsResponse) {
	barriers := []string{"startup", "chrome_exec", "login"}
	types := []string{"seconds", "rdbytes"}
	for i, b := range barriers[:len(barriers)-1] {
		for _, t := range types {
			begin := fmt.Sprintf("%s_kernel_to_%s", t, b)
			end := fmt.Sprintf("%s_kernel_to_%s", t, barriers[i+1])

			rb, ok1 := results.Metrics[begin]
			if re, ok2 := results.Metrics[end]; ok1 && ok2 {
				diffName := fmt.Sprintf("%s_%s_to_%s", t, b, barriers[i+1])
				results.Metrics[diffName] = round(re - rb)
			}
		}
	}
}

// gatherTimestampFiles gathers content of raw data files to be returned to the
// client.
func gatherTimestampFiles(raw map[string][]byte) {
	statlist, _ := filepath.Glob(uptimeFileGlob)
	diskFiles, _ := filepath.Glob(diskFileGlob)
	statlist = append(statlist, diskFiles...)

	for _, f := range statlist {
		b, err := ioutil.ReadFile(f)
		if err == nil {
			raw[path.Base(f)] = b
		}
	}

	b, err := ioutil.ReadFile(firmwareTimeFile)
	if err == nil {
		raw[path.Base(firmwareTimeFile)] = b
	}
}

// gatherConsoleRamoops gathers console_ramoops from previous reboot.
func gatherConsoleRamoops(raw map[string][]byte) {
	list, _ := filepath.Glob(ramOopsFileGlob)
	for _, f := range list {
		b, err := ioutil.ReadFile(f)
		if err == nil {
			raw[path.Base(f)] = b
		}
	}
}

// getRootPartition returns root partition index by running `rootdev -s` and
// converting the last digit to partition index.
// For example, "/dev/mmcblk0p3" corresponds to root partition index 2 that is
// used in modifying the kernel args.
func getRootPartition() (string, error) {
	out, err := exec.Command("/usr/bin/rootdev", "-s").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed in getting the root partition")
	}

	re := regexp.MustCompile(`dev/.*\dp(\d+)`)
	groups := re.FindStringSubmatch(string(out))
	if len(groups) != 2 {
		return "", errors.Errorf("failed to parse root partition from %s", out)
	}

	i, _ := strconv.Atoi(groups[1]) // We captured \d+ and that shouldn't fail Atoi

	return strconv.Itoa(i - 1), nil
}

// editKernelArgs is a helper function for editing kernel args. Function |f|
// performs the editing action by transforming the content of saved config.
func editKernelArgs(f func([]byte) []byte) error {
	part, err := getRootPartition()
	if err != nil {
		return err
	}

	// Save the current boot config to |prefix|.|part| (make_dev_ssd.sh saves the content to a file named |prefix|.|part|).
	prefix := "/tmp/" + uuid.New().String()
	err = exec.Command("/usr/share/vboot/bin/make_dev_ssd.sh", "--save_config", prefix, "--partitions", part).Run()

	if err != nil {
		return errors.Wrap(err, "failed to save boot config")
	}

	savedKconf := fmt.Sprintf("%s.%s", prefix, part)
	savedKConfstr, err := ioutil.ReadFile(savedKconf)
	if err != nil {
		return errors.Wrap(err, "failed to read saved kernel config")
	}

	// Transform the content.
	savedKConfstr = f(savedKConfstr)
	err = ioutil.WriteFile(savedKconf, savedKConfstr, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to edit saved kernel config")
	}

	err = exec.Command("/usr/share/vboot/bin/make_dev_ssd.sh", "--set_config", prefix, "--partitions", part).Run()
	if err != nil {
		return errors.Wrap(err, "failed to set boot config")
	}

	return nil
}

// EnableBootchart enables bootchart by adding "cros_bootchart" to kernel
// arguments.
func (*BootPerfService) EnableBootchart(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if err := editKernelArgs(func(str []byte) []byte {
		s := string(str)
		if strings.Contains(s, bootchartKConfig) {
			// Bootchart already enabled: leave the kernel args as is.
			return str
		}

		// Append "cros_bootchart" to kernel args.
		return []byte(fmt.Sprintf("%s %s", s, bootchartKConfig))
	}); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// DisableBootchart Disables bootchart by removing "cros_bootchart" from kernel
// arguments.
func (*BootPerfService) DisableBootchart(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	if err := editKernelArgs(func(str []byte) []byte {
		s := string(str)
		if !strings.Contains(s, bootchartKConfig) {
			// Bootchart already disabled: leave the kernel args as is.
			return str
		}

		// Remove "cros_bootchart" from kernel args.
		return []byte(strings.ReplaceAll(s, " "+bootchartKConfig, ""))
	}); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

// GetBootPerfMetrics gathers recorded timing and disk usage statistics during
// boot time. The test calculates some or all of the following metrics:
//   * seconds_kernel_to_startup
//   * seconds_kernel_to_startup_done
//   * seconds_kernel_to_chrome_exec
//   * seconds_kernel_to_chrome_main
//   * seconds_kernel_to_signin_start
//   * seconds_kernel_to_signin_wait
//   * seconds_kernel_to_signin_users
//   * seconds_kernel_to_login
//   * seconds_kernel_to_network
//   * seconds_startup_to_chrome_exec
//   * seconds_chrome_exec_to_login
//   * rdbytes_kernel_to_startup
//   * rdbytes_kernel_to_startup_done
//   * rdbytes_kernel_to_chrome_exec
//   * rdbytes_kernel_to_chrome_main
//   * rdbytes_kernel_to_login
//   * rdbytes_startup_to_chrome_exec
//   * rdbytes_chrome_exec_to_login
//   * seconds_power_on_to_kernel
//   * seconds_power_on_to_login
//   * seconds_shutdown_time
//   * seconds_reboot_time
//   * seconds_reboot_error
func (*BootPerfService) GetBootPerfMetrics(ctx context.Context, _ *empty.Empty) (*platform.GetBootPerfMetricsResponse, error) {
	out := new(platform.GetBootPerfMetricsResponse)
	out.Metrics = make(map[string]float64)

	// perform a testing.Poll() to wait for boot perf artifacts to show up.
	testing.ContextLog(ctx, "wait until boot complete")
	err := waitUntilBootComplete(&ctx)
	if err != nil {
		return out, err
	}

	testing.ContextLog(ctx, "gather boot time metrics")
	err = gatherTimeMetrics(&ctx, out)
	if err != nil {
		return out, err
	}

	testing.ContextLog(ctx, "gather boot disk read metrics")
	gatherDiskMetrics(out)

	testing.ContextLog(ctx, "gather firmware boot metric")
	err = gatherFirmwareBootTime(out)
	if err != nil {
		return out, err
	}

	testing.ContextLog(ctx, "gather reboot metrics")
	err = gatherRebootMetrics(out)
	if err != nil {
		return out, err
	}

	testing.ContextLog(ctx, "calculate diff")
	calculateDiff(out)

	return out, nil
}

// GetBootPerfRawData gathers raw data used in calculating boot perf metrics for
// debugging.
func (*BootPerfService) GetBootPerfRawData(ctx context.Context, _ *empty.Empty) (*platform.GetBootPerfRawDataResponse, error) {
	out := new(platform.GetBootPerfRawDataResponse)
	// Passed cached bootstat raw data to the client.
	raw := make(map[string][]byte)

	gatherTimestampFiles(raw)
	gatherConsoleRamoops(raw)

	out.RawData = raw
	return out, nil
}
