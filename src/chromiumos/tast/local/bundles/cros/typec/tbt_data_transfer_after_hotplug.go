// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

const (
	// Config file which contains expected values of TBT parameters
	jsonTestConfig = "test_config.json"
	// Source file path in DUT
	sourcePath = "/home/chronos/user/Downloads/"
	// Source file name
	transFilename = "file_ogg.ogg"
	// Expected file size to be created
	fileSize = 10 * 1024 * 1000
	// TBT mount location
	tbtMount = "/media/removable/"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TbtDataTransferAfterHotplug,
		Desc:         "TBT data tarnsfer after hot plug",
		Contacts:     []string{"pathan.jilani@gmail.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{jsonTestConfig},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIp"},
		HardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
		Pre:          chrome.LoggedIn(),
	})
}

// TbtDataTransferAfterHotplug performs the following:
// - Login to chrome
// - Hot plug TBT Device into DUT with help of cswitch
// - Performs TBT Device enumeration check
// - Performs data transfer from DUT to TBT device
// - Unplug TBT device from DUT and validates TBT device detection
//
// This test requires the following H/W topology to run.
//
//
//        DUT ------> C-Switch(device that performs hot plug-unplug)---->TBT SSD

// TbtDataTransferAfterHotplug func performs data transfer after TBT hot plug.
func TbtDataTransferAfterHotplug(ctx context.Context, s *testing.State) {
	// TBT port ID in the DUT
	tbtPort := s.RequiredVar("typec.dutTbtPort")
	// cswitch port ID
	cSwitchPort := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device
	domainIp := s.RequiredVar("typec.domainIp")

	// Read json config file
	json_data, err := ioutil.ReadFile(s.DataPath(jsonTestConfig))
	if err != nil {
		s.Fatal("Failed to read response data", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(json_data, &data); err != nil {
		s.Fatal("Failed to read json: ", err)
	}

	// checking for TBT config data
	tbtVal, ok := data["TBT"].(map[string]interface{})
	if !ok {
		s.Fatal("Failed to found TBT config data in JSON file")
	}

	// Create C-Switch session that performs hot plug-unplug on TBT device.
	sessionId, err := cswitch.CreateSession(ctx, domainIp)
	if err != nil {
		s.Fatal(err)
	}

	defer func() {
		s.Log("Cleanup..")
		if err := cswitch.CloseSession(ctx, sessionId, domainIp); err != nil {
			s.Fatal(err)
		}
	}()

	// Enabling cswitch port to perform TBT hot plug.
	if err := cswitch.EnableCSwitchPort(ctx, sessionId, cSwitchPort, domainIp); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	// Verifying TBT device name.
	if err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort); err != nil {
		s.Fatal("Failed to match device name: ", err)
	}

	// Verifying TBT device rx_speed.
	if err := cswitch.TxRxSpeed(ctx, tbtPort, tbtVal["tx_speed"].(string), tbtVal["rx_speed"].(string)); err != nil {
		s.Fatal("Failed to match tx/rx speed: ", err)
	}

	// Verifying TBT device nvme version.
	if err := cswitch.NvmeVersion(ctx, tbtPort, tbtVal["nvme_version"].(string)); err != nil {
		s.Fatal("Failed to match nvme version: ", err)
	}

	// Verifying TBT device nvme generation.
	if err = cswitch.Generation(ctx, tbtPort, tbtVal["generation"].(string)); err != nil {
		s.Fatal("Failed to match generation: ", err)
	}

	s.Log("TBT device plugged sucessfully")
	devicePlugged := tbtVal["device_detection"].(string)
	// Source file path
	sourceFilePath := path.Join(sourcePath, transFilename)
	// Destination file path
	destinationFilePath := path.Join(tbtMount, devicePlugged, transFilename)

	// Waits for TBT detection till timeout
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(path.Join(tbtMount, devicePlugged)); os.IsNotExist(err) {
			return errors.New("waiting for detection")
		}

		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 10 * time.Second}); err != nil {
		s.Fatalf("failed to detect device")
	}

	s.Log("Creating file to perform data transfer")
	if err := createFile(ctx, path.Join(sourcePath, transFilename), fileSize); err != nil {
		s.Fatal("Failed to create file : ", err)
	}

	s.Logf("Transferring file from %s to %s", sourceFilePath, destinationFilePath)
	copyErr := testexec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("cp -rf %s %s", sourceFilePath, destinationFilePath)).Run()
	if copyErr != nil {
		s.Fatalf("Failed to copy file to %s path", destinationFilePath)
	}

	s.Log("Clearing transfered file")
	if err := testexec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("rm -rf %s", destinationFilePath)).Run(); err != nil {
		s.Fatal("Failed to remove file : ", destinationFilePath)
	}

	// Disabling cswitch port to perform TBT hot unplug
	if err := cswitch.DisableCSwitchPort(ctx, sessionId, domainIp); err != nil {
		s.Fatal("Failed to disable c-switch port: ", err)
	}

	// Validating TBT device enumeration
	if err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort); err == nil {
		s.Fatal("Device still detecting after unplug: ", err)
	}
	s.Log("TBT device unplugged sucessfully")
}

// Creating a file to perform file transfer
// filespath holds the location of files creation
// fileSize size of the files
func createFile(ctx context.Context, filesPath string, fileSize int) error {
	dd_file_create_cmd := fmt.Sprintf("dd if=/dev/zero of=%s count=%d bs=1024", filesPath, fileSize)
	cmd := testexec.CommandContext(ctx, "sh", "-c", dd_file_create_cmd)
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to create file")
	}
	return nil
}
