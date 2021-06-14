// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/typec/typecutils"
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
		Attr:         []string{"group:typec"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"test_config.json", "testcert.p12"},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIP", "ui.signinProfileTestExtensionManifestKey"},
		HardwareDeps: hwdep.D(hwdep.Model("volteer", "voxel")),
		Timeout:      5 * time.Minute,
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
		downloadsPath = "/home/chronos/user/Downloads/"
		// Source file name.
		transFilename = "file_ogg.ogg"
		// Expected file size to be created.
		fileSize = 10 * 1024 * 1000
		// TBT mount location
		tbtMount = "/media/removable/"
		//Test directory
		tempDir = "temp"
	)
	// TBT port ID in the DUT.
	tbtPort := s.RequiredVar("typec.dutTbtPort")
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("typec.domainIP")

	// Shorten deadline to leave time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Get to the Chrome login screen.
	cr, err := chrome.New(ctx,
		chrome.DeferLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))
	if err != nil {
		s.Fatal("Failed to start Chrome at login screen: ", err)
	}
	defer cr.Close(ctx)

	if err := typecutils.EnablePeripheralDataAccess(ctx, s.DataPath("testcert.p12")); err != nil {
		s.Fatal("Failed to enable peripheral data access setting: ", err)
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to login: ", err)
	}

	// Read json config file.
	jsonData, err := ioutil.ReadFile(s.DataPath(jsonTestConfig))
	if err != nil {
		s.Fatal("Failed to read response data: ", err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		s.Fatal("Failed to read json: ", err)
	}

	// Checking for TBT config data.
	tbtVal, ok := data["TBT"].(map[string]interface{})
	if !ok {
		s.Fatal("Failed to found TBT config data in JSON file")
	}

	// Create C-Switch session that performs hot plug-unplug on TBT device.
	sessionID, err := cswitch.CreateSession(ctx, domainIP)
	if err != nil {
		s.Fatal("Failed to create session: ", err)
	}

	defer func(ctx context.Context) {
		s.Log("Cleanup..")
		if err := cswitch.CloseSession(ctx, sessionID, domainIP); err != nil {
			s.Log("Failed to close session: ", err)
		}
	}(cleanupCtx)

	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	if _, err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort); err != nil {
		s.Fatal("Failed to enumerate TBT device :", err)
	}

	tbtGeneration, err := cswitch.Generation(ctx, tbtPort)
	if err != nil {
		s.Fatal("Failed to get TBT genaration: ", err)
	}
	if strings.TrimSpace(tbtGeneration) != tbtVal["generation"].(string) {
		s.Fatalf("Failed to verify the generation, got %s, want %s", tbtGeneration, tbtVal["generation"].(string))
	}

	devicePlugged := tbtVal["device_detection"].(string)

	sourcePath := path.Join(downloadsPath, tempDir)
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		s.Fatal("Failed to create temp directory: ", err)
	}

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

	ddFileCreateCmd := fmt.Sprintf("dd if=/dev/urandom of=%s count=%d bs=1024", sourceFilePath, fileSize)
	cmd := testexec.CommandContext(ctx, "sh", "-c", ddFileCreateCmd)

	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to create file : ", err)
	}
	localHash, err := calculateHashOfFile(sourceFilePath)
	if err != nil {
		s.Errorf("Failed to calculate hash of the source file : %v", err)
	}

	s.Logf("Transferring file from %s to %s", sourceFilePath, destinationFilePath)
	copyErr := testexec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("cp -rf %s %s", sourceFilePath, destinationFilePath)).Run()
	if copyErr != nil {
		s.Fatalf("Failed to copy file to %s path", destinationFilePath)
	}

	destHash, err := calculateHashOfFile(destinationFilePath)
	if err != nil {
		s.Errorf("Failed to calculate hash of the destination file : %v", err)
	}

	if !bytes.Equal(localHash, destHash) {
		s.Errorf(" The hash doesn't match (destHash path: %q)", destHash)
	}
	// Clearing file from destination path.
	if err := testexec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("rm -rf %s", destinationFilePath)).Run(); err != nil {
		s.Fatal("Failed to remove file : ", destinationFilePath)
	}
	// Clearing file from source path.
	if err := testexec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("rm -rf %s", sourcePath)).Run(); err != nil {
		s.Fatal("Failed to remove file : ", sourcePath)
	}

	cSwitchOFF := "0"
	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
		s.Fatal("Failed to disable c-switch port: ", err)
	}

	if _, err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort); err == nil {
		s.Fatal("Failed to hot unplug the TBT device: ", err)
	}
}

func calculateHashOfFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return []byte{}, errors.Wrap(err, "failed to open files")
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return []byte{}, errors.Wrap(err, "failed to calculate the hash of the files")
	}

	return h.Sum(nil), nil
}
