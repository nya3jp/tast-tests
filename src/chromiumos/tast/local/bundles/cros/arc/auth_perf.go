// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/common/perf"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/disk"
	"chromiumos/tast/local/power"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type testParam struct {
	username string
	password string
	// maxErrorBootCount is the number of maximum allowed boot errors.
	maxErrorBootCount int
	chromeArgs        []string
	dropCaches        bool
}

var resultPropRegexp = regexp.MustCompile(`OK,(\d+)`)

func init() {
	testing.AddTest(&testing.Test{
		Func: AuthPerf,
		Desc: "Measure auth times in ARC",
		Contacts: []string{
			"khmel@chromium.org", // Original author.
			"niwa@chromium.org",  // Tast port author.
			"arc-performance@google.com",
		},
		Attr:         []string{"group:crosbolt", "crosbolt_perbuild"},
		SoftwareDeps: []string{"chrome", "chrome_internal"},
		// This test steps through opt-in flow 10 times and each iteration takes 20~40 seconds.
		Timeout: 20 * time.Minute,
		Params: []testing.Param{{
			Name:              "unmanaged",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: testParam{
				maxErrorBootCount: 1,
			},
		}, {
			Name:              "unmanaged_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testParam{
				maxErrorBootCount: 3,
			},
		}, {
			Name:              "unmanaged_no_guest_readahead_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.MinMemory(7500)),
			Val: testParam{
				maxErrorBootCount: 3,
				chromeArgs:        []string{"--arcvm-ureadahead-mode=disabled"},
				dropCaches:        true,
			},
		}, {
			Name:              "unmanaged_guest_readahead_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			ExtraHardwareDeps: hwdep.D(hwdep.MinMemory(7500)),
			Val: testParam{
				maxErrorBootCount: 3,
				dropCaches:        true,
			},
		}, {
			Name:              "unmanaged_rt_vcpu_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testParam{
				maxErrorBootCount: 3,
				chromeArgs:        []string{"--enable-arcvm-rt-vcpu"},
			},
		}, {
			Name:              "unmanaged_huge_pages_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testParam{
				maxErrorBootCount: 3,
				chromeArgs:        []string{"--arcvm-use-hugepages"},
			},
		}, {
			Name:              "managed",
			ExtraSoftwareDeps: []string{"android_p"},
			Val: testParam{
				username:          "arc.AuthPerf.managed_username",
				password:          "arc.AuthPerf.managed_password",
				maxErrorBootCount: 1,
			},
		}, {
			Name:              "managed_vm",
			ExtraSoftwareDeps: []string{"android_vm"},
			Val: testParam{
				username:          "arc.AuthPerf.managed_username",
				password:          "arc.AuthPerf.managed_password",
				maxErrorBootCount: 3,
			},
		}},
		VarDeps: []string{
			"arc.AuthPerf.managed_username",
			"arc.AuthPerf.managed_password",
			"ui.gaiaPoolDefault",
		},
	})
}

// measuredValues stores measured times in milliseconds.
type measuredValues struct {
	playStoreShownTime float64
	accountCheckTime   float64
	checkinTime        float64
	networkWaitTime    float64
	signInTime         float64
	bootTime           float64
	energyUsage        *power.RAPLValues
}

// AuthPerf steps through multiple opt-ins and collects checkin time, network wait time,
// sign-in time and Play Store shown time.
// It also reports average, min and max results.
func AuthPerf(ctx context.Context, s *testing.State) {
	const (
		// successBootCount is the number of passing ARC boots to collect results.
		successBootCount = 10
	)

	param := s.Param().(testParam)
	maxErrorBootCount := param.maxErrorBootCount

	args := append(arc.DisableSyncFlags(), "--arc-force-show-optin-ui", "--ignore-arcvm-dev-conf")
	if param.chromeArgs != nil {
		args = append(args, param.chromeArgs...)
	}

	var gaia chrome.Option
	if param.username != "" {
		gaia = chrome.GAIALogin(chrome.Creds{User: s.RequiredVar(param.username), Pass: s.RequiredVar(param.password)})
	} else {
		gaia = chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault"))
	}

	// TODO(crbug.com/995869): Remove set of flags to disable app sync, PAI, locale sync, Play Store auto-update.
	cr, err := chrome.New(ctx,
		chrome.ARCSupported(),
		chrome.RestrictARCCPU(),
		gaia,
		chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating test API connection failed: ", err)
	}
	defer tconn.Close()

	errorCount := 0
	var playStoreShownTimes []float64
	var accountCheckTimes []float64
	var checkinTimes []float64
	var networkWaitTimes []float64
	var signInTimes []float64
	var bootTimes []float64
	var energyUsage []*power.RAPLValues

	for len(playStoreShownTimes) < successBootCount {
		s.Logf("Running ARC opt-in iteration #%d out of %d",
			len(playStoreShownTimes)+1, successBootCount)

		v, err := bootARC(ctx, s, cr, tconn)
		logcatName := ""
		if err == nil {
			// Append Play Store shown time in ms for quick reference.
			logcatName = fmt.Sprintf("logcat_ok_%d.log", int(v.playStoreShownTime))
		} else {
			logcatName = fmt.Sprintf("logcat_error_%d.log", errorCount)
		}
		logcatFilePath := filepath.Join(s.OutDir(), logcatName)
		if err := dumpLogcat(ctx, s, logcatFilePath); err != nil {
			s.Log("Failed to dump logcat: ", err)
		}
		if err != nil {
			errorCount++
			s.Logf("Error found during the ARC boot: %v - logcat was dumped to %s",
				err, logcatFilePath)

			if errorCount > maxErrorBootCount {
				s.Fatalf("Too many ARC boot errors (%d time(s)), last error: %q", errorCount, err)
			}
			continue
		}

		playStoreShownTimes = append(playStoreShownTimes, v.playStoreShownTime)
		accountCheckTimes = append(accountCheckTimes, v.accountCheckTime)
		checkinTimes = append(checkinTimes, v.checkinTime)
		networkWaitTimes = append(networkWaitTimes, v.networkWaitTime)
		signInTimes = append(signInTimes, v.signInTime)
		bootTimes = append(bootTimes, v.bootTime)
		if v.energyUsage != nil {
			energyUsage = append(energyUsage, v.energyUsage)
		}
	}

	perfValues := perf.NewValues()

	// Array for generating comma separated results.
	var resultForLog []string

	version, err := chromeOSVersion()
	if err != nil {
		s.Fatal("Can't get ChromeOS version: ", err)
	}
	resultForLog = append(resultForLog, version, strconv.Itoa(len(playStoreShownTimes)))

	reportResult := func(name string, samples []float64) {
		for _, x := range samples {
			perfValues.Append(perf.Metric{
				Name:      name,
				Unit:      "milliseconds",
				Direction: perf.SmallerIsBetter,
				Multiple:  true,
			}, x)
		}

		var sum float64
		var max float64
		min := math.MaxFloat64
		for _, x := range samples {
			sum += x
			max = math.Max(x, max)
			min = math.Min(x, min)
		}
		average := sum / float64(len(samples))

		s.Logf("%s - average: %d, range %d - %d based on %d samples",
			name, int(average), int(min), int(max), len(samples))
		resultForLog = append(resultForLog, strconv.Itoa(int(min)),
			strconv.Itoa(int(average)), strconv.Itoa(int(max)))
	}

	reportResult("play_store_shown_time", playStoreShownTimes)
	reportResult("account_check_time", accountCheckTimes)
	reportResult("checkin_time", checkinTimes)
	reportResult("network_wait_time", networkWaitTimes)
	reportResult("sign_in_time", signInTimes)
	reportResult("boot_time", bootTimes)

	for _, rapl := range energyUsage {
		rapl.ReportPerfMetrics(perfValues, "power_")
	}

	// Outputs comma separated results to the log.
	// It is used in many sheet tables to automate paste operations.
	s.Log(strings.Join(resultForLog, ","))

	if err = perfValues.Save(s.OutDir()); err != nil {
		s.Fatal("Failed saving perf data: ", err)
	}
}

// readResultProp reads the system property set by ARC to save provisioning flow step result.
func readResultProp(ctx context.Context, a *arc.ARC, propName string) (float64, error) {
	out, err := a.Command(ctx, "getprop", propName).Output(testexec.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get the result property %q", propName)
	}

	outStr := string(out)
	m := resultPropRegexp.FindStringSubmatch(outStr)
	if m == nil {
		return 0, errors.Errorf("Result property %q could not be parsed: %q", propName, outStr)
	}

	return strconv.ParseFloat(m[1], 64)
}

// bootARC performs one ARC boot iteration, opt-out and opt-in again.
// It calculates the time when the Play Store appears and set of ARC auth times.
func bootARC(ctx context.Context, s *testing.State, cr *chrome.Chrome, tconn *chrome.TestConn) (measuredValues, error) {
	var v measuredValues

	// Opt out.
	if err := optin.SetPlayStoreEnabled(ctx, tconn, false); err != nil {
		return v, err
	}

	s.Log("Waiting for Android data directory to be removed")
	if err := waitForAndroidDataRemoved(ctx); err != nil {
		return v, err
	}

	s.Log("Waiting for ARC to stop")
	if err := waitForARCStopped(ctx); err != nil {
		return v, err
	}

	// Drop host OS caches if test config requires it for predictable results.
	if s.Param().(testParam).dropCaches {
		if err := disk.DropCaches(ctx); err != nil {
			return v, errors.Wrap(err, "failed to drop caches")
		}
	}

	if _, err := power.WaitUntilCPUCoolDown(ctx, power.DefaultCoolDownConfig(power.CoolDownPreserveUI)); err != nil {
		s.Fatal("Failed to wait until CPU is cooled down: ", err)
	}

	energyBefore, err := power.NewRAPLSnapshot()
	if err != nil {
		s.Error("Energy status is not available for this board")
	}

	startTime := time.Now()

	// Opt in. From performance perspective, optin longer than 90 seconds is failure.
	// This also aligned with global 20 minute timeout.
	shorterCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	s.Log("Waiting for ARC opt-in flow to complete")
	if err := optin.Perform(shorterCtx, cr, tconn); err != nil {
		return v, err
	}

	s.Log("Waiting for Play Store window to be shown")
	if err := optin.WaitForPlayStoreShown(shorterCtx, tconn, time.Minute); err != nil {
		return v, err
	}

	v.playStoreShownTime = time.Now().Sub(startTime).Seconds() * 1000

	// energyBefore could be nil (not considered an error) on non-Intel CPUs.
	if energyBefore != nil {
		v.energyUsage, err = energyBefore.DiffWithCurrentRAPL()
		if err != nil {
			s.Fatal("Failed to get RAPL values: ", err)
		}
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		return v, errors.Wrap(err, "failed to connect ARC")
	}
	defer a.Close(ctx)

	// Read sign-in results via properties.
	if v.signInTime, err = readResultProp(ctx, a, "dev.arc.accountsignin.result"); err != nil {
		return v, err
	}
	if v.accountCheckTime, err = readResultProp(ctx, a, "dev.arc.accountcheck.result"); err != nil {
		return v, err
	}
	if v.networkWaitTime, err = readResultProp(ctx, a, "dev.arc.networkwait.result"); err != nil {
		return v, err
	}
	if v.checkinTime, err = readResultProp(ctx, a, "dev.arc.accountcheckin.result"); err != nil {
		return v, err
	}

	// Calculate
	//   * kernel boot time as a difference init process started and preStartTime.
	var ret struct {
		Provisioned bool `json:"provisioned"`
		TOSNeeded   bool `json:"tosNeeded"`
		// mini-ARC started
		PreStartTime float64 `json:"preStartTime"`
		// ARC started.
		StartTime float64 `json:"startTime"`
	}

	if err := tconn.Eval(ctx, "tast.promisify(chrome.autotestPrivate.getArcState)()", &ret); err != nil {
		return v, errors.Wrap(err, "failed to run getArcState()")
	}
	if ret.PreStartTime == 0 {
		return v, errors.New("cannot get ARC pre-start time")
	}

	output, err := a.Command(ctx, "stat", "-c", "%z", "/proc/1").Output(testexec.DumpLogOnError)
	if err != nil {
		return v, errors.Wrap(err, "failed to get init process start time")
	}
	timeStr := strings.TrimSpace(string(output))
	testing.ContextLogf(ctx, "ARC pre-start time: %f UNIX ms, and /init proc time %q", ret.PreStartTime, timeStr)
	preStartTimeNS := (int64)(ret.PreStartTime * 1000000.0)
	tPreStart := time.Unix(preStartTimeNS/1000000000, preStartTimeNS%1000000000)
	tInit, err := time.Parse("2006-01-02 15:04:05.999999999 -0700", timeStr)
	if err != nil {
		tInit, err = time.ParseInLocation("2006-01-02 15:04:05.999999999", timeStr, time.Local)
	}
	if err != nil {
		return v, errors.Wrap(err, "failed to parse time")
	}
	v.bootTime = float64(tInit.Sub(tPreStart).Milliseconds())
	return v, nil
}

// waitForAndroidDataRemoved waits for Android data directory to be removed.
func waitForAndroidDataRemoved(ctx context.Context) error {
	const androidDataDirPath = "/opt/google/containers/android/rootfs/android-data/data"

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(androidDataDirPath); !os.IsNotExist(err) {
			return errors.New("Android data still exists")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for Android data directory to be removed")
	}
	return nil
}

// waitForARCStopped waits for ARC to stop.
func waitForARCStopped(ctx context.Context) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if exist, err := arc.InitExists(); err != nil {
			return testing.PollBreak(err)
		} else if exist {
			return errors.New("ARC is not yet stopped")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second}); err != nil {
		return errors.Wrap(err, "failed to wait for ARC to stop")
	}
	return nil
}

// chromeOSVersion returns the Chrome OS version string. (e.g. "12345.0.0")
func chromeOSVersion() (string, error) {
	m, err := lsbrelease.Load()
	if err != nil {
		return "", err
	}
	val, ok := m[lsbrelease.Version]
	if !ok {
		return "", errors.Errorf("failed to find %s in /etc/lsb-release", lsbrelease.Version)
	}
	return val, nil
}

// dumpLogcat dumps logcat output to a log file in the test result directory.
func dumpLogcat(ctx context.Context, s *testing.State, filePath string) error {
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		return errors.Wrap(err, "failed to connect ARC")
	}
	defer a.Close(ctx)

	logFile, err := os.Create(filePath)
	if err != nil {
		return errors.Wrap(err, "failed to create log file")
	}
	defer logFile.Close()
	cmd := a.Command(ctx, "logcat", "-d")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	return cmd.Run()
}
