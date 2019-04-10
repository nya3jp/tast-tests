// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package security

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Minijail,
		Desc: "Verifies minijail0's basic functionality",
		Contacts: []string{
			"jorgelo@chromium.org", // Security team
			"derat@chromium.org",   // Tast port author
			"chromeos-security@google.com",
		},
	})
}

func Minijail(ctx context.Context, s *testing.State) {
	const (
		minijailPath = "/sbin/minijail0"
		bashPath     = "/bin/bash"
		// This is installed by the chromeos-base/tast-local-helpers-cros package.
		staticBashPath = "/usr/local/libexec/tast/helpers/local/cros/security.Minijail.staticbashexec"
	)
	if _, err := os.Stat(staticBashPath); err != nil {
		s.Fatalf("Failed to stat %v: %v", staticBashPath, err)
	}

	// Create a directory that can be written to by test cases running in user namespaces.
	usernsDir, err := ioutil.TempDir("", "tast.security.Minijail.userns.")
	if err != nil {
		s.Fatal("Failed to create userns dir: ", err)
	}
	defer os.RemoveAll(usernsDir)
	if err := os.Chmod(usernsDir, 0777); err != nil {
		s.Fatal("Failed to chmod userns dir: ", err)
	}

	type setupFunc func(tempDir string) error
	type checkFunc func(stdout string) error

	// testCase describes a minijail0 invocation.
	type testCase struct {
		name   string    // human-readable test case name
		cmd    string    // shell-quoted command and arguments to run via "bash -c"
		args   []string  // minijail0-specific args; "%T" is replaced by temp dir
		args64 []string  // like args, but only added if /lib64 exists
		setup  setupFunc // optional function to run before test
		check  checkFunc // optional function to run after test
	}

	runTestCase := func(tc *testCase, static bool) {
		// Construct a human-readable test name.
		name := tc.name
		if static {
			name += ".static"
		}

		// Create a temp dir that the test's setup function (if any) can write to.
		td, err := ioutil.TempDir("", "tast.security.Minijail."+name+".")
		if err != nil {
			s.Fatal("Failed to create temp dir: ", err)
		}
		defer os.RemoveAll(td)

		if tc.setup != nil {
			if err := tc.setup(td); err != nil {
				s.Errorf("Failed %v setup: %v", name, err)
				return
			}
		}

		// We need to make a copy of the args slice before modifying it, as each test case is
		// used twice with a static and non-static shell executable.
		args := append([]string{}, tc.args...)
		if _, err := os.Stat("/lib64"); err == nil {
			args = append(args, tc.args64...)
		}
		for i, a := range args {
			args[i] = strings.Replace(a, "%T", td, -1)
		}

		shell := bashPath
		if static {
			shell = staticBashPath
		}

		s.Log("Running test case ", name)
		args = append(args, shell, "-c", tc.cmd)
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		cmd := testexec.CommandContext(ctx, minijailPath, args...)
		out, err := cmd.Output()
		if err != nil {
			s.Errorf("%v failed: %v", name, err)
			cmd.DumpLog(ctx)
			return
		}

		if tc.check != nil {
			if err := tc.check(string(out)); err != nil {
				s.Errorf("%v command %q produced bad output: %v", name, tc.cmd, err)
			}
		}
	}

	// subdirSetup returns a setupFunc that creates the supplied subdirectories with the temp dir.
	subdirSetup := func(dirs ...string) setupFunc {
		return func(td string) error {
			for _, d := range dirs {
				if err := os.MkdirAll(filepath.Join(td, d), 0755); err != nil {
					return err
				}
			}
			return nil
		}
	}

	// mountTestCase returns a testCase based on common settings needed by chroot and pivotroot test cases.
	mountTestCase := func(name, cmd string, extraArgs []string) testCase {
		return testCase{
			name:  name,
			cmd:   cmd,
			setup: subdirSetup("c/bin", "c/lib64", "c/lib", "c/usr/lib", "c/usr/local", "c/tmp-rw", "c/tmp-ro", "tmp"),
			args: append([]string{
				"-b", "/bin,/bin",
				"-b", "/lib,/lib",
				"-b", "/usr/lib,/usr/lib",
				"-b", "/usr/local,/usr/local",
				"-b", "%T/tmp,/tmp-rw,1",
				"-b", "%T/tmp,/tmp-ro",
				"-v",
			}, extraArgs...),
			args64: []string{"-b", "/lib64,/lib64"},
		}
	}

	// checkRegexp returns a checkFunc that verifies that re matches stdout.
	checkRegexp := func(re string) checkFunc {
		r := regexp.MustCompile(re)
		return func(stdout string) error {
			if !r.MatchString(stdout) {
				return errors.Errorf("stdout %q not matched by %q", stdout, re)
			}
			return nil
		}
	}

	// checkFilePerms returns a checkFunc that verifies that p is owned by expUID and expGID.
	checkFilePerms := func(p string, expUID, expGID uint32) checkFunc {
		return func(stdout string) error {
			if fi, err := os.Stat(p); err != nil {
				return err
			} else if st := fi.Sys().(*syscall.Stat_t); st.Uid != expUID || st.Gid != expGID {
				return errors.Errorf("%v has UID %v (want %v) and GID %v (want %v)",
					p, st.Uid, expUID, st.Gid, expGID)
			}
			return nil
		}
	}

	chrootArgs := []string{"-C", "%T/c"}
	pivotrootArgs := []string{"-P", "%T/c"}
	usernsArgs := []string{"-m0 1000 1", "-M0 1000 1"}

	for _, tc := range []testCase{
		{
			name:  "caps",
			cmd:   `[ -w /usr/local/bin ] && cat /proc/self/status`,             // check that we kept CAP_DAC_OVERRIDE
			args:  []string{"-u", "1000", "-g", "1000", "-c", "2", "--ambient"}, // 2 is CAP_DAC_OVERRIDE
			check: checkRegexp(`(?m)^CapEff:\s*0000000000000002$`),
		},
		mountTestCase("chroot-cwd-is-root", "[ $(pwd) = / ]", chrootArgs),
		mountTestCase("chroot-lib-exists", "[ -d /lib ]", chrootArgs),
		mountTestCase("chroot-tmp-rw-exists", "[ -d /tmp-rw ]", chrootArgs),
		mountTestCase("chroot-tmp-ro-exists", "[ -d /tmp-ro ]", chrootArgs),
		mountTestCase("chroot-tmp-rw-is-writable", "echo x > /tmp-rw/test-rw", chrootArgs),
		mountTestCase("chroot-tmp-ro-is-read-only", "! echo x > /tmp-ro/test-ro", chrootArgs),
		{
			name:  "create-mount-destination",
			cmd:   "cat /proc/mounts",
			args:  []string{"-v", "-C", "/", "-k", "tmpfs,%T,tmpfs", "-b", "/dev/null,%T/test_null"},
			check: checkRegexp("test_null"),
		},
		{
			name:  "gid",
			cmd:   "id -rg && id -g",
			args:  []string{"-g", "1000"},
			check: checkRegexp("^1000\n1000\n$"),
		},
		{
			name:  "group",
			cmd:   "id -rg && id -g",
			args:  []string{"-g", "chronos"},
			check: checkRegexp("^1000\n1000\n$"),
		},
		{
			name:  "init",
			cmd:   "echo $$",
			args:  []string{"-I"},
			check: checkRegexp("^1\n$"),
		},
		{
			name: "mountns-enter",
			// We run a long one-liner within a new mount namespace:
			cmd: strings.Join([]string{
				// Create a temp dir and mount it as tmpfs.
				`dir=$(mktemp -d)`,
				`f="${dir}/test"`,
				`mount tmpfs "${dir}" -t tmpfs`,
				// Write a file within the dir.
				`echo inaccessible >"$f"`,
				// Try to cat the file while running within init's mountns.
				`(/sbin/minijail0 -V /proc/1/ns/mnt -- /bin/cat "${f}" || true) 2>&1`,
				// Clean up.
				`umount "${dir}"`,
				`rm -r "${dir}"`,
			}, " && "),
			args: []string{"-v"},
			// The cat process should be unable to access the file that we wrote.
			check: checkRegexp(`cat: \S+: No such file or directory`),
		},
		{
			name:  "mount-tmpfs",
			cmd:   "cat /proc/mounts",
			args:  []string{"-v", "-C", "/", "-k", "tmpfs,%T,tmpfs,0x1,uid=5446"},
			check: checkRegexp("tmpfs.*ro.*uid=5446"),
		},
		{
			name:  "netns",
			cmd:   `wc -l </proc/net/dev`, // look in /proc/net/dev so we get even downed devices
			args:  []string{"-e"},
			check: checkRegexp("^3\n$"),
		},
		{
			name: "pid-file",
			// The PID file is written by the parent process after forking,
			// so it may not be there initially: https://crbug.com/949357
			// It may also be empty at first: https://crbug.com/950504
			cmd: `while ! read pid < pidfile || [ "${pid}" != $$ ]; do sleep 0.1; done`,
			args: []string{
				"-b", "/bin,/bin",
				"-b", "/lib,/lib",
				"-b", "/usr/bin,/usr/bin", // for /usr/bin/coreutils, needed by sleep
				"-b", "/usr/lib,/usr/lib",
				"-b", "/usr/local,/usr/local",
				"-C", "%T/c",
				"-f", "%T/c/pidfile",
			},
			args64: []string{"-b", "/lib64,/lib64"},
			setup:  subdirSetup("c/bin", "c/lib64", "c/lib", "c/usr/bin", "c/usr/lib", "c/usr/local"),
		},
		{
			name:  "pidns",
			cmd:   "echo $$",
			args:  []string{"-p"},
			check: checkRegexp("^2\n$"),
		},
		mountTestCase("pivotroot-cwd-is-root", "[ $(pwd) = / ]", pivotrootArgs),
		mountTestCase("pivotroot-lib-exists", "[ -d /lib ]", pivotrootArgs),
		mountTestCase("pivotroot-tmp-rw-exists", "[ -d /tmp-rw ]", pivotrootArgs),
		mountTestCase("pivotroot-tmp-ro-exists", "[ -d /tmp-ro ]", pivotrootArgs),
		mountTestCase("pivotroot-tmp-rw-is-writable", "echo x > /tmp-rw/test-rw", pivotrootArgs),
		mountTestCase("pivotroot-tmp-ro-is-read-only", "! echo x > /tmp-ro/test-ro", pivotrootArgs),
		{
			name: "remount",
			cmd:  "[ ! -w /proc/sys/kernel/printk ]",
			args: []string{"-r"},
		},
		{
			name:  "rlimits",
			cmd:   "cat /proc/self/limits",
			args:  []string{"-R", "13,10,11"}, // 13 is RLIMIT_NICE
			check: checkRegexp(`Max nice priority\s*10\s*11`),
		},
		{
			name:  "tmpfs",
			cmd:   "stat -f /tmp -c %T", // the %T here is a format string to print the FS type
			setup: subdirSetup("c/bin", "c/lib64", "c/lib", "c/usr/lib", "c/usr/local", "c/usr/bin", "c/tmp"),
			args: []string{
				"-b", "/bin,/bin",
				"-b", "/lib,/lib",
				"-b", "/usr/lib,/usr/lib",
				"-b", "/usr/bin,/usr/bin",
				"-b", "/usr/local,/usr/local",
				"-C", "%T/c",
				"-t",
				"-v",
			},
			args64: []string{"-b", "/lib64,/lib64"},
			check:  checkRegexp("^tmpfs\n$"),
		},
		{
			name:  "uid",
			cmd:   "id -ru && id -u",
			args:  []string{"-u", "1000"},
			check: checkRegexp("^1000\n1000\n$"),
		},
		{
			name:  "user",
			cmd:   "id -ru && id -u",
			args:  []string{"-u", "chronos"},
			check: checkRegexp("^1000\n1000\n$"),
		},
		{
			name:  "usergroups-add-new",
			cmd:   "groups",
			args:  []string{"-u", "chronos", "-g", "chronos", "-G"},
			check: checkRegexp(`\baudio\b`),
		},
		{
			name: "usergroups-remove-orig",
			cmd:  "groups",
			args: []string{"-u", "chronos", "-g", "chronos", "-G"},
			check: func(stdout string) error {
				if strings.Contains(stdout, "root") {
					return errors.New("still in group 'root'")
				}
				return nil
			},
		},
		{
			name:  "userns-file",
			cmd:   fmt.Sprintf("[ $(id -u) = 65534 ] && [ $(id -g) = 65534 ] && touch %q", filepath.Join(usernsDir, "userns-file")),
			args:  []string{"-U"},
			check: checkFilePerms(filepath.Join(usernsDir, "userns-file"), 0, 0),
		},
		{
			name:  "userns-file-gid",
			cmd:   fmt.Sprintf("[ $(id -u) = 65534 ] && [ $(id -g) = 0 ] && touch %q", filepath.Join(usernsDir, "userns-file-gid")),
			args:  []string{"-M0 1000 1"},
			check: checkFilePerms(filepath.Join(usernsDir, "userns-file-gid"), 0, 1000),
		},
		{
			name:  "userns-gid",
			cmd:   "id -rg && id -g",
			args:  usernsArgs,
			check: checkRegexp("^0\n0\n$"),
		},
		{
			name:  "userns-init",
			cmd:   "echo $$",
			args:  append(usernsArgs, "-I"),
			check: checkRegexp("^1\n$"),
		},
		{
			name:  "userns-netns",
			cmd:   `wc -l </proc/net/dev`, // look in /proc/net/dev so we get even downed devices
			args:  append(usernsArgs, "-e"),
			check: checkRegexp("^3\n$"),
		},
		{
			name:  "userns-pidns",
			cmd:   "echo $$",
			args:  append(usernsArgs, "-p"),
			check: checkRegexp("^2\n$"),
		},
		{
			name:  "userns-uid",
			cmd:   "id -ru && id -u",
			args:  usernsArgs,
			check: checkRegexp("^0\n0\n$"),
		},
	} {
		runTestCase(&tc, false) // non-static
		runTestCase(&tc, true)  // static
	}
}
