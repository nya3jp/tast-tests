// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package intel

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"chromiumos/tast/common/shillconst"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/audio"
	"chromiumos/tast/local/audio/crastestclient"
	"chromiumos/tast/local/bluetooth/bluez"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cuj"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/mtbf/youtube"
	"chromiumos/tast/local/shill"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: BTConnectionStabilityWithWifi,
		// TODO(b/249441896): Need to add lacros variant.
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Verifies Youtube video's audio play via BT device and checks BT connection stability during WiFi download",
		Contacts:     []string{"ambalavanan.m.m@intel.com", "intel-chrome-system-automation-team@intel.com"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"intel.btHeadset", "intel.wifissid", "intel.wifipassword"},
		Fixture:      "chromeLoggedIn",
		Timeout:      20 * time.Minute,
	})
}

func BTConnectionStabilityWithWifi(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	vkb, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to create keyboard eventwriter: ", err)
	}

	btHeadset := s.RequiredVar("intel.btHeadset")
	ssid := s.RequiredVar("intel.wifissid")
	wifiPwd := s.RequiredVar("intel.wifipassword")
	manager, err := shill.NewManager(ctx)
	if err != nil {
		s.Fatal("Failed creating shill manager proxy: ", err)
	}

	if err := enableWiFi(ctx, manager); err != nil {
		s.Fatal("Failed to enable WiFi: ", err)
	}

	expectProps := map[string]interface{}{
		shillconst.ServicePropertyType:          shillconst.TypeWifi,
		shillconst.ServicePropertyName:          ssid,
		shillconst.ServicePropertySecurityClass: shillconst.SecurityPSK,
	}
	service, err := manager.FindMatchingService(ctx, expectProps)
	if err != nil {
		s.Fatal("Failed to find matching services: ", err)
	}

	if err := setWiFiProperties(ctx, manager, service, wifiPwd); err != nil {
		s.Fatal("Failed to set WiFi properties: ", err)
	}

	s.Log("Connecting AP")
	if err := service.Connect(ctx); err != nil {
		s.Fatal(err, "Failed to connect to service")
	}

	if err := wifiConnected(ctx, service); err != nil {
		s.Fatal("Failed as WiFi is disconnected: ", err)
	}
	defer service.Disconnect(cleanupCtx)

	adapters, err := bluez.Adapters(ctx)
	if err != nil {
		s.Fatal("Failed to get bluetooth adapters: ", err)
	}
	if len(adapters) == 0 {
		s.Fatal("Failed to get bluetooth adapters")
	}
	adapter := adapters[0]

	// Turn on bluetooth adapter.
	isPowered, err := adapter.Powered(ctx)
	if err != nil {
		s.Fatal("Failed to get powered property value: ", err)
	}
	if !isPowered {
		if err := adapter.SetPowered(ctx, true); err != nil {
			s.Fatal("Failed to turn on bluetooth adapter: ", err)
		}
		defer adapter.SetPowered(cleanupCtx, false)
	}

	if err := adapter.StartDiscovery(ctx); err != nil {
		s.Fatal("Failed to enable discovery: ", err)
	}

	// Waits for a specific BT device to be found.
	var btDevice *bluez.Device
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		btDevice, err = bluez.DeviceByAlias(ctx, btHeadset)
		if err != nil {
			return errors.Wrap(err, "failed to get bluetooth device by alias name")
		}
		return nil
	}, &testing.PollOptions{Timeout: 40 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Timeout waiting for BT Headset: ", err)
	}

	// Pair BT Device.
	isPaired, err := btDevice.Paired(ctx)
	if !isPaired {
		if err := btDevice.Pair(ctx); err != nil {
			s.Fatal("Failed to pair bluetooth device: ", err)
		}
	}

	if err := bluez.DisconnectAllDevices(ctx); err != nil {
		s.Fatal("Failed to disconnect the devices: ", err)
	}

	// Get connected status of BT device and connect if not already connected.
	isConnected, err := btDevice.Connected(ctx)
	if err != nil {
		s.Fatal("Failed to get BT connected status: ", err)
	}
	if !isConnected {
		if err := btDevice.Connect(ctx); err != nil {
			s.Fatal("Failed to connect bluetooth device: ", err)
		}
	}
	// Disconnect BT device.
	defer btDevice.Disconnect(cleanupCtx)

	var videoSource = youtube.VideoSrc{
		URL:     "https://www.youtube.com/watch?v=LXb3EKWsInQ",
		Title:   "COSTA RICA IN 4K 60fps HDR (ULTRA HD)",
		Quality: "1440p60",
	}

	uiHandler, err := cuj.NewClamshellActionHandler(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create clamshell action handler: ", err)
	}
	defer uiHandler.Close()

	cui := uiauto.New(tconn)
	isExternalDisplay := false
	videoApp := youtube.NewYtWeb(cr.Browser(), tconn, vkb, videoSource, isExternalDisplay, cui, uiHandler)
	if err := videoApp.OpenAndPlayVideo(ctx); err != nil {
		s.Fatalf("Failed to open %s: %v", videoSource.URL, err)
	}
	defer videoApp.Close(cleanupCtx)

	deviceName, err := checkAudioPlay(ctx)
	if err != nil {
		s.Fatal("Failed to play audio: ", err)
	}

	downloadsPath, err := cryptohome.DownloadsPath(ctx, cr.NormalizedUser())
	if err != nil {
		s.Fatal("Failed to get user's Download path: ", err)
	}

	beforeFilesInfo, err := ioutil.ReadDir(downloadsPath)
	if err != nil {
		s.Fatal("Failed to read dir: ", err)
	}

	s.Log("Downloading Ubuntu OS")
	ubuntuOSDownloadURL := "https://releases.ubuntu.com/16.04/ubuntu-16.04.6-desktop-i386.iso"
	if err := testexec.CommandContext(ctx, "wget", ubuntuOSDownloadURL, fmt.Sprintf("--directory=%s", downloadsPath)).Run(); err != nil {
		s.Fatal("Failed to download Ubuntu OS: ", err)
	}
	defer os.RemoveAll(downloadsPath)

	afterFilesInfo, err := ioutil.ReadDir(downloadsPath)
	if err != nil {
		s.Fatal("Failed to read dir: ", err)
	}

	if len(beforeFilesInfo)+1 != len(afterFilesInfo) {
		s.Fatal("Failed to get downloaded files")
	}

	// Check whether audio is routing through bluetooth during downloading file via WiFi.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			return errors.Wrap(err, "failed to detect running output device")
		}
		if deviceName != devName {
			return errors.Wrapf(err, "routing audio through unexpected audio node: got %q; want %q", devName, deviceName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		s.Fatal("Failed to check for audio routing through bluetooth after downloading file: ", err)
	}
}

// checkAudioPlay sets and verifies audio is routing through BT device.
func checkAudioPlay(ctx context.Context) (string, error) {
	cras, err := audio.NewCras(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create Cras object")
	}

	// Get current audio output device info.
	deviceName, deviceType, err := cras.SelectedOutputDevice(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get the selected audio device")
	}

	const expectedAudioNode = "BLUETOOTH"
	if deviceType != expectedAudioNode {
		if err := cras.SetActiveNodeByType(ctx, expectedAudioNode); err != nil {
			return "", errors.Wrapf(err, "failed to select active device %s", expectedAudioNode)
		}
		deviceName, deviceType, err = cras.SelectedOutputDevice(ctx)
		if err != nil {
			return "", errors.Wrap(err, "failed to get the selected audio device")
		}
		if deviceType != expectedAudioNode {
			return "", errors.Wrapf(err, "failed to set the audio node type: got %q; want %q", deviceType, expectedAudioNode)
		}
	}

	// Verify whether audio is routing through BT device or not.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		devName, err := crastestclient.FirstRunningDevice(ctx, audio.OutputStream)
		if err != nil {
			return errors.Wrap(err, "failed to detect running output device")
		}
		if deviceName != devName {
			return errors.Wrapf(err, "failed to route the audio through expected audio node: got %q; want %q", devName, deviceName)
		}
		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 250 * time.Millisecond}); err != nil {
		return "", errors.Wrap(err, "timeout waiting for BT Headset")
	}
	return deviceName, nil
}

// enableWiFi enables WiFi technology.
func enableWiFi(ctx context.Context, manager *shill.Manager) error {
	if err := manager.EnableTechnology(ctx, shill.TechnologyWifi); err != nil {
		return errors.Wrap(err, "failed to enable WiFi")
	}

	if enabled, err := manager.IsEnabled(ctx, shill.TechnologyWifi); err != nil {
		return errors.Wrap(err, "failed to get WiFi enabled state")
	} else if !enabled {
		return errors.New("failed to enable WiFi")
	}
	testing.ContextLog(ctx, "WiFi is enabled")
	return nil
}

// wifiConnected verifies whether WiFi is connected or not.
func wifiConnected(ctx context.Context, service *shill.Service) error {
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		connected, err := service.IsConnected(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get WiFi connected state")
		}
		if !connected {
			return errors.New("WiFi is disconnected")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 10 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to connect to WiFi SSID")
	}
	return nil
}

// setWiFiProperties sets WiFi with provided properties.
func setWiFiProperties(ctx context.Context, manager *shill.Manager, service *shill.Service, wifiPassword string) error {
	if err := service.SetProperty(ctx, shillconst.ServicePropertyPassphrase, wifiPassword); err != nil {
		return errors.Wrap(err, "failed to set service passphrase")
	}

	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := manager.RequestScan(ctx, shill.TechnologyWifi); err != nil {
			return errors.Wrap(err, "failed to request WiFi active scan")
		}
		return nil
	}, &testing.PollOptions{
		Timeout: 20 * time.Second,
	}); err != nil {
		return errors.Wrap(err, "failed to find the WiFi AP")
	}
	return nil
}
