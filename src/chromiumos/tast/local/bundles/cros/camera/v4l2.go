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
		Attr:         []string{"informational"},
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

	devicePaths, err := filepath.Glob("/dev/video*")
	if err != nil {
		s.Fatal("Failed to glob /dev/video*: ", err)
	}

	foundAny := false

	for _, devicePath := range devicePaths {
		if !isV4L2CaptureDevice(ctx, devicePath) {
			s.Logf("Skip %v because it's not a V4L2 capture device", devicePath)
			continue
		}

		usbInfo, err := getUSBInfo(devicePath)
		if os.IsNotExist(err) {
			s.Logf("Skip %v because it's not an USB device", devicePath)
			continue
		}
		if err != nil {
			s.Fatalf("Failed to get USB info of %v: %v", devicePath, err)
		}

		foundAny = true

		extraArgs := []string{
			"--device_path=" + devicePath,
			"--test_list=" + testList,
			"--usb_info=" + usbInfo,
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

	if !foundAny {
		s.Fatal("Failed to find any valid device")
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
		v1Path = "/usr/bin/arc_camera_service"
		v3Path = "/usr/bin/cros_camera_service"
	)

	hasV1, err := pathExist(v1Path)
	if err != nil {
		return "", err
	}

	hasV3, err := pathExist(v3Path)
	if err != nil {
		return "", err
	}

	if hasV3 && !hasV1 {
		return "halv3", nil
	}

	return "default", nil
}

func isV4L2CaptureDevice(ctx context.Context, path string) bool {
	err := testexec.CommandContext(ctx, "media_v4l2_is_capture_device", path).Run()
	return err == nil
}

func getUSBInfo(path string) (string, error) {
	baseName := filepath.Base(path)
	vidPath := fmt.Sprintf("/sys/class/video4linux/%s/device/../idVendor", baseName)
	pidPath := fmt.Sprintf("/sys/class/video4linux/%s/device/../idProduct", baseName)

	b, err := ioutil.ReadFile(vidPath)
	if err != nil {
		return "", err
	}
	vid := strings.TrimSpace(string(b))

	b, err = ioutil.ReadFile(pidPath)
	if err != nil {
		return "", err
	}
	pid := strings.TrimSpace(string(b))

	return fmt.Sprintf("%s:%s", vid, pid), nil
}
