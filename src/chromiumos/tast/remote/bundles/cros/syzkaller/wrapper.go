// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package syzkaller

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware/reporters"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	// syzkallerRunDuration represents the overall run duration of the fuzzer.
	syzkallerRunDuration = 30 * time.Minute

	// gsURL points to the GCS bucket for syzkaller artifacts.
	gsURL = "gs://syzkaller-ctp-corpus"

	// syzUnknownEnabled is an error string to look out for in the fuzzer run logs.
	syzUnknownEnabled = "unknown enabled syscall"
)

// A global runtime variable to indicate the test is running locally.
var isLocal = testing.RegisterVarString(
	"syzkaller.isLocal",
	"false",
	"A variable to indicate the test is running locally.",
)

const startupScriptContents = `
sysctl -w kernel.panic_on_warn=1
dmesg --clear
`

var boardArchMapping = map[string]string{
	"octopus":  "amd64",
	"dedede":   "amd64",
	"nautilus": "amd64",
	"guybrush": "amd64",
	"brya":     "amd64",
	// syzkaller binaries built for trogdor and strongbad are 32 bit.
	"trogdor":   "arm",
	"strongbad": "arm",
	// syzkaller binaries built for Mediatek platforms are 32 bit.
	"kukui": "arm",
	// b/242131739: Jacuzzi and Cherry are for local testing at the moment.
	// The test is not enabled in the lab yet.
	"jacuzzi":   "arm",
	"cherry":    "arm",
	"herobrine": "aarch64",
}

// dutConfig represents information related to the DUT configuration;
// commands that need to be executed before each fuzzing
// iteration, directory from which to execute syz-executor, whether
// or not to perform a reboot after reading pstore contents.
type dutConfig struct {
	Targets       []string `json:"targets"`
	TargetDir     string   `json:"target_dir"`
	TargetReboot  bool     `json:"target_reboot"`
	StartupScript string   `json:"startup_script"`
	Pstore        bool     `json:"pstore"`
}

type syzkallerConfig struct {
	Name           string    `json:"name"`
	HubClient      string    `json:"hub_client"`
	HubAddr        string    `json:"hub_addr"`
	HubKey         string    `json:"hub_key"`
	Target         string    `json:"target"`
	Reproduce      bool      `json:"reproduce"`
	HTTP           string    `json:"http"`
	Workdir        string    `json:"workdir"`
	Syzkaller      string    `json:"syzkaller"`
	Type           string    `json:"type"`
	SSHKey         string    `json:"sshkey"`
	Procs          int       `json:"procs"`
	DUTConfig      dutConfig `json:"vm"`
	EnableSyscalls []string  `json:"enable_syscalls"`
}

type fuzzEnvConfig struct {
	// Driver or subsystem.
	Driver string `json:"driver"`
	// Boards specifies the boards to run a set of syscalls on. Boards can
	// be empty.
	Boards []string `json:"boards"`
	// ExcludeBoards specifies boards on which to not fuzz certain syscalls. ExcludeBoards
	// can be empty.
	ExcludeBoards []string `json:"exclude_boards"`
	// Startup commands specific to this subsystem.
	StartupCmds []string `json:"startup_cmds"`
	// Syscalls belonging to the driver or subsystem.
	Syscalls []string `json:"syscalls"`
}

type periodicConfig struct {
	// Board against which Cmd need to be run periodically.
	Board string `json:"board"`
	// Periodicity specifies in seconds how often Cmd should run against
	// the DUT.
	Periodicity int `json:"periodicity"`
	// Cmd to run against the DUT while fuzzing.
	Cmd string `json:"cmd"`
}

const (
	enabledSyscallsBrya    string = "enabled_syscalls_brya.json"
	enabledSyscallsNonBrya string = "enabled_syscalls.json"

	syzManagerHost string = "localhost"
	syzManagerPort int    = 56701
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Wrapper,
		Desc: "Wrapper test that runs Syzkaller",
		Contacts: []string{
			"zsm@chromium.org", // Test author
			"chromeos-kernel@google.com",
		},
		SoftwareDeps: []string{"pstore", "reboot"},
		// This wrapper runs syzkaller against the DUT for a duration of 30 minutes before
		// stopping. The overall test duration is 32 minutes.
		Timeout: syzkallerRunDuration + 2*time.Minute,
		Attr:    []string{"group:syzkaller"},
		Data:    []string{"testing_rsa", "periodic.json"},
		VarDeps: []string{"syzkaller.Wrapper.botoCredSection"},
		Params: []testing.Param{
			{
				// This testcase should only run on brya devices.
				Name:              "brya_cellular",
				Val:               enabledSyscallsBrya,
				ExtraData:         []string{enabledSyscallsBrya},
				ExtraHardwareDeps: hwdep.D(hwdep.Cellular(), hwdep.Model(bryaModels()...)),
			},
			{
				// This testcase should only run on non-brya devices.
				Name:              "non_brya",
				Val:               enabledSyscallsNonBrya,
				ExtraData:         []string{enabledSyscallsNonBrya},
				ExtraHardwareDeps: hwdep.D(hwdep.SkipOnModel(bryaModels()...)),
			},
		},
	})
}

func bryaModels() []string {
	return []string{
		"brya",
		"anahera",
		"banshee",
		"crota360",
		"felwinter",
		"gimble",
		"kano",
		"primus",
		"redrix",
		"taeko",
		"taeland",
		"taniks",
		"tarlo",
		"vell",
		"volmar",
		"zavala",
	}
}

// Wrapper runs Syzkaller against DUTs with KASAN and KCOV enabled.
func Wrapper(ctx context.Context, s *testing.State) {
	d := s.DUT()

	board, syzArch, err := findSyzkallerBoardAndArch(ctx, d)
	if err != nil {
		s.Fatal("Unable to find syzkaller arch: ", err)
	}
	s.Log("syzArch found to be: ", syzArch)

	kernelCommit, err := findKernelCommit(ctx, d)
	if err != nil {
		s.Fatal("Unable to find kernel commit: ", err)
	}
	s.Log("kernelCommit found to be: ", kernelCommit)

	syzkallerTastDir, err := ioutil.TempDir("", "tast-syzkaller")
	if err != nil {
		s.Fatal("Unable to create tast temporary directory: ", err)
	}
	defer os.RemoveAll(syzkallerTastDir)

	artifactsDir := filepath.Join(syzkallerTastDir, "artifacts")
	if err := os.Mkdir(artifactsDir, 0755); err != nil {
		s.Fatal("Unable to create temp artifacts dir: ", err)
	}

	// Fetch syz-* binaries. Run syzkaller without vmlinux.
	if err := fetchFuzzArtifacts(ctx, d, artifactsDir, syzArch); err != nil {
		s.Fatal("Encountered error fetching fuzz artifacts: ", err)
	}

	// Create a syzkaller working directory.
	syzkallerWorkdir := filepath.Join(syzkallerTastDir, "workdir")
	if err := os.Mkdir(syzkallerWorkdir, 0755); err != nil {
		s.Fatal("Unable to create temp workdir: ", err)
	}
	if isLocal.Value() != "true" {
		if err := loadCorpus(
			ctx,
			s.RequiredVar("syzkaller.Wrapper.botoCredSection"),
			board,
			syzkallerWorkdir,
		); err != nil {
			s.Fatal("Unable to load corpus: ", err)
		}
	}

	// Chmod the keyfile so that ssh connections do not fail due to
	// open permissions.
	cmd := exec.Command("cp", s.DataPath("testing_rsa"), syzkallerTastDir)
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to copy testing_rsa to tast temp dir: ", err)
	}
	sshKey := filepath.Join(syzkallerTastDir, "testing_rsa")
	if err := os.Chmod(sshKey, 0600); err != nil {
		s.Fatal("Unable to chmod sshkey to 0600: ", err)
	}

	param := s.Param().(string)
	s.Log("Loading enabled syscalls from: ", param)

	// Read enabled_syscalls.
	drivers, enabledSyscalls, scriptContents, err := loadEnabledSyscalls(s.DataPath(param), board)
	if err != nil {
		s.Fatal("Unable to load enabled syscalls: ", err)
	}
	s.Log("Drivers: ", drivers)
	s.Log("Enabled syscalls: ", enabledSyscalls)

	// Load periodic commands.
	pCmd, err := loadPeriodic(s.DataPath("periodic.json"), board)
	if err != nil {
		s.Fatal("Unable to load periodic cmds: ", err)
	}
	s.Log("Periodic cmds: ", pCmd)

	// Create startup script.
	startupScript := filepath.Join(syzkallerTastDir, "startup_script")
	if err := ioutil.WriteFile(startupScript, []byte(scriptContents), 0755); err != nil {
		s.Fatal("Unable to create temp configfile: ", err)
	}

	// Create syzkaller configuration file.
	// Generating reproducers is unlikely to work as :
	// [1] Corpus is not shared across two runs of the test.
	// [2] A test is run for a short duration(30 minutes).
	// Hence, set Reproduce:false.
	config := syzkallerConfig{
		Name:      board,
		Target:    fmt.Sprintf("linux/%v", syzArch),
		Reproduce: false,
		HTTP:      fmt.Sprintf("%v:%v", syzManagerHost, syzManagerPort),
		Workdir:   syzkallerWorkdir,
		Syzkaller: artifactsDir,
		Type:      "isolated",
		SSHKey:    sshKey,
		Procs:     5,
		DUTConfig: dutConfig{
			Targets:       []string{d.HostName()},
			TargetDir:     "/usr/local/tmp",
			TargetReboot:  true,
			StartupScript: startupScript,
			Pstore:        true,
		},
		EnableSyscalls: enabledSyscalls,
	}

	configFile, err := os.Create(filepath.Join(syzkallerTastDir, "config"))
	if err != nil {
		s.Fatal("Unable to create syzkaller configfile: ", err)
	}
	defer configFile.Close()

	if err := json.NewEncoder(configFile).Encode(config); err != nil {
		s.Fatal("Invalid syzkaller configuration: ", err)
	}

	logFile, err := os.Create(filepath.Join(syzkallerTastDir, "logfile"))
	if err != nil {
		s.Fatal("Unable to create temp logfile: ", err)
	}
	defer logFile.Close()

	// Ensure that system logs(related to tests that might have run earlier)
	// are flushed to disk.
	rcmd := d.Conn().CommandContext(ctx, "sync")
	if err := rcmd.Run(); err != nil {
		s.Fatal("Unable to flush cached content to disk: ", err)
	}

	s.Log("Starting syzkaller with logfile at ", logFile.Name())
	syzManager := filepath.Join(artifactsDir, "syz-manager")
	cmdArgs := []string{"-config", configFile.Name(), "-vv", "10"}
	managerCmd := testexec.CommandContext(ctx, syzManager, cmdArgs...)
	managerCmd.Stdout = logFile
	managerCmd.Stderr = logFile

	if err := managerCmd.Start(); err != nil {
		s.Fatal("Running syz-manager failed: ", err)
	}

	done := make(chan bool)
	if pCmd != nil {
		go runPeriodic(ctx, d, done, pCmd)
	}

	// Gracefully shut down syzkaller.
	func() {
		defer managerCmd.Wait()

		if err := testing.Sleep(ctx, syzkallerRunDuration); err != nil {
			managerCmd.Kill()
			s.Fatal("Failed to wait on syz-manager: ", err)
		}

		// Fetch coverage from syz-manager before stopping syz-manager.
		if isLocal.Value() != "true" {
			if err := saveCoverage(
				ctx,
				s.RequiredVar("syzkaller.Wrapper.botoCredSection"),
				s.OutDir(),
				board,
				kernelCommit,
			); err != nil {
				s.Fatal("Failed to upload coverage info: ", err)
			}
		}

		managerCmd.Process.Signal(os.Interrupt)
	}()

	if pCmd != nil {
		done <- true
	}

	logs, err := ioutil.ReadFile(logFile.Name())
	if err != nil {
		s.Fatalf("Unable to read logfile at [%v]: %v", logFile.Name(), err)
	}
	var unknown []string
	for _, line := range strings.Split(string(logs), "\n") {
		if strings.Contains(line, syzUnknownEnabled) {
			unknown = append(unknown, line)
		}
	}
	if len(unknown) != 0 {
		s.Fatal("Unsupported enabled syscall[s] found: ", unknown)
	}

	// Copy the syzkaller stdout/stderr logfile and the working directory
	// as part of the tast results directory.
	tastResultsDir := s.OutDir()
	s.Log("Copying syzkaller workdir to tast results directory")
	cmd = exec.Command("cp", "-r", syzkallerWorkdir, tastResultsDir)
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to copy syzkaller workdir: ", err)
	}
	s.Log("Copying syzkaller logfile to tast results directory")
	cmd = exec.Command("cp", logFile.Name(), tastResultsDir)
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to copy syzkaller logfile: ", err)
	}
	if isLocal.Value() != "true" {
		if err := saveCorpus(
			ctx,
			s.RequiredVar("syzkaller.Wrapper.botoCredSection"),
			board,
			filepath.Join(syzkallerWorkdir, "corpus.db"),
		); err != nil {
			s.Fatal("Failed to save corpus: ", err)
		}
	}

	s.Log("Done fuzzing, exiting")
}

func gsutilCmd(ctx context.Context, cred string, args ...string) *testexec.Cmd {
	gsutilArgs := append([]string{"-o", cred}, args...)
	cmd := testexec.CommandContext(ctx, "gsutil", gsutilArgs...)
	cmd.Env = append(os.Environ(), "BOTO_CONFIG= ")
	return cmd
}

// loadCorpus should only be used when running the test as scheduled in the lab.
func loadCorpus(ctx context.Context, cred, board, syzkallerWorkdir string) error {
	out, err := gsutilCmd(ctx, cred, "ls", gsURL).Output(testexec.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "failed to list corpus bucket")
	}
	objects := strings.Split(string(out), "\n")
	var url string
	for _, object := range objects {
		if strings.Contains(object, board) {
			url = object
		}
	}
	if url == "" {
		testing.ContextLog(ctx, "No pre-existing corpus found for board: ", board)
		return nil
	}
	testing.ContextLog(ctx, "Fetching ", url)
	// Note: No corpus is downloaded when running this test locally.
	if err = gsutilCmd(ctx, cred, "cp", url, filepath.Join(syzkallerWorkdir, "corpus.db")).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrapf(err, "failed to fetch: %v", url)
	}
	testing.ContextLog(ctx, "Fetched ", url)
	return nil
}

// saveCorpus should only be used when running the test as scheduled in the lab.
func saveCorpus(ctx context.Context, cred, board, corpusPath string) error {
	timestamp := time.Now().Format("2006-01-02-15:04:05")
	url := fmt.Sprintf("%s/corpus-%v-%v.db", gsURL, board, timestamp)
	testing.ContextLog(ctx, "Uploading ", url)
	// Note: No corpus is uploaded when running this test locally.
	if err := gsutilCmd(ctx, cred, "copy", corpusPath, url).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to save corpus.db")
	}
	testing.ContextLog(ctx, "Uploaded ", url)
	return nil
}

// saveCoverage should only be used when running the test as scheduled in the lab.
func saveCoverage(ctx context.Context, cred, outDir, board, kernelCommit string) error {
	timestamp := time.Now().Format("2006-01-02-15:04:05")
	coverName := fmt.Sprintf("rawcover-%v-%v-%v", board, timestamp, kernelCommit)
	coverFile := filepath.Join(outDir, coverName)

	testing.ContextLog(ctx, "Retrieving rawcoverage to ", coverFile)
	coverURL := fmt.Sprintf("http://%v:%v/rawcover32", syzManagerHost, syzManagerPort)
	if err := testexec.CommandContext(ctx, "wget", coverURL, "-O", coverFile).Run(); err != nil {
		return errors.Wrap(err, "unable to retrieve rawcover")
	}

	// Note: No coverage is uploaded when running this test locally.
	uploadURL := fmt.Sprintf("%s/rawcover32/%s", gsURL, coverName)
	testing.ContextLog(ctx, "Uploading to ", uploadURL)
	if err := gsutilCmd(ctx, cred, "copy", coverFile, uploadURL).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to save coverage file")
	}
	testing.ContextLog(ctx, "Uploaded to ", uploadURL)
	return nil
}

func findKernelCommit(ctx context.Context, d *dut.DUT) (string, error) {
	kernelRelease, err := d.Conn().CommandContext(ctx, "uname", "-r").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to find uname")
	}
	// Release for devices with a debug kernel should look something as follows.
	// "5.10.141-lockdep-19696-gb7597b887eec".
	parts := strings.Split(strings.TrimSpace(string(kernelRelease)), "-")
	if len(parts) < 2 {
		return "", errors.Errorf("unexpected release in uname [%v]", string(kernelRelease))
	}
	commit := parts[len(parts)-1]
	if !strings.HasPrefix(commit, "g") {
		return "", errors.Errorf("unexpected commit [%v] for uname [%v]", commit, kernelRelease)
	}
	return commit[1:], nil
}

func findSyzkallerBoardAndArch(ctx context.Context, d *dut.DUT) (board, arch string, err error) {
	board, err = reporters.New(d).Board(ctx)
	if err != nil {
		return "", "", errors.Wrap(err, "unable to find board")
	}
	if _, ok := boardArchMapping[board]; !ok {
		return "", "", errors.Wrapf(err, "unexpected board: %v", board)
	}
	return board, boardArchMapping[board], nil
}

func fetchFuzzArtifacts(ctx context.Context, d *dut.DUT, artifactsDir, syzArch string) error {
	binDir := fmt.Sprintf("bin/linux_%v", syzArch)
	if err := os.MkdirAll(filepath.Join(artifactsDir, binDir), 0755); err != nil {
		return err
	}

	// Get syz-manager, syz-fuzzer, syz-execprog and syz-executor from the DUT image.
	if err := linuxssh.GetFile(ctx, d.Conn(), "/usr/local/bin/syz-manager", filepath.Join(artifactsDir, "syz-manager"), linuxssh.PreserveSymlinks); err != nil {
		return err
	}

	// syz-manager expects (syz-executor,syz-fuzzer,syz-execprog) to be at <artifactsDir>/linux_<arch>/syz-*.
	artifacts := []string{"syz-fuzzer", "syz-executor", "syz-execprog"}
	for _, artifact := range artifacts {
		if err := linuxssh.GetFile(ctx, d.Conn(), filepath.Join("/usr/local/bin", artifact), filepath.Join(artifactsDir, binDir, artifact), linuxssh.PreserveSymlinks); err != nil {
			return err
		}
	}

	return nil
}

func loadEnabledSyscalls(fpath, board string) (drivers, enabledSyscalls []string, scriptContents string, err error) {
	contains := func(aList []string, item string) bool {
		for _, each := range aList {
			if each == item {
				return true
			}
		}
		return false
	}

	contents, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, nil, "", err
	}

	var feconfig []fuzzEnvConfig
	err = json.Unmarshal([]byte(contents), &feconfig)
	if err != nil {
		return nil, nil, "", err
	}

	scriptContents = startupScriptContents
	for _, config := range feconfig {
		if len(config.Boards) > 0 && len(config.ExcludeBoards) > 0 {
			return nil, nil, "", errors.Errorf("non-empty Boards and ExcludeBoards found for [%v]", config.Driver)
		}
		// Enable syscalls of a driver if |Boards| contains the DUT board.
		// Enable syscalls of a driver if |Boards| is empty, and the DUT board is not present in |ExcludeBoards|.
		ok := !contains(config.ExcludeBoards, board)
		if len(config.Boards) > 0 {
			ok = contains(config.Boards, board)
		}
		if ok {
			enabledSyscalls = append(enabledSyscalls, config.Syscalls...)
			drivers = append(drivers, config.Driver)
			scriptContents = scriptContents + strings.Join(config.StartupCmds, "\n") + "\n"
		}
	}

	return drivers, enabledSyscalls, scriptContents, nil
}

func loadPeriodic(fpath, board string) (*periodicConfig, error) {
	contents, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, err
	}
	var peconfig []*periodicConfig
	err = json.Unmarshal([]byte(contents), &peconfig)
	if err != nil {
		return nil, err
	}
	for _, config := range peconfig {
		if board == config.Board {
			return config, nil
		}
	}
	return nil, nil
}

func runPeriodic(ctx context.Context, d *dut.DUT, done chan bool, cfg *periodicConfig) {
	for {
		// Non-blocking check to see if we should stop running
		// Cmd periodically.
		select {
		case <-done:
			return
		default:
		}
		// Do not fail the test if the command fails to execute, only log.
		// Fuzzing can cause spurious device reboots.
		cmd := []string{"bash", "-c", cfg.Cmd}
		testing.ContextLog(ctx, "Going to run: ", cmd)
		if err := d.Conn().CommandContext(ctx, cmd[0], cmd[1:]...).Run(); err != nil {
			testing.ContextLogf(ctx, "Failed to run [%v]: %v", cmd, err)
		}
		// Poll is not used as device might reboot during fuzzing.
		testing.Sleep(ctx, time.Duration(cfg.Periodicity)*time.Second)
	}
}
