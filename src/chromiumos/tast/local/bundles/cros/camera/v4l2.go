// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/local/gtest"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/shutil"
	"chromiumos/tast/testing"
)

const hwTimestampsPath = "/sys/module/uvcvideo/parameters/hwtimestamps"

func init() {
	testing.AddTest(&testing.Test{
		Func: V4L2,
		Desc: "Verifies required V4L2 operations on USB camera devices",
		Contacts: []string{
			"shik@chromium.org",
			"henryhsu@chromium.org",
			"chromeos-camera-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{caps.BuiltinUSBCamera},
	})
}

func V4L2(ctx context.Context, s *testing.State) {
	hasHWTimestamps, err := pathExist(hwTimestampsPath)
	if err != nil {
		s.Fatal("Failed to check hardware timestamps: ", err)
	}
	if hasHWTimestamps {
		origVal, err := setHWTimestamps("1")
		if err != nil {
			s.Fatal("Failed to set hardware timestamps: ", err)
		}
		defer setHWTimestamps(origVal)
	}

	testList, err := getTestList()
	if err != nil {
		s.Fatal("Failed to get test list: ", err)
	}

	usbCams, err := getUSBCamerasFromV4L2Test(ctx)
	if err != nil {
		s.Fatal("Failed to get USB cameras: ", err)
	}
	if len(usbCams) == 0 {
		s.Fatal("Failed to find any valid device")
	}
	s.Log("USB cameras: ", usbCams)

	for _, devicePath := range usbCams {
		extraArgs := []string{
			"--device_path=" + devicePath,
			"--test_list=" + testList,
		}

		logFile := fmt.Sprintf("media_v4l2_test_%s.log", filepath.Base(devicePath))

		t := gtest.New("media_v4l2_test",
			gtest.Logfile(filepath.Join(s.OutDir(), logFile)),
			gtest.ExtraArgs(extraArgs...))

		if args, err := t.Args(); err == nil {
			s.Log("Running " + shutil.EscapeSlice(args))
		}

		report, err := t.Run(ctx)
		if err != nil {
			if report != nil {
				for _, name := range report.FailedTestNames() {
					s.Error(name, " failed")
				}
			}
			s.Fatal("Failed to run media_v4l2_test: ", err)
		}
	}

}

func pathExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func isV1Legacy() bool {
	// For unibuild, we can determine if a device is v1 legacy by checking
	// 'legacy-usb' under path '/camera' in cros_config.
	c := testexec.CommandContext(ctx, "cros_config", "/camera", "legacy-usb")
	crosConfigOutput, err := c.Output()
	if err == nil && string(crosConfigOutput) == "true" {
		return true
	}

	// For non-unibuild, we can check if 'v1device' presents in the config file
	// '/etc/camera/camera_chracteristics.conf'.
	configFilePath := "/etc/camera/camera_characteristics.conf"
	if config, err := ioutil.ReadFile(configFilePath); err == nil {
		if strings.Contains(string(config), "v1device") {
			return true
		}
	}
	return false
}

func setHWTimestamps(newValue string) (oldValue string, err error) {
	b, err := ioutil.ReadFile(hwTimestampsPath)
	if err != nil {
		return "", err
	}
	oldValue = string(b)

	if err := ioutil.WriteFile(hwTimestampsPath, []byte(newValue), 0644); err != nil {
		return "", err
	}
	return oldValue, err
}

func getTestList() (string, error) {
	const (
		v3Path = "/usr/bin/cros_camera_service"
	)

	hasV3, err := pathExist(v3Path)
	if err != nil {
		return "", err
	}

	if hasV3 && !isV1Legacy() {
		return "halv3", nil
	}

	return "default", nil
}

func getUSBCamerasFromV4L2Test(ctx context.Context) ([]string, error) {
	cmd := testexec.CommandContext(ctx, "media_v4l2_test", "--list_usbcam")
	out, err := cmd.Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}
