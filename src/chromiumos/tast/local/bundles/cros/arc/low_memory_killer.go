// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LowMemoryKiller,
		Desc:         "Checks that oom_score_adj is set for Chrome and Android processes and that a process is killed by Chrome tab manager before OOM",
		Contacts:     []string{"wvk@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome", "android"},
		Timeout:      4 * time.Minute,
		Data:         []string{"low_memory_killer_manifest.json", "low_memory_killer_background.js"},
	})
}

func LowMemoryKiller(ctx context.Context, s *testing.State) {
	s.Log("Copying extension to temp directory")
	extDir, err := ioutil.TempDir("", "tast.arc.LowMemoryKillerExtension")
	if err != nil {
		s.Fatal("Failed to create temp dir: ", err)
	}
	defer os.RemoveAll(extDir)
	if err := fsutil.CopyFile(s.DataPath("low_memory_killer_manifest.json"), filepath.Join(extDir, "manifest.json")); err != nil {
		s.Fatal("Failed to copy extension manifest: ", err)
	}
	if err := fsutil.CopyFile(s.DataPath("low_memory_killer_background.js"), filepath.Join(extDir, "background.js")); err != nil {
		s.Fatal("Failed to copy extension background.js: ", err)
	}
	extID, err := chrome.ComputeExtensionID(extDir)
	if err != nil {
		s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
	}

	s.Log("Starting browser instance")
	cr, err := chrome.New(ctx,
		chrome.ExtraArgs("--vmodule=memory_kills_monitor=2"),
		chrome.UnpackedExtension(extDir),
		chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	s.Log("Opening tabs")
	tabsConn := make([]*chrome.Conn, 3)
	for i := range tabsConn {
		tabsConn[i], err = cr.NewConn(ctx, "")
		if err != nil {
			s.Fatal("Opening tab failed: ", err)
		}
		defer tabsConn[i].Close()
	}

	s.Log("Connecting to extension background page")
	bgURL := chrome.ExtensionBackgroundPageURL(extID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatalf("Could not connect to extension at %v: %v", bgURL, err)
	}
	defer conn.Close()

	s.Log("Waiting for chrome.processes and chrome.tabs API to become available")
	if err := conn.WaitForExpr(ctx, "chrome.processes"); err != nil {
		s.Fatal("chrome.processes API unavailable: ", err)
	}
	if err := conn.WaitForExpr(ctx, "chrome.tabs"); err != nil {
		s.Fatal("chrome.tabs API unavailable: ", err)
	}
	if err := conn.WaitForExpr(ctx, "TabPids"); err != nil {
		s.Fatal("TabPids object unavailable in extension background page: ", err)
	}

	// Tabs may switch processes soon after loading, so start ARC and example
	// app before checking tab pids, to allow time for any switches.
	s.Log("Starting ARC")
	arcConn, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Could not start ARC: ", err)
	}
	defer arcConn.Close()

	const (
		exampleApp      = "com.android.vending"
		exampleActivity = "com.android.vending.AssetBrowserActivity"
	)
	s.Log("Launching ", exampleApp)
	act, err := arc.NewActivity(arcConn, exampleApp, exampleActivity)
	if err != nil {
		s.Fatalf("Could not launch %v: %v", exampleApp, err)
	}
	defer act.Close()
	if err := act.Start(ctx); err != nil {
		s.Fatalf("Could not start %v: %v", exampleApp, err)
	}

	s.Log("Retrieving PID of app ", exampleApp)
	actPID, err := arc.GetNewestPID(exampleApp)
	if err != nil {
		s.Fatalf("Unable to get pid of %v: %v", exampleApp, err)
	}
	s.Logf("PID of %v: %v", exampleApp, actPID)

	s.Log("Retrieving PIDs of open tabs")
	var tabs []int
	if err := conn.EvalPromise(ctx, "TabPids()", &tabs); err != nil {
		s.Fatal("Retrieving tab pids failed: ", err)
	}
	s.Log("PIDs of Chrome tabs: ", tabs)

	s.Log("Checking OOM scores of app and tabs")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		for _, pid := range append(tabs, actPID) {
			if set, err := checkOOMScoreSet(pid); err != nil {
				return testing.PollBreak(err)
			} else if !set {
				return errors.Errorf("OOM score of pid %v is not set", pid)
			}
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Checking OOM scores failed: ", err)
	}

	s.Log("Checking OOM scores for system and persistent processes")
	const (
		androidHomeApp           = "org.chromium.arc.home"
		examplePersistentApp     = "org.chromium.arc.applauncher"
		exampleSystemProcess     = "netd"
		persistentArcAppOOMScore = -100
	)
	for _, name := range []string{examplePersistentApp, exampleSystemProcess, androidHomeApp} {
		pid, err := arc.GetNewestPID(name)
		if err != nil {
			s.Fatalf("Unable to get pid of %v: %v", name, err)
		}
		if score, err := readOOMScoreAdj(pid); err != nil {
			s.Fatalf("Checking oom score for %v/%v failed: %v", name, pid, err)
		} else if score != persistentArcAppOOMScore {
			s.Errorf("System process %v/%v should have an oom_score_adj of %v, but instead it is %v", name, pid, persistentArcAppOOMScore, score)
		}
	}

	// Run memory-eater and monitor for low memory kills
	const (
		minMemoryMarginMB            = 100
		deviceCriticalMemoryMarginMB = 200
		deviceModerateMemoryMarginMB = 1000
		deviceMarginSysFile          = "/sys/kernel/mm/chromeos-low_mem/margin"
		chromeLogFile                = "/var/log/chrome/chrome"
		kernelOOMKill                = "OOM_KILL"
	)
	var bgJobs []*testexec.Cmd
	defer func() {
		for _, cmd := range bgJobs {
			cmd.Kill()
			cmd.Wait()
		}
	}()

	// Set on-device minimum memory margin before eating memory. This way
	// we are sure to consume below the margin and trigger low memory kills.
	margin := fmt.Sprintf("%d %d", deviceCriticalMemoryMarginMB, deviceModerateMemoryMarginMB)
	if err = ioutil.WriteFile(deviceMarginSysFile, []byte(margin), 0644); err != nil {
		s.Fatalf("Unable to set low-memory margin to %q in file %s: %v", margin, deviceMarginSysFile, err)
	}

	s.Log("Monitoring for low memory kill logs in ", chromeLogFile)
	for {
		available, err := estimatedFreeMemoryMB()
		if err != nil {
			s.Fatal("Reading available memory failed: ", err)
		}
		if available < minMemoryMarginMB {
			s.Logf("Available memory (%vMB) is less than %vMB; stopping memory-eater", available, minMemoryMarginMB)
			s.Fatal("Nothing was killed")
			break
		}
		portion := available / 2
		s.Logf("Consuming %dMB", portion)

		const memoryEaterExecutable = "/usr/local/bin/memory-eater"
		cmd := testexec.CommandContext(ctx, memoryEaterExecutable, "--size", strconv.FormatInt(int64(portion), 10))
		if err := cmd.Start(); err != nil {
			s.Fatal("Could not start memory-eater: ", err)
		}
		bgJobs = append(bgJobs, cmd)

		var killEvent string
		testing.Poll(ctx, func(ctx context.Context) error {
			killEvent, err = findLowMemoryKill(chromeLogFile)
			if err != nil {
				return err
			}
			if killEvent != "" {
				return nil
			}
			return errors.New("could not find memory kill")
		}, &testing.PollOptions{
			Timeout:  2 * time.Second,
			Interval: time.Second,
		})
		// If a memory kill isn't found, the test will continue consuming memory
		// until it hits the margin, and then throw a fatal error.
		if killEvent != "" {
			s.Logf("Memory kill event: %q", killEvent)
			if killEvent == kernelOOMKill {
				s.Fatal("Kernel OOM kill happened before Chrome low-memory kill")
			}
			break
		}
	}
}

// readOOMScoreAdj returns the oom_score_adj of pid.
func readOOMScoreAdj(pid int) (int, error) {
	data, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/oom_score_adj", pid))
	if err != nil {
		return 0, err
	}
	score, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 32)
	if err != nil {
		return 0, err
	}
	return int(score), nil
}

// checkOOMScoreSet checks if oom_score_adj for pid is set.
// The default score is -1000 if nobody has changed its value.
func checkOOMScoreSet(pid int) (bool, error) {
	const nonKillableOOMScore = -1000
	score, err := readOOMScoreAdj(pid)
	if err != nil {
		return false, errors.Wrapf(err, "unable to read oom score for %v", pid)
	}
	return score != nonKillableOOMScore, nil
}

// findLowMemoryKill scans chromeLogPath to find a low memory kill event.
// chromeLogPath should be the path of a Chrome log file, usually
// /var/log/chrome/chrome. If found, the kill event type is returned
// (LOW_MEMORY_KILL_APP, LOW_MEMORY_KILL_TAB, OOM_KILL). If no event is found,
// an empty string is returned.
func findLowMemoryKill(chromeLogPath string) (string, error) {
	var lowMemoryKillPattern *regexp.Regexp = regexp.MustCompile(
		`memory_kills_monitor.* \d+, (LOW_MEMORY_KILL_APP|LOW_MEMORY_KILL_TAB|OOM_KILL)`)

	chromeLog, err := os.Open(chromeLogPath)
	if err != nil {
		return "", err
	}
	defer chromeLog.Close()

	scanner := bufio.NewScanner(chromeLog)
	for scanner.Scan() {
		match := lowMemoryKillPattern.FindStringSubmatch(scanner.Text())
		if match != nil {
			return match[1], nil
		}
	}
	return "", scanner.Err()
}

func estimatedFreeMemoryMB() (int, error) {
	const freeMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/available"
	data, err := ioutil.ReadFile(freeMemorySysFile)
	if err != nil {
		return 0, err
	}
	available, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 32)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to convert %q to integer", data)
	}
	return int(available), nil
}
