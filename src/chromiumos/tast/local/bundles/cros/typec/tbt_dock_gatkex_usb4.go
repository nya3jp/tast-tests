// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/typec/setup"
	typecutilshelper "chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/cswitch"
	"chromiumos/tast/local/typecutils"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         TBTDockGatkexUSB4,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies test TBT Docking station on USB4 gatkex card TBT port via USB4 port using 40G passive cable",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:typec"},
		Data:         []string{"test_config.json", "testcert.p12", "bear-320x240.h264.mp4", "video.html", "playback.js"},
		Vars:         []string{"typec.dutTbtPort", "typec.cSwitchPort", "typec.domainIP", "typec.tbtDockPort", "ui.signinProfileTestExtensionManifestKey"},
		Fixture:      "chromeLoggedInThunderbolt",
		HardwareDeps: hwdep.D(setup.ThunderboltSupportedDevices()),
		Timeout:      7 * time.Minute,
	})
}

func TBTDockGatkexUSB4(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	const (
		// Config file which contains expected values of TBT parameters.
		testConfig = "test_config.json"
		// Source file name.
		transFilename = "tbt_usb3_test_file.txt"
	)

	// TBT port ID in the DUT.
	tbtPort := s.RequiredVar("typec.dutTbtPort")
	// Dock port ID in the DUT.
	dockPort := s.RequiredVar("typec.tbtDockPort")
	// cswitch port ID.
	cSwitchON := s.RequiredVar("typec.cSwitchPort")
	// IP address of Tqc server hosting device.
	domainIP := s.RequiredVar("typec.domainIP")

	// Media removable path.
	const mediaRemovable = "/media/removable/"

	if err := typecutils.EnablePeripheralDataAccess(ctx, s.DataPath("testcert.p12")); err != nil {
		s.Fatal("Failed to enable peripheral data access setting: ", err)
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		s.Fatal("Failed to login: ", err)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal(s, "Failed to create test API connection: ", err)
	}

	// Read json config file.
	jsonData, err := ioutil.ReadFile(s.DataPath(testConfig))
	if err != nil {
		s.Fatalf("Failed to open %v file : %v", testConfig, err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		s.Fatal("Failed to read json: ", err)
	}

	// Checking for USB4 config data.
	tbtVal, ok := data["TBT"].(map[string]interface{})
	if !ok {
		s.Fatal("Failed to find TBT config data in JSON file")
	}

	dirsBeforePlug, err := typecutilshelper.RemovableDirs(mediaRemovable)
	if err != nil {
		s.Fatal("Failed to get removable devices: ", err)
	}

	// Create C-Switch session that performs hot plug-unplug on TBT device.
	sessionID, err := cswitch.CreateSession(ctx, domainIP)
	if err != nil {
		s.Fatal("Failed to create sessionID: ", err)
	}

	const cSwitchOFF = "0"
	defer func(ctx context.Context) {
		s.Log("Cleanup")
		if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchOFF, domainIP); err != nil {
			s.Fatal("Failed to disable c-switch port: ", err)
		}
		if err := cswitch.CloseSession(cleanupCtx, sessionID, domainIP); err != nil {
			s.Log("Failed to close sessionID: ", err)
		}
	}(cleanupCtx)

	if err := cswitch.ToggleCSwitchPort(ctx, sessionID, cSwitchON, domainIP); err != nil {
		s.Fatal("Failed to enable c-switch port: ", err)
	}

	if err := typecutils.CheckUSBPdMuxinfo(ctx, "USB4=1"); err != nil {
		s.Fatal("Failed to verify dmesg logs: ", err)
	}

	if _, err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_name"].(string), tbtPort); err != nil {
		s.Fatal("Failed to enumerate the TBT device: ", err)
	}

	if _, err := cswitch.IsDeviceEnumerated(ctx, tbtVal["device_detection"].(string), dockPort); err != nil {
		s.Fatal("Failed to enumerate the TBT device: ", err)
	}

	sourcePath, err := ioutil.TempDir("", "temp")
	if err != nil {
		s.Fatal("Failed to create temp directory: ", err)
	}

	defer os.RemoveAll(sourcePath)

	// Source file path.
	sourceFilePath := path.Join(sourcePath, transFilename)

	// Create a file with size.
	file, err := os.Create(sourceFilePath)
	if err != nil {
		s.Fatal("Failed to create file: ", err)
	}
	if err := file.Truncate(int64(1024 * 1024 * 1024 * 2)); err != nil {
		s.Fatal("Failed to truncate file with size: ", err)
	}

	var dirsAfterPlug []string
	// Waits for USB pendrive detection till timeout.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		dirsAfterPlug, err = typecutilshelper.RemovableDirs(mediaRemovable)
		if err != nil {
			return errors.Wrap(err, "failed to get removable devices")
		}
		if len(dirsBeforePlug) >= len(dirsAfterPlug) {
			return errors.New("failed to mount removable devices")
		}
		return nil
	}, &testing.PollOptions{Timeout: 40 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Timeout waiting for USB pendrive detection: ", err)
	}

	// Verify USB pendrive speed.
	speedOut, err := typecutilshelper.MassStorageUSBSpeed(ctx)
	if err != nil {
		s.Fatal("Failed to check for USB speed: ", err)
	}

	typecUSBSpeed := "5000M"
	speedFound := false
	for _, speed := range speedOut {
		if speed == typecUSBSpeed {
			speedFound = true
			break
		}
	}
	if !speedFound {
		s.Fatalf("Unexpected USB device speed: want %q, got %q", typecUSBSpeed, speedOut)
	}
	devicePath := typecutilshelper.TbtMountPath(dirsAfterPlug, dirsBeforePlug)
	if devicePath == "" {
		s.Fatal("Failed to get vaild devicePath")
	}

	// Destination file path.
	destinationFilePath := path.Join(mediaRemovable, devicePath, transFilename)

	defer os.Remove(destinationFilePath)

	localHash, err := typecutilshelper.FileChecksum(sourceFilePath)
	if err != nil {
		s.Error("Failed to calculate hash of the source file: ", err)
	}

	// Tranferring file from source to destination.
	testing.ContextLogf(ctx, "Transferring file from %s to %s", sourceFilePath, destinationFilePath)
	if err := typecutilshelper.CopyFile(sourceFilePath, destinationFilePath); err != nil {
		s.Fatal("Failed to copy file: ", err)
	}

	destHash, err := typecutilshelper.FileChecksum(destinationFilePath)
	if err != nil {
		s.Error("Failed to calculate hash of the destination file: ", err)
	}

	if !bytes.Equal(localHash, destHash) {
		s.Errorf("The hash doesn't match (destHash path: %q)", destHash)
	}

	// Tranferring file from destination to source.
	testing.ContextLogf(ctx, "Transferring file from %s to %s", destinationFilePath, sourceFilePath)
	if err := typecutilshelper.CopyFile(destinationFilePath, sourceFilePath); err != nil {
		s.Fatal("Failed to copy file: ", err)
	}

	if err := typecutils.FindConnectedDisplay(ctx, 1); err != nil {
		s.Fatal("Failed to find connected display: ", err)
	}

	if err := typecutils.CheckDisplayInfo(ctx, false, true); err != nil {
		s.Fatal("Failed to check display info : ", err)
	}

	if err := typecutils.VerifyDisplay4KResolution(ctx); err != nil {
		s.Fatal("Failed to Verify display 4K resolution: ", err)
	}

	// Set mirror mode display.
	if err := typecutils.SetMirrorDisplay(ctx, tconn, true); err != nil {
		s.Fatal("Failed to set mirror mode: ", err)
	}

	srv := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer srv.Close()
	url := srv.URL + "/video.html"
	conn, err := cr.NewConn(ctx, url)

	if err != nil {
		s.Fatal("Failed to load video.html: ", err)
	}

	videoFile := "bear-320x240.h264.mp4"
	if err := conn.Call(ctx, nil, "playRepeatedly", videoFile); err != nil {
		s.Fatal("Failed to play video: ", err)
	}
	defer conn.Close()
}
