// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"fmt"
	"path"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/health/utils"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
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

func (info *osVersion) validate(ctx context.Context) error {
	lsb, err := lsbrelease.Load()
	if err != nil {
		return errors.Wrap(err, "failed to get lsb-release info")
	}
	if err := utils.CompareString(lsb[lsbrelease.Milestone], info.ReleaseMilestone); err != nil {
		return errors.Wrap(err, "ReleaseMilestone")
	}
	if err := utils.CompareString(lsb[lsbrelease.BuildNumber], info.BuildNumber); err != nil {
		return errors.Wrap(err, "BuildNumber")
	}
	if err := utils.CompareString(lsb[lsbrelease.PatchNumber], info.PatchNumber); err != nil {
		return errors.Wrap(err, "PatchNumber")
	}
	if err := utils.CompareString(lsb[lsbrelease.ReleaseTrack], info.ReleaseChannel); err != nil {
		return errors.Wrap(err, "ReleaseChannel")
	}
	return nil
}

func validateBootMode(got string) error {
	v, err := utils.ReadFile("/proc/cmdline")
	if err != nil {
		return err
	}
	if got == "Unknown" || v == nil || !strings.Contains(*v, got) {
		return errors.Wrapf(err, "BootMode is not in /proc/cmdline, got: %v, /proc/cmdline: %v", got, utils.PtrToStr(v))
	}
	return nil
}

func (info *osInfo) validate(ctx context.Context) error {
	const (
		cfgCodeName      = "/name"
		cfgMarketingName = "/arc/build-properties/marketing-name"
	)
	if err := utils.CompareStringPtrWithCrosConfig(ctx, cfgCodeName, &info.CodeName); err != nil {
		return errors.Wrap(err, "CodeName")
	}
	if err := utils.CompareStringPtrWithCrosConfig(ctx, cfgMarketingName, info.MarketingName); err != nil {
		return errors.Wrap(err, "MarketingName")
	}
	if err := info.OsVersion.validate(ctx); err != nil {
		return err
	}
	if err := validateBootMode(info.BootMode); err != nil {
		return err
	}
	return nil
}

func compareSkuNumber(ctx context.Context, fpath string, got *string) error {
	const (
		cfgSkuNumber = "/cros-healthd/cached-vpd/has-sku-number"
	)
	c, err := utils.IsCrosConfigTrue(ctx, cfgSkuNumber)
	if err != nil {
		return err
	}
	if !c {
		return utils.CompareStringPtr(nil, got)
	}
	e, err := utils.ReadFile(fpath)
	if e == nil || err != nil {
		return errors.Wrap(err, "this board must have sku_number")
	}
	return utils.CompareStringPtr(e, got)
}

func (info *vpdInfo) validate(ctx context.Context) error {
	const (
		ro = "/sys/firmware/vpd/ro/"
		rw = "/sys/firmware/vpd/rw/"
	)
	if err := utils.CompareStringPtrWithFile(path.Join(rw, "ActivateDate"), info.ActivateDate); err != nil {
		return errors.Wrap(err, "ActivateDate")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(ro, "mfg_date"), info.MfgDate); err != nil {
		return errors.Wrap(err, "MfgDate")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(ro, "model_name"), info.ModelName); err != nil {
		return errors.Wrap(err, "ModelName")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(ro, "region"), info.Region); err != nil {
		return errors.Wrap(err, "Region")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(ro, "serial_number"), info.SerialNumber); err != nil {
		return errors.Wrap(err, "SerialNumber")
	}
	if err := compareSkuNumber(ctx, path.Join(ro, "sku_number"), info.SkuNumber); err != nil {
		return errors.Wrap(err, "SkuNumber")
	}
	return nil
}

func getChassisTypeStrPtr(v *jsontypes.Uint64) *string {
	if v == nil {
		return nil
	}
	s := fmt.Sprintf("%d", *v)
	return &s
}

func (info *dmiInfo) validate(ctx context.Context) error {
	const (
		dmi = "/sys/class/dmi/id"
	)
	if err := utils.CompareStringPtrWithFile(path.Join(dmi, "bios_vendor"), info.BiosVendor); err != nil {
		return errors.Wrap(err, "BiosVendor")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(dmi, "bios_version"), info.BiosVersion); err != nil {
		return errors.Wrap(err, "BiosVersion")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(dmi, "board_name"), info.BoardName); err != nil {
		return errors.Wrap(err, "BoardName")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(dmi, "board_vendor"), info.BoardVender); err != nil {
		return errors.Wrap(err, "BoardVender")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(dmi, "board_version"), info.BoardVersion); err != nil {
		return errors.Wrap(err, "BoardVersion")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(dmi, "chassis_vendor"), info.ChassisVendor); err != nil {
		return errors.Wrap(err, "ChassisVendor")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(dmi, "chassis_type"), getChassisTypeStrPtr(info.ChassisType)); err != nil {
		return errors.Wrap(err, "ChassisType")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(dmi, "product_family"), info.ProductFamily); err != nil {
		return errors.Wrap(err, "ProductFamily")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(dmi, "product_name"), info.ProductName); err != nil {
		return errors.Wrap(err, "ProductName")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(dmi, "product_version"), info.ProductVersion); err != nil {
		return errors.Wrap(err, "ProductVersion")
	}
	if err := utils.CompareStringPtrWithFile(path.Join(dmi, "sys_vendor"), info.SysVendor); err != nil {
		return errors.Wrap(err, "SysVendor")
	}
	return nil
}

func (info *systemInfo) validate(ctx context.Context) error {
	if err := info.OsInfo.validate(ctx); err != nil {
		return err
	}
	if err := info.VpdInfo.validate(ctx); err != nil {
		return err
	}
	if err := info.DmiInfo.validate(ctx); err != nil {
		return err
	}
	return nil
}

func ProbeSystemInfoV2(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategorySystem2}
	var info systemInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		s.Fatal("Failed to get system info v2 telemetry info: ", err)
	}
	if err := info.validate(ctx); err != nil {
		s.Fatal("Validation failed: ", err)
	}
}
