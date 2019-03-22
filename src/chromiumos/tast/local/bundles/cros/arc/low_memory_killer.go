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

	"github.com/shirou/gopsutil/process"

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
		SoftwareDeps: []string{"chrome_login", "android"},
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

	s.Log("Retrieving PIDs of open tabs")
	if err := conn.WaitForExpr(ctx, "TabPids"); err != nil {
		s.Fatal("TabPids object unavailable in extension background page: ", err)
	}
	var tabs []int
	if err := conn.EvalPromise(ctx, "TabPids()", &tabs); err != nil {
		s.Fatal("Retrieving tab pids failed: ", err)
	}
	s.Log("The list of Chrome tabs: ", tabs)

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
	actPID, err := getNewestPID(exampleApp)
	if err != nil {
		s.Fatalf("Unable to get pid of %v: %v", exampleApp, err)
	}

	s.Log("Checking OOM scores of app and tabs")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		for _, pid := range append(tabs[:], actPID) {
			set, err := checkOOMScoreSet(pid)
			if err != nil {
				return testing.PollBreak(err)
			}
			if !set {
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
		pid, err := getNewestPID(name)
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
		minMemoryMarginMB = 100
		chromeLogFile     = "/var/log/chrome/chrome"
	)
	var killEvent string
	bgJobs := []*testexec.Cmd{}
	killBgJobs := func() {
		for _, cmd := range bgJobs {
			cmd.Kill()
			cmd.Wait()
		}
	}
	defer killBgJobs()

	chromeLog, err := os.Open(chromeLogFile)
	if err != nil {
		s.Fatalf("Unable to read log file %v: %v", chromeLogFile, err)
	}
	defer chromeLog.Close()

	s.Log("Monitoring for low memory kill logs in ", chromeLogFile)
	for {
		available, err := estimatedFreeMemoryMB()
		if err != nil {
			s.Fatal("Reading available memory failed: ", err)
		}
		if available < minMemoryMarginMB {
			s.Logf("Available memory (%vMB) is less than %v; stopping memory-eater", available, minMemoryMarginMB)
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

		testing.Poll(ctx, func(ctx context.Context) error {
			killEvent, err = findLowMemoryKill(chromeLog)
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
		if killEvent != "" {
			s.Logf("Memory kill event: %q", killEvent)
			break
		}
	}
	killBgJobs()
	bgJobs = nil
	if killEvent == "" {
		s.Fatal("Nothing was killed")
	}
}

// getNewestPID returns the newest PID with name.
func getNewestPID(name string) (int, error) {
	procs, err := process.Processes()
	if err != nil {
		return 0, err
	}
	var mostRecentMatch *process.Process
	var mostRecentCreateTime int64
	for _, proc := range procs {
		if cl, err := proc.Cmdline(); err != nil || !strings.Contains(cl, name) {
			continue
		}
		createTime, err := proc.CreateTime()
		if err != nil {
			continue
		}
		if mostRecentMatch == nil || createTime > mostRecentCreateTime {
			mostRecentMatch = proc
			mostRecentCreateTime = createTime
		}
	}
	if mostRecentMatch == nil {
		return 0, errors.Errorf("unable to find process with name %v", name)
	}
	return int(mostRecentMatch.Pid), nil
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
	var score int
	var err error
	score, err = readOOMScoreAdj(pid)
	if err != nil {
		return false, errors.Wrapf(err, "unable to read oom score for %v", pid)
	}
	return score != nonKillableOOMScore, nil
}

// findLowMemoryKill scans log to find a low memory kill event. log should
// be a Chrome log file, usually /var/log/chrome/chrome
func findLowMemoryKill(log *os.File) (string, error) {
	var lowMemoryKillPattern *regexp.Regexp = regexp.MustCompile(
		`memory_kills_monitor.* (\d+), (LOW_MEMORY_KILL_APP|LOW_MEMORY_KILL_TAB|OOM_KILL)`)
	scanner := bufio.NewScanner(log)
	for scanner.Scan() {
		match := lowMemoryKillPattern.FindString(scanner.Text())
		if match != "" {
			return match, nil
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
