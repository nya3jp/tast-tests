// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"bytes"
	"context"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/servo"
	"chromiumos/tast/common/usbutils"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/powercontrol"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/power"
	"chromiumos/tast/testing"
)

type usbPowerMode int

const (
	warmboot usbPowerMode = iota
	coldboot
)

type usbTypeATestParam struct {
	powerMode       usbPowerMode
	usbSpeed        string
	cbmemSleepState int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         USBTypeAStorageFunctionality,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies USB type-A storage device functionality on warmboot/coldboot operation",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome", "reboot"},
		ServiceDeps:  []string{"tast.cros.power.USBService"},
		VarDeps:      []string{"servo"},
		Params: []testing.Param{{
			Name:    "usb2_warmboot",
			Val:     usbTypeATestParam{warmboot, "480M", 0},
			Timeout: 5 * time.Minute,
		}, {
			Name:    "usb2_coldboot",
			Val:     usbTypeATestParam{coldboot, "480M", 5},
			Timeout: 5 * time.Minute,
		}, {
			Name:    "usb3_warmboot",
			Val:     usbTypeATestParam{warmboot, "5000M", 0},
			Timeout: 5 * time.Minute,
		}, {
			Name:    "usb3_coldboot",
			Val:     usbTypeATestParam{coldboot, "5000M", 5},
			Timeout: 5 * time.Minute,
		}}})
}

func USBTypeAStorageFunctionality(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 3*time.Minute)
	defer cancel()

	dut := s.DUT()
	testParam := s.Param().(usbTypeATestParam)

	servoSpec := s.RequiredVar("servo")
	pxy, err := servo.NewProxy(ctx, servoSpec, dut.KeyFile(), dut.KeyDir())
	if err != nil {
		s.Fatal("Failed to connect to servo: ", err)
	}
	defer pxy.Close(cleanupCtx)

	// Connect to gRPC server.
	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	client := power.NewUSBServiceClient(cl.Conn)
	if _, err := client.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer client.CloseChrome(cleanupCtx, &empty.Empty{})

	initialMuxState, err := pxy.Servo().GetUSBMuxState(ctx)
	if err != nil {
		s.Fatal("Failed to get USB Mux state info: ", err)
	}
	defer pxy.Servo().SetUSBMuxState(cleanupCtx, initialMuxState)

	defer func(ctx context.Context) {
		if !dut.Connected(ctx) {
			if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
				s.Error("Failed to power on DUT in cleanup: ", err)
			}
		}
	}(cleanupCtx)

	if err := pxy.Servo().SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
		s.Fatal("Failed to plug USB storage device to DUT: ", err)
	}

	// Check for USB storage device detection before warmboot/coldboot.
	if err := validateUSBStorageDetection(ctx, dut, testParam.usbSpeed); err != nil {
		s.Fatal("Failed to detect connected USB storage device before warmboot/coldboot: ", err)
	}

	switch testParam.powerMode {
	case warmboot:
		s.Log("Performing warmboot")
		if err := dut.Reboot(ctx); err != nil {
			s.Fatal("Failed to warmboot DUT: ", err)
		}

	case coldboot:
		s.Log("Performing coldboot")
		powerState := "S5"
		if err := powercontrol.ShutdownAndWaitForPowerState(ctx, pxy, dut, powerState); err != nil {
			s.Fatalf("Failed to shutdown and wait for %q powerstate: %v", powerState, err)
		}
		if err := powercontrol.PowerOntoDUT(ctx, pxy, dut); err != nil {
			s.Fatal("Failed to power on DUT: ", err)
		}
	}

	// Login to Chrome after warmboot/coldboot.
	cl, err = rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	client = power.NewUSBServiceClient(cl.Conn)
	if _, err := client.NewChrome(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer client.CloseChrome(cleanupCtx, &empty.Empty{})

	// Check for prev_sleep_state after warmboot/coldboot.
	if err := powercontrol.ValidatePrevSleepState(ctx, dut, testParam.cbmemSleepState); err != nil {
		s.Fatal("Failed to validate previous sleep state: ", err)
	}

	// Check for USB storage device detection after warmboot/coldboot.
	if err := validateUSBStorageDetection(ctx, dut, testParam.usbSpeed); err != nil {
		s.Fatal("Failed to detect connected USB storage device after warmboot/coldboot: ", err)
	}

	// Unplug USB storage device after warmboot/coldboot.
	if err := pxy.Servo().SetUSBMuxState(ctx, servo.USBMuxHost); err != nil {
		s.Fatal("Failed to unplug USB storage device to DUT: ", err)
	}

	// Check for USB storage device detection after unplug.
	if err := validateUSBStorageDetection(ctx, dut, testParam.usbSpeed); err == nil {
		s.Fatal("Failed USB storage device still detecting after unplug: ", err)
	}

	const fileName = "test_sample_file.txt"
	const fileSize = 1 * 1024 * 1024
	fileNameAndFileSize := &power.TestFileRequest{FileName: fileName, FileSize: int64(fileSize)}
	sourceFilePath, err := client.GenerateTestFile(ctx, fileNameAndFileSize)
	if err != nil {
		s.Fatal("Failed to create temp file: ", err)
	}
	defer client.RemoveFile(cleanupCtx, &power.TestFileRequest{Path: sourceFilePath.Path})

	// Again plug USB storage device and check for its detection.
	// If detected tranfer file from DUT to USB device and vice-versa.
	if err := pxy.Servo().SetUSBMuxState(ctx, servo.USBMuxDUT); err != nil {
		s.Fatal("Failed to plug USB storage device to DUT: ", err)
	}

	var dirsAfterPlug *power.MountPathResponse
	// Waits for USB pendrive detection till timeout.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		dirsAfterPlug, err = client.USBMountPaths(ctx, &empty.Empty{})
		if err != nil {
			return errors.Wrap(err, "failed to get removable devices")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Timeout waiting for USB pendrive mount path: ", err)
	}

	devicePath := dirsAfterPlug.MountPaths[0]

	// Destination file path.
	destinationFilePath := filepath.Join(devicePath, fileName)
	defer client.RemoveFile(cleanupCtx, &power.TestFileRequest{Path: destinationFilePath})

	localHash, err := client.FileChecksum(ctx, &power.TestFileRequest{Path: sourceFilePath.Path})
	if err != nil {
		s.Error("Failed to calculate hash of the source file: ", err)
	}

	// Tranferring file from DUT to USB storage device.
	testing.ContextLogf(ctx, "Transferring file from %s to %s", sourceFilePath.Path, destinationFilePath)
	transferFiles := &power.TestFileRequest{SourceFilePath: sourceFilePath.Path, DestinationFilePath: destinationFilePath}
	if _, err := client.CopyFile(ctx, transferFiles); err != nil {
		s.Fatal("Failed to copy file: ", err)
	}

	destHash, err := client.FileChecksum(ctx, &power.TestFileRequest{Path: destinationFilePath})
	if err != nil {
		s.Error("Failed to calculate hash of the destination file: ", err)
	}

	if !bytes.Equal(localHash.FileChecksumValue, destHash.FileChecksumValue) {
		s.Errorf("Failed: The hash doesn't match: got %v, want %v", localHash.FileChecksumValue, destHash.FileChecksumValue)
	}

	// Tranferring file from USB storage device to DUT.
	testing.ContextLogf(ctx, "Transferring file from %s to %s", destinationFilePath, sourceFilePath.Path)
	transferFiles = &power.TestFileRequest{SourceFilePath: destinationFilePath, DestinationFilePath: sourceFilePath.Path}
	if _, err := client.CopyFile(ctx, transferFiles); err != nil {
		s.Fatal("Failed to copy file: ", err)
	}

	// Check for USB storage device detection after transferring file.
	if err := validateUSBStorageDetection(ctx, dut, testParam.usbSpeed); err != nil {
		s.Fatal("Failed to detect USB storage device after transferring file: ", err)
	}
}

// validateUSBStorageDetection checks for connected USB storage detection.
func validateUSBStorageDetection(ctx context.Context, dut *dut.DUT, usbSpeed string) error {
	usbDeviceClassName := "Mass Storage"
	return testing.Poll(ctx, func(ctx context.Context) error {
		usbDevicesList, err := usbutils.ListDevicesInfo(ctx, dut)
		if err != nil {
			return errors.Wrap(err, "failed to get USB devices list")
		}
		got := usbutils.NumberOfUSBDevicesConnected(usbDevicesList, usbDeviceClassName, usbSpeed)
		if want := 1; got != want {
			return errors.Errorf("unexpected number of %q devices connected with %q speed: got %d, want %d",
				usbDeviceClassName, usbSpeed, got, want)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second})
}
