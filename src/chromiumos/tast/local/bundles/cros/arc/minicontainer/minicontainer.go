// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package minicontainer implements the tests to verify ARC Mini container's
// conditions.
package minicontainer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/bundles/cros/arc/cpuset"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func testZygote(ctx context.Context, s *testing.State) {
	s.Log("Running testZygote")

	var roZygote string
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		out, err := arc.BootstrapCommand(ctx, "/system/bin/getprop", "ro.zygote").Output()
		if err != nil {
			return errors.Wrap(err, "getprop ro.zygote failed")
		}
		roZygote = strings.TrimSpace(string(out))
		if roZygote == "" {
			// Note: Even if Android boots, getprop may return
			// empty string till it is initialized.
			return errors.New("ro.zygote is empty")
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Mini container didn't start: ", err)
	}

	// Depends on the configuration, there may be 32 or 64 bit zygote with
	// a different name. We need regular expressions because zygote
	// changes its process name to 'main' after preloading Java classes.
	// The preloading may or may not happen depending on $BOARD and DUT's
	// state.
	zygoteName, ok := map[string]string{
		"zygote32":    "(app_process|main)",
		"zygote64":    "(app_process64|main)",
		"zygote32_64": "(app_process32|main)",
		"zygote64_32": "(app_process64|main)",
	}[roZygote]
	if !ok {
		s.Errorf("Unrecognized ro.zygote: %q", roZygote)
		return
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := testexec.CommandContext(ctx, "pgrep", zygoteName).Run(); err != nil {
			return errors.Wrap(err, "zygote is not yet running")
		}
		return nil
	}, nil); err != nil {
		s.Fatal("Zygote did't start: ", err)
	}
}

func testCoreServices(ctx context.Context, s *testing.State) {
	s.Log("Running testCoreServices")

	targets := []string{
		"healthd",
		"logd",
		"servicemanager",
		"surfaceflinger",
		"vold",
		"android.hardware.audio@2.0-service-bertha",
		"android.hardware.cas@1.0-service",
		"android.hardware.configstore@1.1-service",
		"android.hardware.graphics.allocator@2.0-service",
		"android.hidl.allocator@1.0-service",
		"audioserver",
		"hwservicemanager",
		"thermalserviced",
		"ueventd",
		"vndservicemanager",
	}

	for _, target := range targets {
		// Although testZygote waits for the mini container boot,
		// some service may not yet started at this moment.
		// So, poll that the processes start.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			return testexec.CommandContext(ctx, "pidof", "-s", target).Run()
		}, nil); err != nil {
			s.Errorf("Could not find %s: %v", target, err)
		}
	}
}

func testAndroidLogs(ctx context.Context, s *testing.State, sr *syslog.Reader) {
	s.Log("Running testAndroidLogs")

	// Obtain arc-kmsg-logger logs for the latest Mini container.
	var logs bytes.Buffer
	for {
		e, err := sr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.Error("Failed to read syslog: ", err)
			return
		}
		if e.Program != "arc-kmsg-logger" {
			continue
		}
		logs.WriteString(e.Line)
	}

	// The log size is about 8KB after starting the mini container.
	// If the size is less than 2KB, something must be broken.
	const minLogSize = 2048
	if size := logs.Len(); size < minLogSize {
		// Dump arc-kmsg-logger log to output directory for debugging on failure.
		if err := ioutil.WriteFile(filepath.Join(s.OutDir(), "arc-kmsg-logger.log"), logs.Bytes(), 0644); err != nil {
			s.Error("Failed to write arc-kmsg-logger.log: ", err)
		}
		s.Errorf("log size is too small: got %d; want >= %d", size, minLogSize)
	}
}

func testSELinuxLabels(ctx context.Context, s *testing.State) {
	s.Log("Running testSELinuxLabels")
	const (
		root           = "/opt/google/containers/android/rootfs/root"
		cacheDir       = root + "/cache"
		dataDir        = root + "/data"
		dalvikCacheDir = dataDir + "/dalvik-cache"
	)

	verify := func(path, expect string) {
		out, err := testexec.CommandContext(ctx, "getfattr", "--no-dereference", "--only-values", "--name", "security.selinux", path).Output(testexec.DumpLogOnError)
		if err != nil {
			s.Errorf("Failed to get selinux label for %s: %v", path, err)
			return
		}
		label := strings.TrimSuffix(strings.TrimSpace(string(out)), "\x00")
		if label != expect {
			s.Errorf("Unexpected selinux label for %q: got %q; want %s", path, label, expect)
		}
	}

	verify(cacheDir, "u:object_r:cache_file:s0")
	verify(dataDir, "u:object_r:system_data_file:s0")
	verify(dalvikCacheDir, "u:object_r:dalvikcache_data_file:s0")

	if err := filepath.Walk(dalvikCacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		verify(path, "u:object_r:dalvikcache_data_file:s0")
		return nil
	}); err != nil {
		s.Error("Failed to walk dalvik-cache dir: ", err)
	}
}

func testCgroupDirectory(ctx context.Context, s *testing.State) {
	s.Log("Running testCgroupDirectory")
	const path = "/sys/fs/cgroup/devices/session_manager_containers/android"
	if fi, err := os.Stat(path); err != nil {
		s.Errorf("cgroup directory %s for the container does not exist: %v", path, err)
	} else if !fi.IsDir() {
		s.Errorf("cgroup directory %s is not a directory", path)
	}
}

func testBootOATSymlinks(ctx context.Context, s *testing.State) {
	s.Log("Running testBootOATSymlinks")
	if err := arc.BootstrapCommand(ctx, "/system/bin/sh", "-c", "cat /data/dalvik-cache/*/*.oat > /dev/null").Run(testexec.DumpLogOnError); err != nil {
		s.Error("Could not read *.oat files: ", err)
	}
}

func testDevFiles(ctx context.Context, s *testing.State) {
	s.Log("Running testDevFiles")

	// /dev/.coldboot_done is a file to tell Android's init that the
	// firmware initialization is done. In our case, the file has to be
	// created before the mini container is started. Otherwise, init will
	// freeze for a while waiting for the file until it times out.
	const target = "/dev/.coldboot_done"
	if err := arc.BootstrapCommand(ctx, "/system/bin/stat", target).Run(testexec.DumpLogOnError); err != nil {
		s.Errorf("Could not stat %s: %v", target, err)
	}
}

func testCPUSet(ctx context.Context, s *testing.State) {
	s.Log("Running testCPUSet")

	// Verify that /dev/cpuset is properly set up.
	types := []string{"foreground", "background", "system-background", "top-app", "restricted"}

	for _, t := range types {
		path := fmt.Sprintf("/dev/cpuset/%s/effective_cpus", t)
		out, err := arc.BootstrapCommand(ctx, "/system/bin/cat", path).Output(testexec.DumpLogOnError)
		if err != nil {
			s.Errorf("Failed to read %s: %v", path, err)
			continue
		}
		val := strings.TrimSpace(string(out))
		cpusInUse, err := cpuset.Parse(ctx, val)
		if err != nil {
			s.Errorf("Failed to parse %s: %v", path, err)
			continue
		}
		if len(cpusInUse) != runtime.NumCPU() {
			s.Errorf("Unexpected CPU setting %q for %s: got %d CPUs, want %d CPUs", val, path,
				len(cpusInUse), runtime.NumCPU())
		}
	}
}

// setUp restarts the ui job to make sure that the login screen is shown and
// ARC Mini container is started. While the ui job is stopped, /var/log/messages
// is opened to remember the log position, which is returned as syslog.Reader.
// It can be later used to read syslog after restarting the ui job.
func setUp(ctx context.Context) (_ *syslog.Reader, retErr error) {
	if err := upstart.StopJob(ctx, "ui"); err != nil {
		return nil, err
	}
	sr, err := syslog.NewReader(ctx)
	if err != nil {
		// Note: leave "ui" job stopped. A following test should handle
		// such a situation properly, if necessary.
		return nil, err
	}
	defer func() {
		if retErr != nil {
			sr.Close()
		}
	}()
	if err := upstart.StartJob(ctx, "ui"); err != nil {
		return nil, err
	}
	return sr, nil
}

// RunTest exercises conditions the ARC mini container needs to satisfy.
func RunTest(ctx context.Context, s *testing.State) {
	sr, err := setUp(ctx)
	if err != nil {
		s.Fatal("Failed to set up test: ", err)
	}
	defer sr.Close()

	testZygote(ctx, s)
	// Note: testZygote will wait for the mini container boot, or fail
	// with s.Fatal() family.
	// So, the following subtests can assume that the mini container is
	// running somehow.
	// In case of boot failure, the following subtests won't run
	// intentionally.
	testCoreServices(ctx, s)
	testAndroidLogs(ctx, s, sr)
	testSELinuxLabels(ctx, s)
	testCgroupDirectory(ctx, s)
	testBootOATSymlinks(ctx, s)
	testDevFiles(ctx, s)
	testCPUSet(ctx, s)
}
