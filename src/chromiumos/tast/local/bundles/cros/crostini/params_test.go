// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package crostini

// To update test parameters after modifying this file, run:
// TAST_GENERATE_UPDATE=1 ~/trunk/src/platform/tast/tools/go.sh test -count=1 chromiumos/tast/local/bundles/cros/crostini/

// See src/chromiumos/tast/local/crostini/params.go for more documentation

import (
	"testing"
	"time"

	"chromiumos/tast/common/genparams"
	"chromiumos/tast/local/crostini"
)

// Files that are not handled either here or in their own tests, along
// with why they currently can't be handled.
var unhandledFiles = []string{
	"disk_io_perf.go",       // Uses StartedTraceVM
	"no_access_to_drive.go", // Uses real gaia login
	"run_with_arc.go",       // Requires ARC++ enabled
	"share_drive.go",        // Uses real gaia login
	"startup_perf.go",       // Doesn't use crostini preconditions at all
	"two_users_install.go",  // Uses real gaia login and doesn't use crostini preconditions
}

var testFiles = []string{
	"audio_basic.go",
	"audio_playback_configurations.go",
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
	"security_cpu_vulnerability.go",
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

var perfTests = map[string]time.Duration{
	"cpu_perf.go":      12 * time.Minute,
	"input_latency.go": 10 * time.Minute,
	"mouse_perf.go":    7 * time.Minute,
	"network_perf.go":  10 * time.Minute,
	"vim_compile.go":   12 * time.Minute,
}

var mainlineExpensiveTests = map[string]time.Duration{
	"fs_corruption.go":     10 * time.Minute,
	"pulse_audio_basic.go": 7 * time.Minute, // Not actually expensive, but broken on stretch.
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
