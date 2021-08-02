// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"strings"

	"chromiumos/tast/local/bundles/cros/health/utils"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/testing"
)

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
	BiosVendor     *string `json:"bios_vendor"`
	BiosVersion    *string `json:"bios_version"`
	BoardName      *string `json:"board_name"`
	BoardVender    *string `json:"board_vendor"`
	BoardVersion   *string `json:"board_version"`
	ChassisVendor  *string `json:"chassis_vendor"`
	ChassisType    *string `json:"chassis_type"`
	ProductFamily  *string `json:"product_family"`
	ProductName    *string `json:"product_name"`
	ProductVersion *string `json:"product_version"`
	SysVendor      *string `json:"sys_vendor"`
}

type systemInfo struct {
	OsInfo  osInfo   `json:"os_info"`
	VpdInfo *vpdInfo `json:"vpd_info"`
	DmiInfo *dmiInfo `json:"dmi_info"`
}

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

func (info *osVersion) validate(ctx context.Context, s *testing.State) {
	lsb, err := lsbrelease.Load()
	if err != nil {
		s.Fatal("Failed to get lsb-release info: ", err)
	}
	if err := utils.CompareString(lsb[lsbrelease.Milestone], info.ReleaseMilestone); err != nil {
		s.Error("ReleaseMilestone: ", err)
	}
	if err := utils.CompareString(lsb[lsbrelease.BuildNumber], info.BuildNumber); err != nil {
		s.Error("BuildNumber: ", err)
	}
	if err := utils.CompareString(lsb[lsbrelease.PatchNumber], info.PatchNumber); err != nil {
		s.Error("PatchNumber: ", err)
	}
	if err := utils.CompareString(lsb[lsbrelease.ReleaseTrack], info.ReleaseChannel); err != nil {
		s.Error("ReleaseChannel: ", err)
	}
}

func (info *osInfo) validate(ctx context.Context, s *testing.State) {
	const (
		cfgCodeName      = "/name"
		cfgMarketingName = "/arc/build-properties/marketing-name"
	)
	v, err := utils.GetCrosConfig(ctx, cfgCodeName)
	if err != nil {
		s.Fatal("Failed to get cros config: ", err)
	}
	if err := utils.CompareStringPtr(v, &info.CodeName); err != nil {
		s.Error("CodeName: ", err)
	}

	v, err = utils.GetCrosConfig(ctx, cfgMarketingName)
	if err != nil {
		s.Fatal("Failed to get cros config: ", err)
	}
	if err := utils.CompareStringPtr(v, info.MarketingName); err != nil {
		s.Error("MarketingName: ", err)
	}

	info.OsVersion.validate(ctx, s)

	v, err = utils.ReadFile("/proc/cmdline")
	if err != nil {
		s.Fatal("Failed to read file: ", err)
	}
	if info.BootMode == "Unknown" || v == nil || !strings.Contains(*v, info.BootMode) {
		s.Errorf("BootMode not in /proc/cmdline, got: %v, /proc/cmdline: %v", info.BootMode, utils.PtrToStr(v))
	}
}

func (info *systemInfo) validate(ctx context.Context, s *testing.State) {
	info.OsInfo.validate(ctx, s)
}

func ProbeSystemInfoV2(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategorySystem2}
	var info systemInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		s.Fatal("Failed to get storage telemetry info: ", err)
	}
	info.validate(ctx, s)
}
