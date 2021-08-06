// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/health/utils"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ProbeSystemInfoV2,
		Desc: "Check that we can probe cros_healthd for system info",
		Contacts: []string{
			"cros-tdm-tpe-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

type osVersion struct {
	ReleaseMilestone string `json:"release_milestone"`
	BuildNumber      string `json:"build_number"`
	PatchNumber      string `json:"patch_number"`
	ReleaseChannel   string `json:"release_channel"`
}

type osInfo struct {
	CodeName      string    `json:"code_name"`
	MarketingName *string   `json:"marketing_name"`
	OsVersion     osVersion `json:"os_version"`
	BootMode      string    `json:"boot_mode"`
}

type vpdInfo struct {
	SerialNumber *string `json:"serial_number"`
	Region       *string `json:"region"`
	MfgDate      *string `json:"mfg_date"`
	ActivateDate *string `json:"activate_date"`
	SkuNumber    *string `json:"sku_number"`
	ModelName    *string `json:"model_name"`
}

type dmiInfo struct {
	BiosVendor     *string           `json:"bios_vendor"`
	BiosVersion    *string           `json:"bios_version"`
	BoardName      *string           `json:"board_name"`
	BoardVender    *string           `json:"board_vendor"`
	BoardVersion   *string           `json:"board_version"`
	ChassisVendor  *string           `json:"chassis_vendor"`
	ChassisType    *jsontypes.Uint64 `json:"chassis_type"`
	ProductFamily  *string           `json:"product_family"`
	ProductName    *string           `json:"product_name"`
	ProductVersion *string           `json:"product_version"`
	SysVendor      *string           `json:"sys_vendor"`
}

type systemInfo struct {
	OsInfo  osInfo   `json:"os_info"`
	VpdInfo *vpdInfo `json:"vpd_info"`
	DmiInfo *dmiInfo `json:"dmi_info"`
}

func (info *osVersion) expected(ctx context.Context, errOut *error) osVersion {
	lsb, err := lsbrelease.Load()
	if err != nil {
		err = errors.Wrap(err, "failed to get lsb-release info")
		errOut = &err
		return osVersion{}
	}
	return osVersion{
		ReleaseMilestone: lsb[lsbrelease.Milestone],
		BuildNumber:      lsb[lsbrelease.BuildNumber],
		PatchNumber:      lsb[lsbrelease.PatchNumber],
		ReleaseChannel:   lsb[lsbrelease.ReleaseTrack],
	}
}

func expectedBootMode(got string, errOut *error) string {
	var err error
	v := utils.ReadFile("/proc/cmdline", &err)
	if err != nil {
		errOut = &err
		return ""
	}
	if got == "Unknown" || v == nil || !strings.Contains(*v, got) {
		err = errors.Errorf("BootMode is not in /proc/cmdline, got: %v, /proc/cmdline: %v", got, utils.PtrToElem(v))
		errOut = &err
		return ""
	}
	return got
}

func (info *osInfo) expected(ctx context.Context, errOut *error) osInfo {
	const (
		cfgCodeName      = "/name"
		cfgMarketingName = "/arc/build-properties/marketing-name"
	)
	var err error
	cn := utils.GetCrosConfig(ctx, cfgCodeName, &err)
	if cn == nil {
		err = errors.Wrap(err, "CodeName is required field but cannot get it from cros config")
		errOut = &err
		return osInfo{}
	}
	return osInfo{
		CodeName:      *cn,
		MarketingName: utils.GetCrosConfig(ctx, cfgMarketingName, errOut),
		OsVersion:     info.OsVersion.expected(ctx, errOut),
		BootMode:      expectedBootMode(info.BootMode, errOut),
	}
}

func (info *systemInfo) expected(ctx context.Context, errOut *error) systemInfo {
	return systemInfo{
		OsInfo:  info.OsInfo.expected(ctx, errOut),
		VpdInfo: info.VpdInfo,
		DmiInfo: info.DmiInfo,
	}
}

func ProbeSystemInfoV2(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategorySystem2}
	var info systemInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		s.Fatal("Failed to get system info v2 telemetry info: ", err)
	}
	var err error
	e := info.expected(ctx, &err)
	if err != nil {
		s.Fatal("Validation failed: ", err)
	}
	if err := utils.Compare(e, info); err != nil {
		s.Fatal("Validation failed: ", err)
	}
}
