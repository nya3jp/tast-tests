// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

import (
	"testing"
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
	"chromiumos/tast/local/vm"
)

var gaiaLoginTests = []string{
	"no_access_to_drive.go",
	"share_drive.go",
}

var perfTests = map[string]time.Duration{
	"cpu_perf.go":      12 * time.Minute,
	"input_latency.go": 10 * time.Minute,
	"mouse_perf.go":    7 * time.Minute,
	"network_perf.go":  10 * time.Minute,
	"vim_compile.go":   12 * time.Minute,
}

var mainlineExpensiveTests = map[string]time.Duration{
	"fs_corruption.go":     7 * time.Minute,
	"pulse_audio_basic.go": 7 * time.Minute, // Not actually expensive, but broken on stretch.
}

var testFiles = []string{
	"audio_basic.go",
	"backup_restore.go",
	"basic.go",
	"command_cd.go",
	"command_ps.go",
	"command_vim.go",
	"copy_files_to_linux_files.go",
	"crash_reporter.go",
	"files_app_watch.go",
	"home_directory_create_file.go",
	"home_directory_delete_file.go",
	"home_directory_rename_file.go",
	"icon_and_username.go",
	"launch_browser.go",
	"launch_terminal.go",
	"no_access_to_downloads.go",
	"no_shared_folder.go",
	"open_with_terminal.go",
	"package_info.go",
	"package_install_uninstall.go",
	"remove_cancel.go",
	"remove_ok.go",
	"resize_cancel.go",
	"restart.go",
	"shared_font_files.go",
	"share_downloads_add_files.go",
	"share_downloads.go",
	"share_files_cancel.go",
	"share_files_manage.go",
	"share_files_ok.go",
	"share_files_restart.go",
	"share_files_toast.go",
	"share_folders.go",
	"share_invalid_paths.go",
	"sshfs_mount.go",
	"sync_time.go",
	"task_manager.go",
	"uninstall_invalid_app.go",
	"verify_app_wayland.go",
	"verify_app_x11.go",
	"vmc_extra_disk.go",
	"vmc_start.go",
	"webserver.go",
}

func TestParams(t *testing.T) {
	params := crostini.MakeTestParams(t)
	for _, filename := range testFiles {
		genparams.Ensure(t, filename, params)
	}
}

func TestExpensiveParams(t *testing.T) {
	for filename, duration := range perfTests {
		params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
			Timeout:       duration,
			MinimalSet:    true,
			IsNotMainline: true,
		}})
		genparams.Ensure(t, filename, params)
	}

	for filename, duration := range mainlineExpensiveTests {
		params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
			Timeout:    duration,
			MinimalSet: true,
		}})
		genparams.Ensure(t, filename, params)
	}
}

func TestGaiaLoginParams(t *testing.T) {
	for _, filename := range gaiaLoginTests {
		params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
			Preconditions: map[vm.ContainerDebianVersion]string{
				vm.DebianStretch: "crostini.StartedByArtifactWithGaiaLoginStretch()",
				vm.DebianBuster:  "crostini.StartedByArtifactWithGaiaLoginBuster()",
			}}})
		genparams.Ensure(t, filename, params)
	}
}
