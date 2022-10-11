// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hps

import (
	"context"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/golang/protobuf/ptypes/empty"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	"google.golang.org/grpc"
	durationpb "google.golang.org/protobuf/types/known/durationpb"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/dbusutil"
	pb "chromiumos/tast/services/cros/hps"
	"chromiumos/tast/testing"
)

const (
	powerdLog = "/var/log/power_manager/powerd.LATEST"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterHpsServiceServer(srv, &SettingService{s: s})
		},
	})
}

type SettingService struct {
	s  *testing.ServiceState
	cr *chrome.Chrome
}

// WaitForDbus wait for the hpsd to finish flashing.
func (hss *SettingService) WaitForDbus(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	dbusName := "org.chromium.Hps"
	dbusPath := "/org/chromium/Hps"

	_, _, err := dbusutil.Connect(ctx, dbusName, dbus.ObjectPath(dbusPath))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to %s", dbusName)
	}
	return &empty.Empty{}, nil
}

// StartUIWithCustomScreenPrivacySetting changes the settings under the screen privacy page depending on the needs.
func (hss *SettingService) StartUIWithCustomScreenPrivacySetting(ctx context.Context, req *pb.StartUIWithCustomScreenPrivacySettingRequest) (*empty.Empty, error) {
	// If user is logged in already, then no need to start again within the same test.
	if hss.cr == nil {
		cr, err := chrome.New(ctx, chrome.ExtraArgs("--enable-features=QuickDim,SnoopingProtection"))
		hss.cr = cr
		if err != nil {
			return nil, errors.Wrap(err, "failed to start UI")
		}
	}
	tconn, err := hss.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Test API connection")
	}

	app := apps.Settings
	if err := apps.Launch(ctx, tconn, app.ID); err != nil {
		return nil, errors.Wrapf(err, "failed to launch %s", app.Name)
	}

	ui := uiauto.New(tconn)

	const subsettingsName = "Screen privacy"
	settings, err := ossettings.LaunchAtPageURL(ctx, tconn, hss.cr, "osPrivacy", ui.WaitUntilExists(nodewith.Name(subsettingsName)))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the settings page")
	}
	if err := ui.LeftClick(nodewith.Name(subsettingsName))(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to open ScreenPrivacy settings")
	}

	if err := ui.WaitUntilExists(nodewith.Name(req.Setting).Role(role.ToggleButton))(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to wait for the toggle button to show up")
	}

	isEnabled, err := settings.IsToggleOptionEnabled(ctx, hss.cr, req.Setting)
	if err != nil {
		return nil, errors.Wrap(err, "could not check the status of the toggle button")
	}
	if isEnabled != req.Enable {
		oldSettings, err := readPowerdLastUpdatedSettings()
		if err != nil {
			return nil, errors.Wrap(err, "failed to read powerd settings")
		}

		if err := ui.LeftClick(nodewith.Name(req.Setting).Role(role.ToggleButton))(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to click on the button")
		}

		testing.ContextLog(ctx, "Waiting for new settings in powerd.LATEST")
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			settings, err := readPowerdLastUpdatedSettings()
			if err != nil {
				return err
			}
			if oldSettings != settings {
				return nil
			}
			return errors.New("powerd settings not changed")
		}, &testing.PollOptions{
			Interval: time.Second,
			Timeout:  time.Minute,
		}); err != nil {
			return nil, errors.Wrap(err, "error during polling for updated powerd setting")
		}
	}
	return &empty.Empty{}, nil
}

// CheckForLockScreen checks if the screen is at lock status.
func (hss *SettingService) CheckForLockScreen(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	tconn, err := hss.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error getting test conn")
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, time.Minute); err != nil {
		return nil, errors.Wrapf(err, "waiting for screen to be locked failed (last status %+v)", st)
	}
	return &empty.Empty{}, nil
}

// OpenHPSInternalsPage opens hps-internal page for debugging purpose.
func (hss *SettingService) OpenHPSInternalsPage(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if _, err := hss.cr.NewConn(ctx, "chrome://hps-internals"); err != nil {
		return nil, errors.Wrap(err, "error rendring hps-internals")
	}
	return &empty.Empty{}, nil
}

// CheckSPAEyeIcon checks if the eye icon is at the right bottom side of the screen when there is spa alert.
func (hss *SettingService) CheckSPAEyeIcon(ctx context.Context, req *empty.Empty) (*wrappers.BoolValue, error) {
	tconn, err := hss.cr.TestAPIConn(ctx)

	if err != nil {
		return nil, errors.Wrap(err, "error getting test conn")
	}
	ui := uiauto.New(tconn)
	hpsNotifyView := nodewith.ClassName("SnoopingProtectionView")

	if err := ui.Exists(hpsNotifyView)(ctx); err != nil {
		if strings.Contains(err.Error(), "failed to find node with properties") {
			return &wrappers.BoolValue{Value: false}, nil
		}
		return nil, err
	}
	return &wrappers.BoolValue{Value: true}, nil
}

// RetrieveDimMetrics gets the quick dim/lock delays after the lol is enabled/disabled.
func (hss *SettingService) RetrieveDimMetrics(ctx context.Context, quickDimEnabled *wrappers.BoolValue) (*pb.RetrieveDimMetricsResponse, error) {
	dat, err := os.ReadFile(powerdLog)
	if err != nil {
		return nil, err
	}
	response, settings, err := processContent(dat, quickDimEnabled.Value)
	testing.ContextLog(ctx, "settings: ", settings)
	return response, err
}

// RetrieveHpsSenseSignal gets current HpsSenseSignal from powerd.
func (hss *SettingService) RetrieveHpsSenseSignal(ctx context.Context, req *empty.Empty) (*pb.HpsSenseSignalResponse, error) {
	dat, err := os.ReadFile(powerdLog)
	if err != nil {
		return nil, err
	}
	rawValue, err := powerdLastHpsSenseSignal(dat)
	if err != nil {
		return nil, err
	}
	response := &pb.HpsSenseSignalResponse{
		RawValue: rawValue,
	}
	return response, nil
}

func readPowerdLastUpdatedSettings() (string, error) {
	dat, err := os.ReadFile(powerdLog)
	if err != nil {
		return "", err
	}
	return powerdLastUpdatedSettings(dat), nil
}

func powerdLastUpdatedSettings(dat []byte) string {
	settingRegex := regexp.MustCompile(`Updated settings:(.+)`)
	results := settingRegex.FindAll(dat, -1)

	if len(results) < 1 {
		return ""
	}
	return string(results[len(results)-1])
}

func powerdLastHpsSenseSignal(dat []byte) (string, error) {
	r := regexp.MustCompile(`HandleHpsSenseSignal is called with value (.+)`)
	results := r.FindAll(dat, -1)

	if len(results) < 1 {
		return "", errors.New("no HandleHpsSenseSignal found in powerd logs")
	}
	lastResult := results[len(results)-1]
	sub := r.FindStringSubmatch(string(lastResult))
	if len(sub) < 2 {
		return "", errors.Errorf("%v doesn't have enough submatches", lastResult)
	}
	return sub[1], nil
}

func processContent(dat []byte, quickDimEnable bool) (*pb.RetrieveDimMetricsResponse, string, error) {
	var dimKey, screenOffKey string
	var dimTime, screenOffTime, lockTime time.Duration

	// http://cs/chromeos_public/src/platform2/system_api/dbus/power_manager/policy.proto
	// describes the Delays message.
	if quickDimEnable {
		dimKey = "quick_dim"
		screenOffKey = "quick_lock"
	} else {
		dimKey = "dim"
		screenOffKey = "screen_off"
	}
	newestSettings := []byte(powerdLastUpdatedSettings(dat))
	if len(newestSettings) == 0 {
		return nil, "", errors.New("no existing settings yet")
	}
	dimTime, err := singleMetric(newestSettings, dimKey)
	if err != nil {
		return nil, "", errors.Wrap(err, "error getting dim delay")
	}
	screenOffTime, err = singleMetric(newestSettings, screenOffKey)
	if err != nil {
		return nil, "", errors.Wrap(err, "error getting screenoff delay")
	}
	if lockTime = screenOffTime + time.Minute; quickDimEnable {
		lockTime = screenOffTime
	}

	response := &pb.RetrieveDimMetricsResponse{
		DimDelay:       durationpb.New(dimTime),
		ScreenOffDelay: durationpb.New(screenOffTime - dimTime),
		LockDelay:      durationpb.New(lockTime - screenOffTime),
	}
	return response, string(newestSettings), nil
}

func singleMetric(settings []byte, key string) (time.Duration, error) {
	reg := regexp.QuoteMeta(key) + `=([0-9][0-9]?[s,m][0-9]?[0-9]?[s,m]?)`
	settingReg := regexp.MustCompile(reg)
	result := string(settingReg.Find(settings))
	delay, err := time.ParseDuration(strings.Replace(result, key+"=", "", -1))
	if err != nil {
		return -1, err
	}
	return delay, nil
}
