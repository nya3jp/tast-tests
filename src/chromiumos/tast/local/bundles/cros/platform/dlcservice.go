// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DLCService,
		Desc:         "Verifies that DLC D-Bus API (install, uninstall, etc.) works",
		Contacts:     []string{"kimjae@chromium.org", "ahassani@chromium.org", "chromeos-core-services@google.com"},
		SoftwareDeps: []string{"dlc"},
		Attr:         []string{"group:mainline"},
	})
}

func DLCService(ctx context.Context, s *testing.State) {
	const (
		dlcID1          = "test1-dlc"
		dlcID2          = "test2-dlc"
		testPackage     = "test-package"
		dlcserviceJob   = "dlcservice"
		updateEngineJob = "update-engine"
		dlcCacheDir     = "/var/cache/dlc"
		tmpDir          = "/tmp"
	)

	type expect bool
	const (
		success expect = true
		failure expect = false
	)

	// TODO(kimjae): Manage this in separate .go file..
	type dlcListOutput struct {
		RootMount string `json:"root_mount"`
	}

	// Check dlcservice is up and running.
	if err := upstart.EnsureJobRunning(ctx, dlcserviceJob); err != nil {
		s.Fatalf("Failed to ensure %s running: %v", dlcserviceJob, err)
	}

	// Delete rollback-version and rollback-happened pref which are
	// generated during Rollback and Enterprise Rollback.
	// rollback-version is written when update_engine Rollback D-Bus API is
	// called. The existence of rollback-version prevents update_engine to
	// apply payload whose version is the same as rollback-version.
	// rollback-happened is written when update_engine finished Enterprise
	// Rollback operation.
	for _, p := range []string{"rollback-version", "rollback-happened"} {
		prefsPath := filepath.Join("/mnt/stateful_partition/unencrypted/preserve/update_engine/prefs", p)
		if err := os.RemoveAll(prefsPath); err != nil {
			s.Fatal("Failed to clean up pref: ", err)
		}
	}

	// Restart update-engine to pick up the new prefs.
	s.Logf("Restarting %s job", updateEngineJob)
	if err := upstart.RestartJob(ctx, updateEngineJob); err != nil {
		s.Fatalf("Failed to restart %s: %v", updateEngineJob, err)
	}

	// Wait for update-engine to be ready.
	if bus, err := dbusutil.SystemBus(); err != nil {
		s.Fatal("Failed to connect to the message bus: ", err)
	} else if err := dbusutil.WaitForService(ctx, bus, "org.chromium.UpdateEngine"); err != nil {
		s.Fatal("Failed to wait for D-Bus service: ", err)
	}

	readFile := func(path string) []byte {
		b, err := ioutil.ReadFile(path)
		if err != nil {
			s.Fatal("Failed to read file: ", err)
		}
		return b
	}

	dlcList := func(path string) (output map[string][]dlcListOutput) {
		if err := json.Unmarshal(readFile(path), &output); err != nil {
			s.Fatal("Failed to read json: ", err)
		}
		return output
	}

	verifyDlcContent := func(path, dlc string) {
		removeExt := func(path string) string {
			return strings.TrimSuffix(path, filepath.Ext(path))
		}
		checkSHA2Sum := func(hash_path string) {
			path := removeExt(hash_path)
			actualSumBytes := sha256.Sum256(readFile(path))
			actualSum := hex.EncodeToString(actualSumBytes[:])
			expectedSum := strings.Fields(string(readFile(hash_path)))[0]
			if actualSum != expectedSum {
				s.Fatalf("SHA2 checksum do not match for %s. Actual=%s Expected=%s",
					path, actualSum, expectedSum)
			}
		}
		checkPerms := func(perms_path string) {
			path := removeExt(perms_path)
			info, err := os.Stat(path)
			if err != nil {
				s.Fatal("Failed to stat: ", err)
			}
			actualPerm := fmt.Sprintf("%#o", info.Mode().Perm())
			expectedPerm := strings.TrimSpace(string(readFile(perms_path)))
			if actualPerm != expectedPerm {
				s.Fatalf("Permissions do not match for %s. Actual=%s Expected=%s",
					path, actualPerm, expectedPerm)
			}
		}
		getRootMounts := func(path, dlc string) (rootMounts []string) {
			if l, ok := dlcList(path)[dlc]; ok {
				for _, val := range l {
					rootMounts = append(rootMounts, val.RootMount)
				}
			}
			return rootMounts
		}

		rootMounts := getRootMounts(path, dlc)
		if len(rootMounts) == 0 {
			s.Fatal("Failed to get root mount for ", dlc)
		}
		for _, rootMount := range rootMounts {
			filepath.Walk(rootMount, func(path string, info os.FileInfo, err error) error {
				switch filepath.Ext(path) {
				case ".sum":
					checkSHA2Sum(path)
					break
				case ".perms":
					checkPerms(path)
					break
				}
				return nil
			})
		}
	}

	// dumpAndVerifyInstalledDLCs calls dlcservice's GetInstalled D-Bus method
	// via dlcservice_util command.
	dumpAndVerifyInstalledDLCs := func(tag string, dlcs ...string) {
		s.Log("Asking dlcservice for installed DLC modules")
		f := tag + ".log"
		path := filepath.Join(s.OutDir(), f)
		cmd := testexec.CommandContext(ctx, "dlcservice_util", "--list", "--dump="+path)
		if err := cmd.Run(testexec.DumpLogOnError); err != nil {
			s.Fatal("Failed to get installed DLC modules: ", err)
		}
		for _, dlc := range dlcs {
			verifyDlcContent(path, dlc)
		}
	}

	runCmd := func(msg string, e expect, name string, args ...string) {
		cmd := testexec.CommandContext(ctx, name, args...)
		if err := cmd.Run(testexec.DumpLogOnError); err != nil && e {
			s.Fatal("Failed to ", msg, err)
		} else if err == nil && !e {
			s.Fatal("Should have failed to ", msg)
		}
	}

	install := func(dlcs []string, omahaURL string, e expect) {
		s.Log("Installing DLC(s): ", dlcs, " to ", omahaURL)
		runCmd("install", e, "dlcservice_util", "--install",
			"--dlc_ids="+strings.Join(dlcs, ":"), "--omaha_url="+omahaURL)
	}

	uninstall := func(dlcs string, e expect) {
		s.Log("Uninstalling DLC(s): ", dlcs)
		runCmd("uninstall", e, "dlcservice_util", "--uninstall", "--dlc_ids="+dlcs)
	}

	startNebraska := func() (string, *testexec.Cmd) {
		s.Log("Starting Nebraska")
		cmd := testexec.CommandContext(ctx, "nebraska.py",
			"--runtime-root", "/tmp/nebraska",
			"--install-metadata", "/usr/local/dlc",
			"--install-payloads-address", "file:///usr/local/dlc")
		if err := cmd.Start(); err != nil {
			s.Fatal("Failed to start Nebraska: ", err)
		}

		success := false
		defer func() {
			if success {
				return
			}
			cmd.Kill()
			cmd.Wait()
		}()

		// Try a few times to make sure Nebraska is up.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			if _, err := os.Stat("/tmp/nebraska/port"); os.IsNotExist(err) {
				return err
			}
			return nil
		}, &testing.PollOptions{Timeout: time.Second * 5}); err != nil {
			s.Fatal("Nebraska did not start: ", err)
		}

		port, err := ioutil.ReadFile("/tmp/nebraska/port")
		if err != nil {
			s.Fatal("Failed to read the Nebraska's port file: ", err)
		}

		success = true
		return fmt.Sprintf("http://127.0.0.1:%s/update?critical_update=True", string(port)), cmd
	}

	stopNebraska := func(cmd *testexec.Cmd, name string) {
		s.Log("Stopping Nebraska")
		// Kill the Nebraska. with SIGINT so it has time to remove port/pid files
		// and cleanup properly.
		cmd.Signal(syscall.SIGINT)
		cmd.Wait()

		if !s.HasError() {
			return
		}

		// Read nebraska log and dump it out.
		if b, err := ioutil.ReadFile("/tmp/nebraska.log"); err != nil {
			s.Error("Nebraska log does not exist: ", err)
		} else if err := ioutil.WriteFile(filepath.Join(s.OutDir(), name+"-nebraska.log"), b, 0644); err != nil {
			s.Error("Failed to write nebraska log: ", err)
		}
	}

	defer func() {
		// Removes the installed DLC module and unmounts all test DLC images mounted under /run/imageloader.
		ids := []string{dlcID1, dlcID2}
		for _, id := range ids {
			path := filepath.Join("/run/imageloader", id, testPackage)
			if err := testexec.CommandContext(ctx, "imageloader", "--unmount", "--mount_point="+path).Run(testexec.DumpLogOnError); err != nil {
				s.Errorf("Failed to unmount DLC (%s): %v", id, err)
			}
			if err := os.RemoveAll(filepath.Join(dlcCacheDir, id)); err != nil {
				s.Error("Failed to clean up: ", err)
			}
		}
	}()

	s.Run(ctx, "Single DLC combination tests", func(context.Context, *testing.State) {
		url, cmd := startNebraska()
		defer stopNebraska(cmd, "single-dlc")

		// Before performing any Install/Uninstall.
		dumpAndVerifyInstalledDLCs("initial_state")

		// Install empty DLC.
		install([]string{}, url, failure)
		dumpAndVerifyInstalledDLCs("install_empty")

		// Install single DLC.
		install([]string{dlcID1}, url, success)
		dumpAndVerifyInstalledDLCs("install_single", dlcID1)

		// Install already installed DLC.
		install([]string{dlcID1}, url, success)
		dumpAndVerifyInstalledDLCs("install_already_installed", dlcID1)

		// Install duplicates of already installed DLC.
		install([]string{dlcID1, dlcID1}, url, success)
		dumpAndVerifyInstalledDLCs("install_already_installed_duplicate", dlcID1)

		// Uninstall single DLC.
		uninstall(dlcID1, success)
		dumpAndVerifyInstalledDLCs("uninstall_dlc")

		// Uninstall already uninstalled DLC.
		uninstall(dlcID1, success)
		dumpAndVerifyInstalledDLCs("uninstall_already_uninstalled")

		// Install duplicates of DLC atomically.
		install([]string{dlcID1, dlcID1}, url, success)
		dumpAndVerifyInstalledDLCs("atommically_install_duplicate", dlcID1)

		// Install unsupported DLC.
		install([]string{"unsupported-dlc"}, url, failure)
		dumpAndVerifyInstalledDLCs("install_unsupported")

	})

	s.Run(ctx, "Multi DLC combination tests", func(context.Context, *testing.State) {
		url, cmd := startNebraska()
		defer stopNebraska(cmd, "multi-dlc")

		// Install multiple DLC(s).
		install([]string{dlcID1, dlcID2}, url, success)
		dumpAndVerifyInstalledDLCs("install_multiple", dlcID1, dlcID2)

		// Install multiple DLC(s) already installed.
		install([]string{dlcID1, dlcID2}, url, success)
		dumpAndVerifyInstalledDLCs("install_multiple_already_installed", dlcID1, dlcID2)

		// Uninstall multiple installed DLC(s).
		uninstall(dlcID1, success)
		uninstall(dlcID2, success)
		dumpAndVerifyInstalledDLCs("uninstall_multiple_installed")
	})
}
