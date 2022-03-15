// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package audio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/firmware"
	"chromiumos/tast/remote/firmware/fixture"
	"chromiumos/tast/rpc"
	"chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         PlayAudioWithDisplayOff,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies audio playback during display off with charger connected and disconnected",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		ServiceDeps:  []string{"tast.cros.ui.AudioService"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay(), hwdep.Speaker()),
		Fixture:      fixture.NormalMode,
	})
}

func PlayAudioWithDisplayOff(ctx context.Context, s *testing.State) {
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	dut := s.DUT()
	h := s.FixtValue().(*fixture.Value).Helper
	if err := h.RequireServo(ctx); err != nil {
		s.Fatal("Failed to init servo: ", err)
	}

	if err := h.RequireConfig(ctx); err != nil {
		s.Fatal("Failed to create config: ", err)
	}

	const expectedAudioOuputNode = "INTERNAL_SPEAKER"

	defer func(ctx context.Context) {
		testing.ContextLog(ctx, "Performing cleanup")
		if err := plugUnplugCharger(ctx, h, true); err != nil {
			s.Fatal("Failed to plug charger at cleanup: ", err)
		}
	}(ctxForCleanUp)

	// Login to Chrome OS.
	cl, err := rpc.Dial(ctx, dut, s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer cl.Close(ctx)
	audioService := ui.NewAudioServiceClient(cl.Conn)
	if _, err := audioService.New(ctx, &empty.Empty{}); err != nil {
		s.Fatal("Failed to login Chrome: ", err)
	}

	initialBrightness, err := systemBrightness(ctx, dut)
	if err != nil {
		s.Fatal("Failed to get system display initial brightness: ", err)
	}

	performTest := func() {
		// Generate sine raw input file that lasts 60 seconds.
		const rawFileName = "AudioFile.raw"
		const downloadsPath = "/home/chronos/user/Downloads/"
		rawFilePath := filepath.Join(downloadsPath, rawFileName)
		rawDataFields := &ui.AudioServiceRequest{FilePath: rawFilePath, DurationInSecs: 60}
		if _, err := audioService.GenerateTestRawData(ctx, rawDataFields); err != nil {
			s.Fatal("Failed to generate test raw data file: ", err)
		}
		defer os.Remove(rawFilePath)

		const wavFileName = "AudioFile.wav"
		wavFile := filepath.Join(downloadsPath, wavFileName)
		convertRawFileFields := &ui.AudioServiceRequest{FilePath: rawFilePath, FileName: wavFile}
		if _, err := audioService.ConvertRawToWav(ctx, convertRawFileFields); err != nil {
			s.Fatal("Failed to convert raw to wav: ", err)
		}
		defer os.Remove(wavFile)

		dirAndFileName := &ui.AudioServiceRequest{DirectoryName: "Downloads", FileName: wavFileName}
		if _, err := audioService.OpenDirectoryAndFile(ctx, dirAndFileName); err != nil {
			s.Fatal("Failed to open local audio file: ", err)
		}

		deviceName, err := audioService.AudioCrasSelectedOutputDevice(ctx, &empty.Empty{})
		if err != nil {
			s.Fatal("Failed to get output audio device info: ", err)
		}

		if deviceName.DeviceType != expectedAudioOuputNode {
			expectedAudioNode := &ui.AudioServiceRequest{Expr: expectedAudioOuputNode}
			if _, err := audioService.SetActiveNodeByType(ctx, expectedAudioNode); err != nil {
				s.Fatal("Failed to select output audio node: ", err)
			}
			deviceName, err = audioService.AudioCrasSelectedOutputDevice(ctx, &empty.Empty{})
			if err != nil {
				s.Fatal("Failed to get output audio device info: ", err)
			}
			if deviceName.DeviceType != expectedAudioOuputNode {
				s.Fatalf("Failed to select audio device %q: %v", expectedAudioOuputNode, err)
			}
		}

		runningDeviceName := &ui.AudioServiceRequest{Expr: deviceName.DeviceName}
		if _, err := audioService.VerifyFirstRunningDevice(ctx, runningDeviceName); err != nil {
			s.Fatalf("Failed to route audio through %q: %v", expectedAudioOuputNode, err)
		}

		if initialBrightness > 0 {
			testing.ContextLog(ctx, "Waiting for display to go off")
			if err := setSystemBrightness(ctx, dut, 0); err != nil {
				s.Fatal("Failed DUT to go for display off state: ", err)
			}
		}
		defer setSystemBrightness(ctxForCleanUp, dut, initialBrightness)

		curBrightness, err := systemBrightness(ctx, dut)
		if err != nil {
			s.Fatal("Failed to get system current brightness in idle state: ", err)
		}
		if curBrightness != 0 {
			s.Fatal("Failed to go to idle state")
		}

		if _, err := audioService.VerifyFirstRunningDevice(ctx, runningDeviceName); err != nil {
			s.Fatalf("Failed to route audio through %q while system display off: %v", expectedAudioOuputNode, err)
		}

		if err := setSystemBrightness(ctx, dut, initialBrightness); err != nil {
			s.Fatal("Failed to reset DUT display brightness: ", err)
		}

		curBrightness, err = systemBrightness(ctx, dut)
		if err != nil {
			s.Fatal("Failed to get system current brightness: ", err)
		}
		if curBrightness != initialBrightness {
			s.Fatal("Failed: DUT display still in idle state")
		}

		// Closing music player.
		accelKeys := &ui.AudioServiceRequest{Expr: "Ctrl+W"}
		if _, err := audioService.KeyboardAccel(ctx, accelKeys); err != nil {
			s.Fatal("Failed to close music player: ", err)
		}
	}

	// Perform audio playback test while display off during charger unplugged.
	if err := plugUnplugCharger(ctx, h, false); err != nil {
		s.Fatal("Failed to unplug charger: ", err)
	} else {
		performTest()
	}

	// Perform audio playback test while display off during charger plugged.
	if err := plugUnplugCharger(ctx, h, true); err != nil {
		s.Fatal("Failed to plug charger: ", err)
	} else {
		performTest()
	}
}

// systemBrightness returns system display brightness value.
func systemBrightness(ctx context.Context, dut *dut.DUT) (int, error) {
	bnsOut, err := dut.Conn().CommandContext(ctx, "backlight_tool", "--get_brightness").Output()
	if err != nil {
		return 0, errors.Wrap(err, "failed to execute backlight_tool command")
	}
	brightness, err := strconv.Atoi(strings.TrimSpace(string(bnsOut)))
	if err != nil {
		return 0, errors.Wrap(err, "failed to convert string to integer")
	}
	return brightness, nil
}

// plugUnplugCharger performs plugging/unplugging of charger via servo.
func plugUnplugCharger(ctx context.Context, h *firmware.Helper, isPowerPlugged bool) error {
	chargerStatus := ""
	if isPowerPlugged {
		testing.ContextLog(ctx, "Starting power supply")
		chargerStatus = "not attached"
	} else {
		testing.ContextLog(ctx, "Stopping power supply")
		chargerStatus = "attached"
	}
	if err := h.SetDUTPower(ctx, isPowerPlugged); err != nil {
		return errors.Wrap(err, "failed to remove charger")
	}
	getChargerPollOptions := testing.PollOptions{Timeout: 10 * time.Second}
	return testing.Poll(ctx, func(ctx context.Context) error {
		if attached, err := h.Servo.GetChargerAttached(ctx); err != nil {
			return err
		} else if isPowerPlugged != attached {
			return errors.Errorf("charger is still %q - use Servo V4 Type-C or supply RPM vars", chargerStatus)
		}
		return nil
	}, &getChargerPollOptions)
}

// setSystemBrightness will sets system brightness to given brightness value.
func setSystemBrightness(ctx context.Context, dut *dut.DUT, brightness int) error {
	if err := dut.Conn().CommandContext(ctx, "backlight_tool", fmt.Sprintf("--set_brightness=%d", brightness)).Run(); err != nil {
		return errors.Wrap(err, "failed to set brightness")
	}
	return nil
}
