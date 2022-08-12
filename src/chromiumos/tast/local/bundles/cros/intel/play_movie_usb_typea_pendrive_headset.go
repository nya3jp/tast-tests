// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/common/usbutils"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/typecutils"
	"chromiumos/tast/testing"
)

func init() {
	// Pre-requisite: Connect Type-A USB 3.0 pendrive and Type-A Headset to the DUT.
	testing.AddTest(&testing.Test{
		Func:         PlayMovieUsbTypeaPendriveHeadset,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies Play movie in USB type-A pen drive with USB type-A HS",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		Data:         []string{"bear-320x240.h264.mp4"},
		Vars:         []string{"intel.usbDetectionName"},
		Fixture:      "chromeLoggedIn",
		Timeout:      7 * time.Minute,
	})
}

func PlayMovieUsbTypeaPendriveHeadset(ctx context.Context, s *testing.State) {
	// Give 5 seconds to cleanup other resources.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Verify USB pendrive speed.
	usbDevicesList, err := usbutils.ListDevicesInfo(ctx, nil)
	if err != nil {
		s.Fatal("Failed to get USB devices list: ", err)
	}
	usbDeviceClassName := "Mass Storage"
	usbSpeed := "5000M"
	got := usbutils.NumberOfUSBDevicesConnected(usbDevicesList, usbDeviceClassName, usbSpeed)
	if want := 1; got != want {
		s.Fatalf("Unexpected number of USB devices connected: got %d, want %d", got, want)
	}

	usbDeviceName := s.RequiredVar("intel.usbDetectionName")
	const (
		mediaRemovable = "/media/removable/"
		videoFileName  = "bear-320x240.h264.mp4"
	)
	destinationFilePath := path.Join(mediaRemovable, usbDeviceName, videoFileName)

	if copyErr := testexec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("cp -rf %s %s", s.DataPath(videoFileName), destinationFilePath)).Run(); copyErr != nil {
		s.Fatalf("Failed to copy file to %s path", destinationFilePath)
	}
	defer os.Remove(destinationFilePath)

	files, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch the Files App: ", err)
	}
	defer files.Close(cleanupCtx)

	if err := files.OpenDir(usbDeviceName, filesapp.FilesTitlePrefix+usbDeviceName)(ctx); err != nil {
		s.Fatal("Failed to open USB directory: ", err)
	}

	if err := files.OpenFile(videoFileName)(ctx); err != nil {
		s.Fatalf("Failed to open the audio file %q: %v", videoFileName, err)
	}
	cui := uiauto.New(tconn)
	togglePlayPause := nodewith.Name("Toggle play pause").Role(role.Button)
	if err := cui.LeftClick(togglePlayPause)(ctx); err != nil {
		s.Fatal("Failed to find and click togglePlayPause button: ", err)
	}

	cras, err := audio.NewCras(ctx)
	if err != nil {
		s.Fatal("Failed to create Cras object: ", err)
	}

	// Get current audio output device info.
	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		s.Fatal("Failed to get the selected audio device: ", err)
	}

	const expectedAudioOutputNode = "USB"
	if deviceType != expectedAudioOutputNode {
		if err := cras.SetActiveNodeByType(ctx, expectedAudioOutputNode); err != nil {
			s.Fatalf("Failed to select active device %s: %v", expectedAudioOutputNode, err)
		}
		deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
		if err != nil {
			s.Fatal("Failed to get the selected audio device: ", err)
		}
		if deviceType != expectedAudioOutputNode {
			s.Fatalf("Failed to set the audio node type: got %q; want %q", deviceType, expectedAudioOutputNode)
		}
	}

	out, err := testexec.CommandContext(ctx, "cras_test_client").Output()
	if err != nil {
		s.Fatal("Failed to exceute cras_test_client command: ", err)
	}
	re := regexp.MustCompile(`yes.*USB.*2\*`)
	if !re.MatchString(string(out)) {
		s.Fatal("Failed to select USB as output audio node")
	}

	if err := typecutils.VerifyAudioRoute(ctx, deviceName); err != nil {
		s.Fatalf("Failed to verify audio routing through %q: %v", expectedAudioOutputNode, err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard eventwriter: ", err)
	}

	var increaseSliderValue, decreasedSliderValue int

	sliderValue, err := quicksettings.SliderValue(ctx, tconn, quicksettings.SliderTypeVolume)
	if err != nil {
		s.Fatal("Failed to get slider value: ", err)
	}
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		decreasedSliderValue, err = quicksettings.DecreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeVolume)
		if err != nil {
			return errors.Wrap(err, "failed to DecreaseSlider")
		}
		if err := quicksettings.Collapse(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to press escape key")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to decrease slider: ", err)
	}
	if sliderValue <= decreasedSliderValue {
		s.Fatalf("Failed to decrease volume slider: got %d want lesser than %d", decreasedSliderValue, sliderValue)
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		increaseSliderValue, err = quicksettings.IncreaseSlider(ctx, tconn, kb, quicksettings.SliderTypeVolume)
		if err != nil {
			return errors.Wrap(err, "failed to IncreaseSlider")

		}
		if err := quicksettings.Collapse(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to press escape key")
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		s.Fatal("Failed to increase slider: ", err)

	}
	if decreasedSliderValue >= increaseSliderValue {
		s.Fatalf("Failed to increase volume slider: got %d want greater than %d", increaseSliderValue, decreasedSliderValue)
	}

}
