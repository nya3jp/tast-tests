// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package syzkaller

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	kcovPath             = "/sys/kernel/debug/kcov"
	kasanInitMsg         = "KernelAddressSanitizer initialized"
	syzkallerRunDuration = 10 * time.Minute
	tastResultsDir       = "/tmp/tast/results/latest/tests/syzkaller.Wrapper/"
)

const startupScriptContents string = `
mount -o remount,rw -o exec /tmp
sysctl -w kernel.panic_on_warn=1
dmesg --clear
`

type vm struct {
	Targets       []string `json:"targets"`
	TargetDir     string   `json:"target_dir"`
	TargetReboot  bool     `json:"target_reboot"`
	StartupScript string   `json:"startup_script"`
}

type syzkallerConfig struct {
	Name      string `json:"name"`
	Target    string `json:"target"`
	Reproduce bool   `json:"reproduce"`
	HTTP      string `json:"http"`
	Workdir   string `json:"workdir"`
	Syzkaller string `json:"syzkaller"`
	Type      string `json:"type"`
	SSHKey    string `json:"sshkey"`
	Procs     int    `json:"procs"`
	VM        vm     `json:"vm"`
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
		Timeout:      12 * time.Minute,
		Attr:         []string{"group:syzkaller"},
		HardwareDeps: hwdep.D(hwdep.Model("octopus")),
	})
}

// Wrapper runs Syzkaller against DUTs with KASAN and KCOV enabled.
func Wrapper(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// If the kernel is not 4.4 or newer, syzkaller will not
	// fuzz the device as KCOV and KASAN have not been backported
	// that far back into Chrome OS kernels.
	rcmd := d.Conn().Command("uname", "-r")
	kernelVer, err := rcmd.Output(ctx)
	if err != nil {
		s.Error("unable to find DUT kernel version info")
		return
	}
	majorVer, err := strconv.Atoi(strings.Split(string(kernelVer), ".")[0])
	if err != nil {
		s.Error("unable to parse kernel version info")
		return
	}
	if majorVer < 4 {
		s.Log("kernel older than 4.4, skipping test")
		return
	}

	// If the kernel on the DUT is not built with KCOV support,
	// syzkaller cannot fuzz the device.
	rcmd = d.Conn().Command("test", "-f", kcovPath)
	if err := rcmd.Run(ctx); err != nil {
		s.Log("KCOV does not exist, skipping test")
		return
	}

	artifactsDir, err := ioutil.TempDir("", "tast-syzkaller-artifacts")
	if err != nil {
		s.Errorf("unable to create temp artifacts dir: %v", err)
		return
	}
	defer os.RemoveAll(artifactsDir)

	// Fetch syz-* binaries. Run syzkaller without vmlinux.
	if err = fetchFuzzArtifacts(ctx, d, artifactsDir); err != nil {
		s.Errorf("encountered error fetching fuzz artifacts: %v", err)
		time.Sleep(time.Minute)
		return
	}

	// Create a syzkaller working directory.
	syzkallerWorkdir, err := ioutil.TempDir("", "tast-syzkaller-workdir")
	if err != nil {
		s.Errorf("unable to create temp workdir: %v", err)
		return
	}
	defer os.RemoveAll(syzkallerWorkdir)

	// Create startup script.
	startupScript, err := ioutil.TempFile("", "tast-syzkaller-startup-script")
	if err != nil {
		s.Errorf("unable t ocreate temp configfile: %v", err)
		return
	}
	defer os.Remove(startupScript.Name())
	startupScript.Write([]byte(startupScriptContents))
	startupScript.Sync()

	// Create syzkaller configuration file.
	// Generating reproducers is unlikely to work as :
	// [1] Corpus is not shared across two runs of the test.
	// [2] A test is run for a short duration(10 minutes).
	// Hence, set Reproduce:false.
	config := syzkallerConfig{
		Name:      "syzkaller_tast",
		Target:    "linux/amd64",
		Reproduce: false,
		HTTP:      "localhost:56700",
		Workdir:   syzkallerWorkdir,
		Syzkaller: artifactsDir,
		Type:      "isolated",
		SSHKey:    d.KeyFile(),
		Procs:     10,
		VM: vm{
			Targets:       []string{d.HostName()},
			TargetDir:     "/tmp",
			TargetReboot:  false,
			StartupScript: startupScript.Name(),
		},
	}
	configFile, err := ioutil.TempFile("", "tast-syzkaller-config")
	if err != nil {
		s.Errorf("unable to create temp configfile: %v", err)
		return
	}
	defer os.Remove(configFile.Name())

	jsonConfig, err := json.Marshal(config)
	if err != nil {
		s.Errorf("invalid syzkaller configuration: %v", err)
		return
	}
	configFile.Write(jsonConfig)
	configFile.Sync()

	logFile, err := ioutil.TempFile("", "tast-syzkaller-logfile")
	if err != nil {
		s.Errorf("unable to create temp logfile: %v", err)
		return
	}
	defer os.Remove(logFile.Name())

	s.Logf("Starting syzkaller with logfile at %v", logFile.Name())
	syzManager := path.Join(artifactsDir, "syz-manager")
	managerCmd := exec.Command(syzManager, "-config", configFile.Name(), "-vv", "10")
	managerCmd.Stdout = logFile
	managerCmd.Stderr = logFile

	// Gracefully shut down syzkaller.
	go func() {
		time.AfterFunc(syzkallerRunDuration, func() {
			managerCmd.Process.Signal(os.Interrupt)
		})
	}()

	if err := managerCmd.Run(); err != nil {
		time.Sleep(time.Minute)
		s.Errorf("running syz-manager failed: %v", err)
	}

	managerCmd.Wait()

	// Copy the syzkaller stdout/stderr logfile and the working directory
	// as part of the tast results directory.
	s.Log("Copying syzkaller workdir to tast results directory")
	cmd := exec.Command("cp", "-r", syzkallerWorkdir, tastResultsDir)
	if err := cmd.Run(); err != nil {
		s.Errorf("failed to copy syzkaller workdir: %v", err)
		return
	}
	s.Log("Copying syzkaller logfile to tast results directory")
	cmd = exec.Command("cp", logFile.Name(), tastResultsDir)
	if err := cmd.Run(); err != nil {
		s.Errorf("failed to copy syzkaller logfile: %v", err)
		return
	}

	s.Log("Done fuzzing, exiting.")
}

func fetchFuzzArtifacts(ctx context.Context, d *dut.DUT, artifactsDir string) error {
	if err := os.MkdirAll(path.Join(artifactsDir, "/bin/linux_amd64"), 0755); err != nil {
		return err
	}

	// Get syz-manager, syz-fuzzer, syz-execprog and syz-executor from the DUT image.
	if err := linuxssh.GetFile(ctx, d.Conn(), "/usr/local/bin/syz-manager", path.Join(artifactsDir, "syz-manager")); err != nil {
		return err
	}

	// syz-manager expects (syz-executor,syz-fuzzer,syz-execprog) to be at <artifactsDir>/linux_<arch>/syz-*.
	artifacts := []string{"syz-fuzzer", "syz-executor", "syz-execprog"}
	for _, artifact := range artifacts {
		if err := linuxssh.GetFile(ctx, d.Conn(), path.Join("/usr/local/bin", artifact), path.Join(artifactsDir, "/bin/linux_amd64", artifact)); err != nil {
			return err
		}
	}

	return nil
}
