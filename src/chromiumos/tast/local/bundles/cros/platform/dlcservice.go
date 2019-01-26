// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package platform

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

const (
	dlcModuleID     = "test-dlc"
	nebraskaBin     = "/usr/local/bin/nebraska.py"
	nebraskaPort    = "2412"
	payloadName     = "dlc_test-dlc.payload"
	payloadNameFull = "dlc/" + payloadName
	dlcserviceJob   = "dlcservice"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DLCService,
		Desc:         "Verifies that dlcservice exits on idle and accepts D-Bus calls",
		Contacts:     []string{"xiaochu@chromium.org"},
		Attr:         []string{"informational"},
		SoftwareDeps: []string{"dlc"},
		Data:         []string{payloadNameFull},
	})
}

func DLCService(ctx context.Context, s *testing.State) {
	dumpInstalledDlcModules := func(ctx context.Context, s *testing.State, filename string) {
		s.Log("Asking dlcservice for installed DLC modules")
		cmd := testexec.CommandContext(ctx, "dlcservice_util", "--list")
		if out, err := cmd.Output(); err != nil {
			cmd.DumpLog(ctx)
			s.Fatal("Failed to get installed DLC modules: ", err)
		} else {
			// Logs the installed DLC modules info into a file in output dir.
			if err := ioutil.WriteFile(filepath.Join(s.OutDir(), filename), out, 0644); err != nil {
				s.Fatal("Failed to write output: ", err)
			}
		}
	}
	ensureNebraskaUp := func(ctx context.Context, s *testing.State) (*testexec.Cmd, error) {
		pollNebraska := func(ctx context.Context, s *testing.State) {
			// Polls nebraska until it is up and running or timeout.
			testing.Poll(ctx, func(ctx context.Context) error {
				cmd := testexec.CommandContext(ctx, "curl", "-H", "\"Accept: application/xml\"", "-H", "\"Content-Type: application/xml\"", "-X", "POST", "-d", "<request></request>", "http://127.0.0.1:2412")
				if _, err := cmd.CombinedOutput(); err != nil {
					return err
				}
				return nil
			}, &testing.PollOptions{Timeout: 5 * time.Second})
		}
		preparePayload := func(ctx context.Context, s *testing.State, payloadDir string) error {
			parseLsbRelease := func(ctx context.Context, s *testing.State) (string, string, error) {
				lsbReleaseFile, err := os.Open("/etc/lsb-release")
				if err != nil {
					s.Fatal("Failed to open lsb-release: ", err)
					return "", "", err
				}
				lsbReleaseContent, err := ioutil.ReadAll(lsbReleaseFile)
				if err != nil {
					s.Fatal("Failed to read lsb-release: ", err)
					return "", "", err
				}
				lsbReleaseContentSlice := strings.Split(string(lsbReleaseContent), "\n")
				appid := ""
				targetVersion := ""
				for _, val := range lsbReleaseContentSlice {
					if len(val) > 20 && val[:20] == "CHROMEOS_BOARD_APPID" {
						appid = val[21:] + "_" + dlcModuleID
					} else if len(val) > 24 && val[:24] == "CHROMEOS_RELEASE_VERSION" {
						targetVersion = val[25:]
					}
				}
				return appid, targetVersion, nil
			}
			s.Log("Prepare update payload")
			// Creates subdirectory for install payloads.
			payloadInstallDir := filepath.Join(payloadDir, "install")
			if err := os.Mkdir(payloadInstallDir, 0755); err != nil {
				s.Fatal("Failed to create dir: ", err)
				return err
			}
			// Copies DLC module payload to payload directory.
			if err := fsutil.CopyFile(s.DataPath(payloadNameFull), filepath.Join(payloadInstallDir, payloadName)); err != nil {
				s.Fatal("Failed to copy payload %v: %v", payloadNameFull, err)
				return err
			}
			// Dumps payload metadata to payload directory (parsed by nebraska server).
			appid, targetVersion, err := parseLsbRelease(ctx, s)
			if err != nil {
				s.Fatal("Failed to parse lsb-release: ", err)
				return err
			}
			payloadManifestContent := fmt.Sprintf("{\"appid\": \"%s\",\"name\": \"%s\",\"target_version\": \"%s\",\"is_delta\": false,\"source_version\": \"0.0.0\",\"size\": 639,\"metadata_signature\": \"\",\"metadata_size\": 1,\"sha256_hex\": \"9f4290e6204eb12042b582a94a968bd565b11ae91f6bec717f0118c532293f62\"}", appid, payloadName, targetVersion)
			if err := ioutil.WriteFile(filepath.Join(payloadDir, "dlc.json"), []byte(payloadManifestContent), 0755); err != nil {
				s.Fatal("Failed to write file: ", err)
				return err
			}
			return nil
		}
		// Creates directory for payloads.
		payloadDir, err := ioutil.TempDir("", "tast.platform.DLCService")
		if err != nil {
			s.Fatal("Failed to create temp directory: ", err)
			return nil, err
		}
		// Prepares update payload.
		if err := preparePayload(ctx, s, payloadDir); err != nil {
			s.Fatal("Failed to prepare payload: ", err)
			return nil, err
		}
		// Ensures Nebraska is up.
		s.Log("Start nebraska server")
		cmd := testexec.CommandContext(ctx, "python", nebraskaBin, "--install-payloads="+payloadDir, "--port="+nebraskaPort, "--payload-addr="+"file://"+payloadDir)
		if _, err := cmd.StdinPipe(); err != nil {
			s.Fatal("Failed to get StdinPipe: ", err)
			return nil, err
		}
		if err := cmd.Start(); err != nil {
			s.Fatal("Failed to run nebraska: ", err)
			return nil, err
		}
		pollNebraska(ctx, s)
		return cmd, nil
	}
	cleanUp := func(ctx context.Context, s *testing.State) error {
		// Removes the DLC module.
		cmd := testexec.CommandContext(ctx, "rm", "-rf", "/var/lib/dlc/"+dlcModuleID)
		if err := cmd.Run(); err != nil {
			s.Fatal("Failed to clean up: ", err)
			return err
		}
		cmd = testexec.CommandContext(ctx, "imageloader", "--unmount_all")
		if err := cmd.Run(); err != nil {
			s.Fatal("Failed to unmount all: ", err)
			return err
		}
		return nil
	}

	defer cleanUp(ctx, s)

	// Ensures nebraska server is up.
	if cmd, err := ensureNebraskaUp(ctx, s); err != nil {
		defer cmd.Wait()
		defer cmd.Kill()
	}

	// Restarts dlcservice.
	s.Logf("Restarting %s job", dlcserviceJob)
	if err := upstart.RestartJob(ctx, dlcserviceJob); err != nil {
		s.Fatalf("Failed to start %s: %v", dlcserviceJob, err)
	}
	// Checks dlcservice exits on idle.
	// dlcservice is a short-lived process.
	if err := upstart.WaitForJobStatus(ctx, dlcserviceJob, upstart.StopGoal, upstart.WaitingState, upstart.TolerateWrongGoal, time.Minute); err != nil {
		s.Fatalf("Job %s did not exit on idle: %v", dlcserviceJob, err)
	}

	// Calls dlcservice's GetInstalled D-Bus method, checks the return results.
	// dlcservice is activated on-demand via D-Bus method call.
	dumpInstalledDlcModules(ctx, s, "installed_dlc_modules_0.txt")

	// Installs a DLC module.
	s.Logf("Install a DLC: %s", dlcModuleID)
	cmd := testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util", "--install", "--dlc_ids="+dlcModuleID, "--omaha_url=http://127.0.0.1:2412")
	if _, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to install DLC modules: ", err)
	}

	dumpInstalledDlcModules(ctx, s, "installed_dlc_modules_1.txt")

	// Uninstalls a DLC module.
	s.Logf("Uninstall a DLC: %s", dlcModuleID)
	cmd = testexec.CommandContext(ctx, "sudo", "-u", "chronos", "dlcservice_util", "--uninstall", "--dlc_ids="+dlcModuleID)
	if _, err := cmd.Output(); err != nil {
		cmd.DumpLog(ctx)
		s.Fatal("Failed to uninstall DLC modules: ", err)
	}

	dumpInstalledDlcModules(ctx, s, "installed_dlc_modules_2.txt")

	// Checks dlcservice exits on idle.
	if err := upstart.WaitForJobStatus(ctx, dlcserviceJob, upstart.StopGoal, upstart.WaitingState, upstart.TolerateWrongGoal, time.Minute); err != nil {
		s.Fatalf("Job %s did not exit on idle: %v", dlcserviceJob, err)
	}
}
