// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"filepath"
	"fmt"
	"os/exec"
	"strings"
)

func ChromeVmoduleFlag() string {
	logging_patterns := []string{
		"*/media/gpu/*video_decode_accelerator.cc=2",
		"*/media/gpu/*video_encode_accelerator.cc=2",
		"*/media/gpu/*jpeg_decode_accelerator.cc=2",
		"*/media/gpu/*jpeg_encode_accelerator.cc=2",
		"*/media/gpu/*image_processor.cc=2",
		"*/media/gpu/*v4l2_device.cc=2",
	}
	chrome_video_vmodule_flag = "--vmodule=" + strings.Join(logging_patterns[:], ",")
	return chrome_video_vmodule_flag
}

type LogInfo struct {
	Files        []string
	EnableValue  string
	DisableValue string
	Desc         string
}

func getLogInfos() []LogInfo {
	info := []LogInfo{}

	//videobuf2 log
	files, _ := filepath.Glob("/sys/module/videobuf2_*/parameters/debug")
	info = append(info, LogInfo{files, "1", "0", "videobuf2 log"})

	// s5p_mfc log
	files, _ := filepath.Glob("/sys/module/s5p_mfc/parameters/debug")
	info = append(info, LogInfo{files, "1", "0", "s5p_mfc log"})

	// rk3399 log
	// rk3399 debug level is controlled by bits.
	// Here, 3 means to enable log level 0 and 1.
	files, _ := filepath.Glob("/sys/module/rockchip_vpu/parameters/debug")
	info = append(info, LogInfo{files, "3", "0", "rk3399 log"})

	// rk3288 log
	// rk3288 debug level is controlled by bits.
	// Here, 3 means to enable log level 0 and 1.
	files, _ := filepath.Glob("/sys/module/rk3288_vpu/parameters/debug")
	info = append(info, LogInfo{files, "3", "0", "rk3288 log"})

	// go2001 log
	files, _ := filepath.Glob("/sys/module/go2001/parameters/go2001_debug_level")
	info = append(info, LogInfo{files, "3", "0", "go2001 log"})

	return info
}

func execCmd(cmd string, msg string) {
	// logging.info
	if out, err := exec.Command(cmd).Run(); err != nil {
		//logging.warning
	}
}

func EnableVideoLogs() []LogInfo {
	info := getLogInfos()
	for _, l := range info {
		for f := range l.Files {
			execCmd(fmt.Sprintf("echo %s > %s", l.EnableValue, f),
				fmt.Sprintf("%s enable", l.Desc))
		}
	}
}

func DisableVideoLogs(info []LogInfo) {
	for _, l := range info {
		for f := range l.Files {
			execCmd(fmt.Sprintf("echo %s > %s", l.DisableValue, f),
				fmt.Sprintf("%s disable", l.Desc))
		}
	}
}
