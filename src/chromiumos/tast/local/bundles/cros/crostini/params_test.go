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
	"chromiumos/tast/local/chrome/devicemode"
	"chromiumos/tast/local/crostini"
)

var testFilesFix = []string{
	"audio_basic.go",
	"audio_playback_configurations.go",
	"basic.go",
	"command_cd.go",
	"command_ps.go",
	"command_vim.go",
	"copy_files_to_linux_files.go",
	"crash_reporter.go",
	"drag_drop.go",
	"files_app_watch.go",
	"home_directory_create_file.go",
	"home_directory_delete_file.go",
	"home_directory_rename_file.go",
	"icon_and_username.go",
	"launch_terminal.go",
	"nested_vm.go",
	"notify.go",
	"no_access_to_downloads.go",
	"no_shared_folder.go",
	"open_with_terminal.go",
	"package_info.go",
	"package_install_uninstall.go",
	"pulse_audio_basic.go",
	"remove_cancel.go",
	"remove_ok.go",
	"resize_cancel.go",
	"resize_ok.go",
	"resize_restart.go",
	"resize_space_constrained.go",
	"restart.go",
	"restart_icon.go",
	"run_with_arc.go",
	"shared_font_files.go",
	"share_downloads_add_files.go",
	"share_downloads.go",
	"share_files_cancel.go",
	"share_files_manage.go",
	"share_files_ok.go",
	"share_files_restart.go",
	"share_files_toast.go",
	"share_folders.go",
	"share_folder_zip_file.go",
	"share_invalid_paths.go",
	"sshfs_mount.go",
	"sync_time.go",
	"task_manager.go",
	"uninstall_invalid_app.go",
	"verify_app_x11.go",
	"vmc_extra_disk.go",
	"vmc_start.go",
	"webserver.go",
	"xattrs.go",
}

var testFilesFixCustomTimeout = map[string]time.Duration{
	// Audio playback configurations took about 6 minutes on model with echo reference
	"audio_playback_configurations.go": 10 * time.Minute,
}

func TestFixTestParams(t *testing.T) {
	for _, filename := range testFilesFix {
		var customTimeout time.Duration
		if timeout, ok := testFilesFixCustomTimeout[filename]; ok {
			customTimeout = timeout
		}
		params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
			Timeout:    customTimeout,
			UseFixture: true,
		}})
		genparams.Ensure(t, filename, params)
	}
}

var lacrosTests = []string{
	"launch_browser.go",
	"verify_app_wayland.go",
}

func TestLacrosTestParams(t *testing.T) {
	for _, filename := range lacrosTests {
		params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
			Timeout:    3 * time.Minute,
			UseFixture: true,
			TestLacros: true,
			Val:        "browser.TypeAsh",
		}})
		genparams.Ensure(t, filename, params)
	}
}

var manateeTests = []string{
	"basic.go",
}

func TestManateeTestParams(t *testing.T) {
	for _, filename := range manateeTests {
		params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
			UseFixture:        true,
			TestManatee:       true,
			ExtraSoftwareDeps: []string{"vm_host"},
		}})
		genparams.Ensure(t, filename, params)
	}
}

var perfTests = map[string]time.Duration{
	"cpu_perf.go":      12 * time.Minute,
	"disk_io_perf.go":  60 * time.Minute,
	"input_latency.go": 10 * time.Minute,
	"mouse_perf.go":    7 * time.Minute,
	"network_perf.go":  10 * time.Minute,
	"startup_perf.go":  1 * time.Minute,
	"vim_compile.go":   20 * time.Minute,
}

var perfTestsExtraData = map[string][]string{
	"vim_compile.go": {"vim.tar.gz"},
}

var mainlineExpensiveTests = map[string]time.Duration{
	"fs_corruption.go":               10 * time.Minute,
	"oom_event.go":                   10 * time.Minute,
	"app_gedit_install_uninstall.go": 12 * time.Minute,
}

func TestExpensiveParams(t *testing.T) {
	for filename, duration := range perfTests {
		params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
			Timeout:       duration,
			MinimalSet:    true,
			IsNotMainline: true,
			UseFixture:    true,
			ExtraData:     perfTestsExtraData[filename],
		}})
		genparams.Ensure(t, filename, params)
	}

	for filename, duration := range mainlineExpensiveTests {
		params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
			Timeout:    duration,
			MinimalSet: true,
			UseFixture: true,
		}})
		genparams.Ensure(t, filename, params)
	}
}

var restartTests = map[string]time.Duration{
	"backup_restore.go":        10 * time.Minute,
	"resize_backup_restore.go": 15 * time.Minute,
}

func TestRestartParams(t *testing.T) {
	for filename, duration := range restartTests {
		params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
			Timeout:    duration,
			MinimalSet: true,
			Restart:    true,
			UseFixture: true,
		}})
		genparams.Ensure(t, filename, params)
	}
}

var appTests = []string{
	"app_android_studio.go",
	"app_eclipse.go",
	"app_emacs.go",
	"app_gedit.go",
	"app_gedit_filesharing.go",
	"app_gedit_unshare_folder.go",
	"app_vscode.go",
	"restart_app.go",
}

func TestAppTestParams(t *testing.T) {
	for _, filename := range appTests {
		params := crostini.MakeTestParamsFromList(t, []crostini.Param{
			{
				Timeout:             15 * time.Minute,
				MinimalSet:          true,
				StableHardwareDep:   "crostini.CrostiniAppStable",
				UnstableHardwareDep: "crostini.CrostiniAppUnstable",
				UseLargeContainer:   true,
				UseFixture:          true,
				DeviceMode:          devicemode.TabletMode,
			},
			{
				Timeout:             15 * time.Minute,
				MinimalSet:          true,
				StableHardwareDep:   "crostini.CrostiniAppStable",
				UnstableHardwareDep: "crostini.CrostiniAppUnstable",
				UseLargeContainer:   true,
				UseFixture:          true,
				DeviceMode:          devicemode.ClamshellMode,
			}})
		genparams.Ensure(t, filename, params)
	}
}

var gaiaTests = []string{
	"no_access_to_drive.go",
	"share_drive.go",
	"share_movies.go",
}

func TestGaiaTestParams(t *testing.T) {
	for _, filename := range gaiaTests {
		params := crostini.MakeTestParamsFromList(t, []crostini.Param{{
			Timeout:      7 * time.Minute,
			UseGaiaLogin: true,
			UseFixture:   true,
		}})
		genparams.Ensure(t, filename, params)
	}
}
