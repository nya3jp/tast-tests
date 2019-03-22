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

const (
	exampleAppToLaunch           = "com.android.vending"
	exampleActivityToLaunch      = "com.android.vending.AssetBrowserActivity"
	androidHomeApp               = "org.chromium.arc.home"
	examplePersistentApp         = "org.chromium.arc.applauncher"
	exampleSystemProcess         = "netd"
	memoryEaterExecutable        = "/usr/local/bin/memory-eater"
	nonKillableOOMScore      int = -1000
	persistentArcAppOOMScore int = -100
	minMemoryMarginMB        int = 100
	chromeLogFile                = "/var/log/chrome/chrome"
	chromeSecondaryLogFile       = "/var/log/chrome/chrome.PREVIOUS"
)

var lowMemoryKillPattern *regexp.Regexp = regexp.MustCompile(
	`memory_kills_monitor.* (\d+), (LOW_MEMORY_KILL_APP|LOW_MEMORY_KILL_TAB|OOM_KILL)`)

func init() {
	testing.AddTest(&testing.Test{
		Func: LowMemoryKiller,
		Desc: `It checks
1. /proc/<pid>/oom_score_adj is set for Chrome tabs and Android apps
2. Any background Chrome tab is killed in lowmemory condition`,
		Contacts:     []string{"wvk@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"chrome_login", "android"},
		Timeout:      4 * time.Minute,
		Data:         []string{"low_memory_killer_manifest.json", "low_memory_killer_background.js"},
	})
}

func LowMemoryKiller(ctx context.Context, s *testing.State) {
	var err error
	var extDir, extID string

	// Copy extension to temp directory
	s.Log("Copying extension to temp directory")
	extDir, err = ioutil.TempDir("", "tast.arc.LowMemoryKillerExtension")
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
	extID, err = chrome.ComputeExtensionID(extDir)
	if err != nil {
		s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
	}

	// Launch Chrome
	s.Log("Starting browser instance")
	var cr *chrome.Chrome
	cr, err = chrome.New(ctx,
		chrome.ExtraArgs("--vmodule=memory_kills_monitor=2"),
		chrome.UnpackedExtension(extDir),
		chrome.ARCEnabled())
	if err != nil {
		s.Fatal("Failed to connect to Chrome: ", err)
	}
	defer cr.Close(ctx)

	// Open a few test tabs
	s.Log("Opening tabs")
	tabsConn := make([]*chrome.Conn, 3)
	for i := 0; i < 3; i++ {
		tabsConn[i], err = cr.NewConn(ctx, "https://maps.google.com")
		if err != nil {
			s.Fatal("Opening tab failed: ", err)
		}
		defer tabsConn[i].Close()
	}

	// Connect to extension and retrieve tab pids
	s.Log("Connecting to extension background page")
	bgURL := chrome.ExtensionBackgroundPageURL(extID)
	var conn *chrome.Conn
	conn, err = cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
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
	if err := conn.Exec(ctx, "var tabGetter = Object.create(TabGetter); tabGetter.getAllTabs()"); err != nil {
		s.Fatal("Could not execute TabGetter.getAllTabs(): ", err)
	}
	if err := conn.WaitForExpr(ctx, "tabGetter.done == true"); err != nil {
		s.Fatal("Could not wait for TabGetter.done == true: ", err)
	}
	var tabs []int
	if err := conn.Eval(ctx, "tabGetter.tabPids", &tabs); err != nil {
		s.Fatal("Could not retrieve tab pids: ", err)
	}
	s.Log("The list of Chrome tabs: ", tabs)

	// Setup ARC and launch Android apps
	var arcConn *arc.ARC
	var act *arc.Activity
	s.Log("Starting ARC")
	arcConn, err = arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Could not start ARC: ", err)
	}
	defer arcConn.Close()
	s.Log("Launching ", exampleAppToLaunch)
	act, err = arc.NewActivity(arcConn, exampleAppToLaunch, exampleActivityToLaunch)
	if err != nil {
		s.Fatalf("Could not launch %v: %v", exampleAppToLaunch, err)
	}
	defer act.Close()
	if err := act.Start(ctx); err != nil {
		s.Fatalf("Could not start %v: %v", exampleAppToLaunch, err)
	}
	actPID := getNewestPID(ctx, s, exampleAppToLaunch)

	// Check that OOM scores are set
	s.Log("Checking OOM scores of app and tabs")
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		all := true
		for _, pid := range append(tabs[:], actPID) {
			var set bool
			set, err = checkOOMScoreSet(pid)
			if err != nil {
				return testing.PollBreak(err)
			}
			all = all && set
		}
		if all {
			return nil
		}
		return errors.New("Not all OOM scores are set correctly")

	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Checking oom scores failed: ", err)
	}

	s.Log("Checking OOM scores for system and persistent processes")
	for _, name := range []string{examplePersistentApp, exampleSystemProcess, androidHomeApp} {
		pid := getNewestPID(ctx, s, name)
		if score, err := readOOMScoreAdj(pid); err != nil {
			s.Fatalf("Checking oom score for %v/%v failed: %v", name, pid, err)
		} else {
			if score != persistentArcAppOOMScore {
				s.Errorf("System process %v/%v should have an oom_score_adj of %v, but instead it is %v", name, pid, persistentArcAppOOMScore, score)
			}
		}
	}

	// Run memory-eater and monitor for low memory kills
	bgJobs := []*testexec.Cmd{}
	var killEvent string
	var chromeLog *os.File
	if chromeLog, err = os.Open(chromeLogFile); err != nil {
		s.Fatalf("Unable to read log file %v: %v", chromeLogFile, err)
	}
	defer chromeLog.Close()
	s.Log("Monitoring for low memory kill logs in ", chromeLogFile)
	for {
		var available int
		available, err = estimatedFreeMemoryMB()
		if err != nil {
			s.Fatal("Reading available memory failed: ", err)
		}
		if available < minMemoryMarginMB {
			s.Logf("Available memory (%vMB) is less than %v; stopping memory-eater", available, minMemoryMarginMB)
			break
		}
		portion := available / 2
		s.Logf("Consuming %dMB", portion)

		cmd := testexec.CommandContext(ctx, memoryEaterExecutable, "--size", strconv.FormatInt(int64(portion), 10))
		if err := cmd.Start(); err != nil {
			s.Fatal("Could not start memory-eater: ", err)
		}
		bgJobs = append(bgJobs, cmd)

		testing.Poll(ctx, func(ctx context.Context) error {
			killEvent, err = findLowMemoryKill(chromeLog)
			if killEvent != "" {
				return nil
			}
			if err != nil {
				return err
			}
			return errors.New("Could not find memory kill in %v")
		}, &testing.PollOptions{time.Duration(2 * time.Second), time.Duration(time.Second)})
		if killEvent != "" {
			s.Logf("Memory kill event: %q", killEvent)
			break
		}
	}
	for _, job := range bgJobs {
		job.Process.Kill()
	}
	if killEvent == "" {
		s.Fatal("Nothing was killed")
	}
	return
}

// Gets the newest pid of the given process name.
func getNewestPID(ctx context.Context, s *testing.State, name string) int {
	var out []byte
	var pid int64
	var err error
	cmd := testexec.CommandContext(ctx, "pgrep", "-f", "-n", name)
	out, err = cmd.Output()
	if err != nil {
		cmd.DumpLog(ctx)
		s.Fatalf("Failed to retrieve pid of %v: %v", name, err)
		return 0
	}
	pid, err = strconv.ParseInt(strings.Trim(string(out), "\n"), 10, 32)
	if err != nil {
		s.Fatalf("Failed to parse pgrep output for %v: %v", name, err)
		return 0
	}
	return int(pid)
}

// Reads OOM score of process pid.
func readOOMScoreAdj(pid int) (int, error) {
	var data []byte
	var score int64
	var err error
	data, err = ioutil.ReadFile(fmt.Sprintf("/proc/%d/oom_score_adj", pid))
	if err != nil {
		return 0, errors.Wrapf(err, "readOOMScoreAdj failed on pid %v", pid)
	}
	score, err = strconv.ParseInt(strings.Trim(string(data), "\n"), 10, 32)
	if err != nil {
		return 0, errors.Wrapf(err, "readOOMScoreAdj failed on pid %v", pid)
	}
	return int(score), nil
}

// Checks if pid has its oom_score_adj set.
// The default score is -1000 is nobody has changed its value.
func checkOOMScoreSet(pid int) (bool, error) {
	var score int
	var err error
	score, err = readOOMScoreAdj(pid)
	if err != nil {
		return false, errors.Wrapf(err, "Unable to read oom score for %v", pid)
	}
	return score != nonKillableOOMScore, nil
}

func findLowMemoryKill(log *os.File) (match string, ero error) {
	scanner := bufio.NewScanner(log)
	for scanner.Scan() {
		match = lowMemoryKillPattern.FindString(scanner.Text())
		if match != "" {
			break
		}
	}
	ero = scanner.Err()
	return
}

func estimatedFreeMemoryMB() (int, error) {
	const freeMemorySysFile = "/sys/kernel/mm/chromeos-low_mem/available"
	var data []byte
	var available int64
	var err error
	data, err = ioutil.ReadFile(freeMemorySysFile)
	if err != nil {
		return 0, errors.Wrapf(err, "Unable to read available memory at %v", freeMemorySysFile)
	}
	available, err = strconv.ParseInt(strings.Trim(string(data), "\n"), 10, 32)
	if err != nil {
		return 0, errors.Wrapf(err, "Unable to convert %v to integer", string(data))
	}
	return int(available), nil
}
