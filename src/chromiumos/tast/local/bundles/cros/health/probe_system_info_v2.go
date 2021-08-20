// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"path"
	"strconv"
	"strings"

	"github.com/google/go-cmp/cmp"

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

func ProbeSystemInfoV2(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategorySystem2}
	var g systemInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &g); err != nil {
		s.Fatal("Failed to get system info v2 telemetry info: ", err)
	}
	e, err := expectedSystemInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get expected system info v2: ", err)
	}
	if d := cmp.Diff(e, g); d != "" {
		s.Fatal("SystemInfoV2 validation failed (-expected + got): ", d)
	}
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

func expectedOsVersion(ctx context.Context) (osVersion, error) {
	lsb, err := lsbrelease.Load()
	if err != nil {
		return osVersion{}, errors.Wrap(err, "failed to get lsb-release info")
	}
	return osVersion{
		ReleaseMilestone: lsb[lsbrelease.Milestone],
		BuildNumber:      lsb[lsbrelease.BuildNumber],
		PatchNumber:      lsb[lsbrelease.PatchNumber],
		ReleaseChannel:   lsb[lsbrelease.ReleaseTrack],
	}, nil
}

func expectedBootMode() (string, error) {
	v, err := utils.ReadStringFile("/proc/cmdline")
	if err != nil {
		return "", err
	}
	modeStr := map[string]bool{
		"cros_secure": true,
		"cros_efi":    true,
		"cros_legacy": true,
	}
	var r []string
	for _, s := range strings.Fields(v) {
		if modeStr[s] {
			r = append(r, s)
			modeStr[s] = false // Only add each type once.
		}
	}
	if len(r) == 0 {
		return "", errors.Errorf("BootMode is not in /proc/cmdline: %v", v)
	}
	if len(r) >= 2 {
		return "", errors.Errorf("too many BootMode in /proc/cmdline, got %v, /proc/cmdline: %v", r, v)
	}
	return r[0], nil
}

func expectedOsInfo(ctx context.Context) (osInfo, error) {
	const (
		cfgCodeName      = "/name"
		cfgMarketingName = "/arc/build-properties/marketing-name"
	)
	var r osInfo
	var err error
	if r.CodeName, err = utils.GetCrosConfig(ctx, cfgCodeName); err != nil {
		return r, err
	}
	if r.MarketingName, err = utils.GetOptionalCrosConfig(ctx, cfgMarketingName); err != nil {
		return r, err
	}
	if r.OsVersion, err = expectedOsVersion(ctx); err != nil {
		return r, err
	}
	if r.BootMode, err = expectedBootMode(); err != nil {
		return r, err
	}
	return r, nil
}

func expectedSkuNumber(ctx context.Context, fpath string) (*string, error) {
	const (
		cfgSkuNumber = "/cros-healthd/cached-vpd/has-sku-number"
	)
	c, err := utils.IsCrosConfigTrue(ctx, cfgSkuNumber)
	if err != nil {
		return nil, err
	}
	if !c {
		return nil, nil
	}
	e, err := utils.ReadStringFile(fpath)
	if err != nil {
		return nil, errors.Wrap(err, "this board must have sku_number, but failed to get")
	}
	return &e, nil
}

func expectedVpdInfo(ctx context.Context) (*vpdInfo, error) {
	const (
		ro = "/sys/firmware/vpd/ro/"
		rw = "/sys/firmware/vpd/rw/"
	)
	var r vpdInfo
	var err error
	if r.ActivateDate, err = utils.ReadOptionalStringFile(path.Join(rw, "ActivateDate")); err != nil {
		return nil, err
	}
	if r.MfgDate, err = utils.ReadOptionalStringFile(path.Join(ro, "mfg_date")); err != nil {
		return nil, err
	}
	if r.ModelName, err = utils.ReadOptionalStringFile(path.Join(ro, "model_name")); err != nil {
		return nil, err
	}
	if r.Region, err = utils.ReadOptionalStringFile(path.Join(ro, "region")); err != nil {
		return nil, err
	}
	if r.SerialNumber, err = utils.ReadOptionalStringFile(path.Join(ro, "serial_number")); err != nil {
		return nil, err
	}
	if r.SkuNumber, err = expectedSkuNumber(ctx, path.Join(ro, "sku_number")); err != nil {
		return nil, err
	}
	return &r, nil
}

func expectedChassisType(fpath string) (*jsontypes.Uint64, error) {
	v, err := utils.ReadOptionalStringFile(fpath)
	if v == nil {
		return nil, err
	}
	i, err := strconv.Atoi(*v)
	if err != nil {
		return nil, err
	}
	r := jsontypes.Uint64(i)
	return &r, nil
}

func expectedDmiInfo(ctx context.Context) (*dmiInfo, error) {
	const (
		dmi = "/sys/class/dmi/id"
	)
	var r dmiInfo
	var err error
	if r.BiosVendor, err = utils.ReadOptionalStringFile(path.Join(dmi, "bios_vendor")); err != nil {
		return nil, err
	}
	if r.BiosVersion, err = utils.ReadOptionalStringFile(path.Join(dmi, "bios_version")); err != nil {
		return nil, err
	}
	if r.BoardName, err = utils.ReadOptionalStringFile(path.Join(dmi, "board_name")); err != nil {
		return nil, err
	}
	if r.BoardVender, err = utils.ReadOptionalStringFile(path.Join(dmi, "board_vendor")); err != nil {
		return nil, err
	}
	if r.BoardVersion, err = utils.ReadOptionalStringFile(path.Join(dmi, "board_version")); err != nil {
		return nil, err
	}
	if r.ChassisVendor, err = utils.ReadOptionalStringFile(path.Join(dmi, "chassis_vendor")); err != nil {
		return nil, err
	}
	if r.ChassisType, err = expectedChassisType(path.Join(dmi, "chassis_type")); err != nil {
		return nil, err
	}
	if r.ProductFamily, err = utils.ReadOptionalStringFile(path.Join(dmi, "product_family")); err != nil {
		return nil, err
	}
	if r.ProductName, err = utils.ReadOptionalStringFile(path.Join(dmi, "product_name")); err != nil {
		return nil, err
	}
	if r.ProductVersion, err = utils.ReadOptionalStringFile(path.Join(dmi, "product_version")); err != nil {
		return nil, err
	}
	if r.SysVendor, err = utils.ReadOptionalStringFile(path.Join(dmi, "sys_vendor")); err != nil {
		return nil, err
	}
	return &r, nil
}

func expectedSystemInfo(ctx context.Context) (systemInfo, error) {
	var r systemInfo
	var err error
	if r.OsInfo, err = expectedOsInfo(ctx); err != nil {
		return r, err
	}
	if r.VpdInfo, err = expectedVpdInfo(ctx); err != nil {
		return r, err
	}
	if r.DmiInfo, err = expectedDmiInfo(ctx); err != nil {
		return r, err
	}
	return r, nil
}
