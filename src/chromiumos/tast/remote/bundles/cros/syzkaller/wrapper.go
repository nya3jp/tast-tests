// Copyright 2020 The Chromium OS Authors. All rights reserved.
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
)

const (
	syzkallerRunDuration = 30 * time.Minute
)

const startupScriptContents = `
sysctl -w kernel.panic_on_warn=1
dmesg --clear
`

var boardArchMapping = map[string]string{
	"octopus":  "amd64",
	"dedede":   "amd64",
	"nautilus": "amd64",
	// syzkaller binaries built for trogdor are 32 bit.
	"trogdor": "arm",
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
	// If `archs` is not specified, run on all archs.
	Archs []string `json:"archs"`
	// Startup commands specific to this subsystem.
	StartupCmds []string `json:"startup_cmds"`
	// Syscalls belonging to the driver or subsystem.
	Syscalls []string `json:"syscalls"`
}

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
		Data:    []string{"testing_rsa", "enabled_syscalls.json", "corpus.db"},
	})
}

// Wrapper runs Syzkaller against DUTs with KASAN and KCOV enabled.
func Wrapper(ctx context.Context, s *testing.State) {
	d := s.DUT()

	syzArch, err := findSyzkallerArch(ctx, d)
	if err != nil {
		s.Fatal("Unable to find syzkaller arch: ", err)
	}
	s.Log("syzArch found to be: ", syzArch)

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
	cmd := exec.Command("cp", s.DataPath("corpus.db"), syzkallerWorkdir)
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to copy seed corpus to workdir: ", err)
	}

	// Chmod the keyfile so that ssh connections do not fail due to
	// open permissions.
	cmd = exec.Command("cp", s.DataPath("testing_rsa"), syzkallerTastDir)
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to copy testing_rsa to tast temp dir: ", err)
	}
	sshKey := filepath.Join(syzkallerTastDir, "testing_rsa")
	if err := os.Chmod(sshKey, 0600); err != nil {
		s.Fatal("Unable to chmod sshkey to 0600: ", err)
	}

	// Read enabled_syscalls.
	drivers, enabledSyscalls, scriptContents, err := loadEnabledSyscalls(s.DataPath("enabled_syscalls.json"), syzArch)
	if err != nil {
		s.Fatal("Unable to load enabled syscalls: ", err)
	}
	s.Log("Drivers: ", drivers)
	s.Log("Enabled syscalls: ", enabledSyscalls)

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
		Name:      "syzkaller_tast",
		Target:    fmt.Sprintf("linux/%v", syzArch),
		Reproduce: false,
		HTTP:      "localhost:56700",
		Workdir:   syzkallerWorkdir,
		Syzkaller: artifactsDir,
		Type:      "isolated",
		SSHKey:    sshKey,
		Procs:     1,
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
	rcmd := d.Conn().Command("sync")
	if err := rcmd.Run(ctx); err != nil {
		s.Fatal("Unable to flush cached content to disk: ", err)
	}

	s.Log("Starting syzkaller with logfile at ", logFile.Name())
	syzManager := filepath.Join(artifactsDir, "syz-manager")
	managerCmd := testexec.CommandContext(ctx, syzManager, "-config", configFile.Name(), "-vv", "10")
	managerCmd.Stdout = logFile
	managerCmd.Stderr = logFile

	if err := managerCmd.Start(); err != nil {
		s.Fatal("Running syz-manager failed: ", err)
	}

	// Gracefully shut down syzkaller.
	func() {
		defer managerCmd.Wait()

		if err := testing.Sleep(ctx, syzkallerRunDuration); err != nil {
			managerCmd.Kill()
			s.Fatal("Failed to wait on syz-manager: ", err)
		}

		managerCmd.Process.Signal(os.Interrupt)
	}()

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

	s.Log("Done fuzzing, exiting")
}

func findSyzkallerArch(ctx context.Context, d *dut.DUT) (string, error) {
	board, err := reporters.New(d).Board(ctx)
	if err != nil {
		return "", errors.Wrap(err, "unable to find board")
	}
	if _, ok := boardArchMapping[board]; !ok {
		return "", errors.Wrapf(err, "unexpected board: %v", board)
	}
	return boardArchMapping[board], nil
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

func loadEnabledSyscalls(fpath, syzArch string) (drivers, enabledSyscalls []string, scriptContents string, err error) {
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
		if len(config.Archs) == 0 || contains(config.Archs, syzArch) {
			enabledSyscalls = append(enabledSyscalls, config.Syscalls...)
			drivers = append(drivers, config.Driver)
			scriptContents = scriptContents + strings.Join(config.StartupCmds, "\n") + "\n"
		}
	}

	return drivers, enabledSyscalls, scriptContents, nil
}
