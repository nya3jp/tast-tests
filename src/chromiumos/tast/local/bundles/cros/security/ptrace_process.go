// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: PtraceProcess,
		Desc: "Checks that the kernel restricts ptrace between processes",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
		Attr: []string{"informational"},
	})
}

func PtraceProcess(ctx context.Context, s *testing.State) {
	const (
		// TODO(derat): Consider moving the base helper path to a shared constant somewhere.
		sleeperPath = "/usr/local/libexec/tast/helpers/local/cros/security.PtraceProcess.sleeper"
		sleepTime   = 120 * time.Second

		unprivUser = "chronos"
		unprivUID  = sysutil.ChronosUID
		unprivGID  = sysutil.ChronosGID
	)

	const sysctl = "/proc/sys/kernel/yama/ptrace_scope"
	b, err := ioutil.ReadFile(sysctl)
	if err != nil {
		s.Fatalf("Failed to read %v: %v", sysctl, err)
	}
	if str := strings.TrimSpace(string(b)); str != "1" {
		s.Fatalf("%v contains %q; want \"1\"", sysctl, str)
	}

	// userCmd returns a testexec.Cmd for running the supplied executable as an unprivileged user.
	userCmd := func(exe string, args ...string) *testexec.Cmd {
		cmd := testexec.CommandContext(ctx, exe, args...)
		cmd.Cred(syscall.Credential{Uid: unprivUID, Gid: unprivGID})
		return cmd
	}

	s.Log("Testing ptrace direct child")
	cmd := userCmd("gdb", "-ex", "run", "-ex", "quit", "--batch", sleeperPath)
	if out, err := cmd.CombinedOutput(testexec.DumpLogOnError); err != nil {
		s.Error("Using gdb to start direct child failed: ", err)
	} else if !strings.Contains(string(out), "Quit anyway") {
		s.Error("ptrace direct child disallowed")
	}

	// attachGDB attempts to run a gdb process that attaches to pid.
	// shouldAllow describes whether ptrace is expected to be allowed or disallowed.
	attachGDB := func(pid int, shouldAllow bool) error {
		testing.ContextLog(ctx, "Attaching gdb to ", pid)
		cmd := userCmd("gdb", "-ex", "attach "+strconv.Itoa(pid), "-ex", "quit", "--batch")
		out, err := cmd.CombinedOutput(testexec.DumpLogOnError)
		if err != nil {
			return errors.Wrap(err, "attaching gdb failed")
		}

		// After attaching, gdb prints a message like this at exit:
		//
		// A debugging session is active.
		//
		//         Inferior 1 [process 26416] will be detached.
		//
		// Quit anyway? (y or n)
		allowed := strings.Contains(string(out), "A debugging session is active.")
		if !allowed && !strings.Contains(string(out), "ptrace: Operation not permitted") {
			fn := fmt.Sprintf("gdb-%d.txt", pid)
			ioutil.WriteFile(filepath.Join(s.OutDir(), fn), out, 0644)
			return errors.New("failed determining if ptrace was allowed; see " + fn)
		}
		if shouldAllow && !allowed {
			return errors.New("ptrace disallowed")
		}
		if !shouldAllow && allowed {
			return errors.New("ptrace allowed")
		}
		return nil
	}

	s.Log("Starting sleep process")
	sleepCmd := userCmd("sleep", strconv.Itoa(int(sleepTime.Seconds())))
	if err := sleepCmd.Start(); err != nil {
		s.Fatal("Failed to start sleep: ", err)
	}
	defer sleepCmd.Wait()
	defer sleepCmd.Kill()
	sleepPID := sleepCmd.Process.Pid

	s.Log("Testing ptrace cousin")
	if err := attachGDB(sleepPID, false); err != nil {
		s.Error("ptrace cousin: ", err)
	}
	s.Log("Testing cousin visibility in /proc")
	procPath := fmt.Sprintf("/proc/%d/exe", sleepPID)
	if err := userCmd("ls", "-la", procPath).Run(testexec.DumpLogOnError); err != nil {
		s.Error("Cousin not visible in /proc: ", err)
	} else {
		s.Log("Cousin visible in /proc (as expected)")
	}

	s.Log("Testing ptrace init")
	if err := attachGDB(1, false); err != nil {
		s.Error("ptrace init: ", err)
	}
	s.Log("Testing init visibility in /proc")
	if err := userCmd("ls", "-la", "/proc/1/exe").Run(); err != nil {
		s.Log("init not visible in /proc (as expected)")
	} else {
		s.Error("init visible in /proc")
	}

	// startSleeper starts the "sleeper" executable from the security_tests package as unprivUser.
	// The process calls prctl(PR_SET_PTRACER, tracerPID, ...).
	// If pidns is true, the process runs in a PID namespace; otherwise it is executed directly.
	// The returned command is started already; the caller must call its Kill and Wait methods.
	// It corresponds to the minijail0 process if pidns is true or the sleeper process otherwise.
	startSleeper := func(tracerPID int, pidns bool) (*testexec.Cmd, error) {
		args := []string{sleeperPath, strconv.Itoa(tracerPID), strconv.Itoa(int(sleepTime.Seconds()))}

		var cmd *testexec.Cmd
		if pidns {
			cmd = testexec.CommandContext(ctx, "minijail0", "-p", "--", "/bin/su", "-c",
				shutil.EscapeSlice(args), unprivUser)
		} else {
			cmd = userCmd(args[0], args[1:]...)
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create sleeper stdout pipe")
		}

		testing.ContextLog(ctx, "Starting sleeper")
		if err := cmd.Start(); err != nil {
			return nil, errors.Wrap(err, "failed to start sleeper")
		}

		// Wait for the process to write "ready\n" to stdout to indicate that it's ready.
		ch := make(chan error, 1)
		go func() {
			const msg = "ready\n"
			b := make([]byte, len(msg))
			if _, err := io.ReadFull(stdout, b); err != nil {
				ch <- err
			} else if string(b) != msg {
				ch <- errors.Errorf("sleeper wrote %q", b)
			} else {
				ch <- nil
			}
		}()

		select {
		case <-ctx.Done():
			err = ctx.Err()
		case err = <-ch:
		}
		if err != nil {
			cmd.Kill()
			cmd.Wait(testexec.DumpLogOnError)
			return nil, errors.Wrap(err, "failed waiting for sleeper to start")
		}
		return cmd, nil
	}

	// testSetPtracer starts the "sleeper" executable with the supplied tracerPID argument
	// and passes the process's PID and shouldAllow to attachGDB.
	testSetPtracer := func(tracerPID int, shouldAllow bool) error {
		sleeperCmd, err := startSleeper(tracerPID, false)
		if err != nil {
			return err
		}
		defer sleeperCmd.Wait()
		defer sleeperCmd.Kill()
		return attachGDB(sleeperCmd.Process.Pid, shouldAllow)
	}

	s.Log("Testing prctl(PR_SET_PTRACER, 0, ...)")
	if err := testSetPtracer(0, false); err != nil {
		s.Error("ptrace after prctl(PR_SET_PTRACER, 0, ...): ", err)
	}
	s.Log("Testing prctl(PR_SET_PTRACER, parent, ...)")
	if err := testSetPtracer(os.Getpid(), true); err != nil {
		s.Error("ptrace after prctl(PR_SET_PTRACER, parent, ...): ", err)
	}
	s.Log("Testing prctl(PR_SET_PTRACER, 1, ...)")
	if err := testSetPtracer(1, true); err != nil {
		s.Error("ptrace after prctl(PR_SET_PTRACER, 1, ...): ", err)
	}
	s.Log("Testing prctl(PR_SET_PTRACER, -1, ...)")
	if err := testSetPtracer(-1, true); err != nil {
		s.Error("ptrace after prctl(PR_SET_PTRACER, -1, ...): ", err)
	}

	// hasAncestor returns true if pid has the specified ancestor.
	hasAncestor := func(pid, ancestor int32) (bool, error) {
		for {
			proc, err := process.NewProcess(pid)
			if err != nil {
				return false, err
			}
			ppid, err := proc.Ppid()
			if err != nil {
				return false, err
			}
			if ppid == 0 {
				return false, nil
			}
			if ppid == ancestor {
				return true, nil
			}
			pid = ppid
		}
	}

	// testSetPtracerPidns is similar to testSetPtracer, but runs the sleeper executable in a PID namespace.
	testSetPtracerPidns := func(tracerPID int, shouldAllow bool) error {
		minijailCmd, err := startSleeper(tracerPID, true)
		if err != nil {
			return err
		}
		defer minijailCmd.Wait()
		defer minijailCmd.Kill()

		// Find the sleeper process, which will be nested under minijail0 and su.
		sleeperPID := -1
		procs, err := process.Processes()
		if err != nil {
			return errors.Wrap(err, "failed listing procesess")
		}
		for _, proc := range procs {
			if exe, err := proc.Exe(); err != nil || exe != sleeperPath {
				continue
			}
			if ok, err := hasAncestor(proc.Pid, int32(minijailCmd.Process.Pid)); err != nil || !ok {
				continue
			}
			sleeperPID = int(proc.Pid)
			break
		}
		if sleeperPID == -1 {
			return errors.Errorf("didn't find sleeper process under minijail0 process %d", minijailCmd.Process.Pid)
		}

		return attachGDB(sleeperPID, shouldAllow)
	}

	s.Log("Testing prctl(PR_SET_PTRACER, 0, ...) across pidns")
	if err := testSetPtracerPidns(0, false); err != nil {
		s.Error("ptrace after prctl(PR_SET_PTRACER, 0, ...) across pidns: ", err)
	}
	s.Log("Testing prctl(PR_SET_PTRACER, -1, ...) across pidns")
	if err := testSetPtracerPidns(-1, true); err != nil {
		s.Error("ptrace after prctl(PR_SET_PTRACER, -1, ...) across pidns: ", err)
	}
}
