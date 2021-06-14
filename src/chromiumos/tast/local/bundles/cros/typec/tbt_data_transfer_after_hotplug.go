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
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TbtDataTransferAfterHotplug,
		Desc:         "TBT data tarnsfer after hot plug",
		Contacts:     []string{"pathan.jilani@intel.com", "intel-chrome-system-automation-team@intel.com"},
		Attr:         []string{"group:typec", "typec_lab"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"test_config.json"},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIp"},
		HardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
		Pre:          chrome.LoggedIn(),
	})
}

// TbtDataTransferAfterHotplug performs the following:
// - Hot plug TBT Device into DUT with help of cswitch.
// - Performs TBT Device enumeration check.
// - Performs data transfer from DUT to TBT device.
// - Unplug TBT device from DUT and validates TBT device detection.
//
// This test requires the following H/W topology to run.
//
//
//        DUT ------> C-Switch(device that performs hot plug-unplug)---->TBT SSD.
func TbtDataTransferAfterHotplug(ctx context.Context, s *testing.State) {

	const (
		// Config file which contains expected values of TBT parameters.
		jsonTestConfig = "test_config.json"
		// Source file path in DUT.
		sourcePath = "/home/chronos/user/Downloads/"
		// Source file name.
		transFilename = "file_ogg.ogg"
		// Expected file size to be created.
		fileSize = 10 * 1024 * 1000
		// TBT mount location
		tbtMount = "/media/removable/"
	)
	// TBT port ID in the DUT.
	tbtPort := s.RequiredVar("typec.dutTbtPort")
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIp := s.RequiredVar("typec.domainIp")

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Read json config file.
	json_data, err := ioutil.ReadFile(s.DataPath(jsonTestConfig))
	if err != nil {
		s.Fatal("Failed to read response data: ", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(json_data, &data); err != nil {
		s.Fatal("Failed to read json: ", err)
	}

	// Checking for TBT config data.
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
		if err := cswitch.CloseSession(cleanupCtx, sessionId, domainIp); err != nil {
			s.Log(err)
		}
	}()

	if err := cswitch.ToggleCSwitchPort(ctx, sessionId, cSwitchON, domainIp); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	connected, err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort)
	if !connected {
		s.Fatal(err)
	}

	txSpeed, err := cswitch.TxSpeed(ctx, tbtPort)
	if err != nil {
		s.Fatal(err)
	}

	if strings.TrimSpace(txSpeed) != tbtVal["tx_speed"].(string) {
		s.Fatalf("Failed to verify the tx speed, got %s, want %s", txSpeed, tbtVal["tx_speed"].(string))
	}

	rxSpeed, err := cswitch.RxSpeed(ctx, tbtPort)
	if err != nil {
		s.Fatal(err)
	}

	if strings.TrimSpace(rxSpeed) != tbtVal["rx_speed"].(string) {
		s.Fatalf("Failed to verify the rx speed, got %s, want %s", rxSpeed, tbtVal["rx_speed"].(string))
	}

	nvmVersion, err := cswitch.NvmVersion(ctx, tbtPort)
	if err != nil {
		s.Fatal(err)
	}

	if strings.TrimSpace(nvmVersion) != tbtVal["nvme_version"].(string) {
		s.Fatalf("Failed to verify the nvme version, got %s, want %s", nvmVersion, tbtVal["nvme_version"].(string))
	}

	tbtGeneration, err := cswitch.Generation(ctx, tbtPort)
	if err != nil {
		s.Fatal(err)
	}
	if strings.TrimSpace(tbtGeneration) != tbtVal["generation"].(string) {
		s.Fatalf("Failed to verify the generation, got %s, want %s", tbtGeneration, tbtVal["generation"].(string))
	}

	devicePlugged := tbtVal["device_detection"].(string)
	// Source file path.
	sourceFilePath := path.Join(sourcePath, transFilename)
	// Destination file path.
	destinationFilePath := path.Join(tbtMount, devicePlugged, transFilename)

	// Waits for TBT detection till timeout.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if _, err := os.Stat(path.Join(tbtMount, devicePlugged)); os.IsNotExist(err) {
			return errors.New("Tbt mount path does not exist")
		}
		return nil
	}, &testing.PollOptions{Timeout: 30 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Timeout waiting for TBT device:", err)
	}

	dd_file_create_cmd := fmt.Sprintf("dd if=/dev/zero of=%s count=%d bs=1024", sourceFilePath, fileSize)
	cmd := testexec.CommandContext(ctx, "sh", "-c", dd_file_create_cmd)

	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to create file : ", err)
	}

	s.Logf("Transferring file from %s to %s", sourceFilePath, destinationFilePath)
	copyErr := testexec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("cp -rf %s %s", sourceFilePath, destinationFilePath)).Run()
	if copyErr != nil {
		s.Fatalf("Failed to copy file to %s path", destinationFilePath)
	}

	if err := testexec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("rm -rf %s", destinationFilePath)).Run(); err != nil {
		s.Fatal("Failed to remove file : ", destinationFilePath)
	}

	cSwitchOFF := "0"
	if err := cswitch.ToggleCSwitchPort(ctx, sessionId, cSwitchOFF, domainIp); err != nil {
		s.Fatal("Failed to disable c-switch port: ", err)
	}

	isConnected, err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort)
	if isConnected {
		s.Fatal(err)
	}
}
