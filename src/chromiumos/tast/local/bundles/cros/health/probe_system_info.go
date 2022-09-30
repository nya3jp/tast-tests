// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"os"
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
		Func:         ProbeSystemInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for system info",
		Contacts:     []string{"cros-tdm-tpe-eng@google.com"},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics"},
		Fixture:      "crosHealthdRunning",
	})
}

func ProbeSystemInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategorySystem}
	var g systemInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &g); err != nil {
		s.Fatal("Failed to get system info telemetry info: ", err)
	}
	e, err := expectedSystemInfo(ctx)
	if err != nil {
		s.Fatal("Failed to get expected system info: ", err)
	}
	if d := cmp.Diff(e, g); d != "" {
		s.Fatal("SystemInfo validation failed (-expected + got): ", d)
	}
}

type osVersion struct {
	ReleaseMilestone string `json:"release_milestone"`
	BuildNumber      string `json:"build_number"`
	BranchNumber     string `json:"branch_number"`
	PatchNumber      string `json:"patch_number"`
	ReleaseChannel   string `json:"release_channel"`
}

type osInfo struct {
	CodeName        string    `json:"code_name"`
	MarketingName   *string   `json:"marketing_name"`
	OSVersion       osVersion `json:"os_version"`
	BootMode        string    `json:"boot_mode"`
	OEMName         *string   `json:"oem_name"`
	EfiPlatformSize string    `json:"efi_platform_size"`
}

type vpdInfo struct {
	SerialNumber *string `json:"serial_number"`
	Region       *string `json:"region"`
	MFGDate      *string `json:"mfg_date"`
	ActivateDate *string `json:"activate_date"`
	SKUNumber    *string `json:"sku_number"`
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
	OSInfo  osInfo   `json:"os_info"`
	VPDInfo *vpdInfo `json:"vpd_info"`
	DMIInfo *dmiInfo `json:"dmi_info"`
}

func expectedOSVersion(ctx context.Context) (osVersion, error) {
	lsb, err := lsbrelease.Load()
	if err != nil {
		return osVersion{}, errors.Wrap(err, "failed to get lsb-release info")
	}
	return osVersion{
		ReleaseMilestone: lsb[lsbrelease.Milestone],
		BuildNumber:      lsb[lsbrelease.BuildNumber],
		BranchNumber:     lsb[lsbrelease.BranchNumber],
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

func expectedOSInfo(ctx context.Context) (osInfo, error) {
	const (
		cfgCodeName      = "/name"
		cfgMarketingName = "/branding/marketing-name"
		cfgOEMName       = "/branding/oem-name"
	)
	var r osInfo
	var err error
	if r.CodeName, err = utils.GetCrosConfig(ctx, cfgCodeName); err != nil {
		return r, err
	}
	if r.MarketingName, err = utils.GetOptionalCrosConfig(ctx, cfgMarketingName); err != nil {
		return r, err
	}
	if r.OEMName, err = utils.GetOptionalCrosConfig(ctx, cfgOEMName); err != nil {
		return r, err
	}
	if r.OSVersion, err = expectedOSVersion(ctx); err != nil {
		return r, err
	}
	if r.BootMode, err = expectedBootMode(); err != nil {
		return r, err
	}
	// Before we have DUT boot with efi, we can assume that the result should be
	// "unknown".
	r.EfiPlatformSize = "unknown"
	return r, nil
}

// expectedSKUNumber return a string pointer here for expected |SKUNumber|.
// Since healthd uses json package to parse json result, the null field becomes
// nil string in go. We return string pointer to simplify the comparison.
func expectedSKUNumber(ctx context.Context, filePath string) (*string, error) {
	const cfgSKUNumber = "/cros-healthd/cached-vpd/has-sku-number"

	c, err := utils.IsCrosConfigTrue(ctx, cfgSKUNumber)
	if err != nil {
		return nil, err
	}
	if !c {
		return nil, nil
	}
	e, err := utils.ReadStringFile(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "this board must have sku_number, but failed to get")
	}
	return &e, nil
}

func expectedVPDInfo(ctx context.Context) (*vpdInfo, error) {
	const (
		vpd = "/sys/firmware/vpd"
		ro  = "/sys/firmware/vpd/ro/"
		rw  = "/sys/firmware/vpd/rw/"
	)
	if _, err := os.Stat(vpd); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var r vpdInfo
	var err error
	if r.ActivateDate, err = utils.ReadOptionalStringFile(path.Join(rw, "ActivateDate")); err != nil {
		return nil, err
	}
	if r.MFGDate, err = utils.ReadOptionalStringFile(path.Join(ro, "mfg_date")); err != nil {
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
	if r.SKUNumber, err = expectedSKUNumber(ctx, path.Join(ro, "sku_number")); err != nil {
		return nil, err
	}
	return &r, nil
}

func expectedChassisType(filePath string) (*jsontypes.Uint64, error) {
	v, err := utils.ReadOptionalStringFile(filePath)
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

func expectedDMIInfo(ctx context.Context) (*dmiInfo, error) {
	const dmi = "/sys/class/dmi/id"

	if _, err := os.Stat(dmi); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
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
	if r.OSInfo, err = expectedOSInfo(ctx); err != nil {
		return r, err
	}
	if r.VPDInfo, err = expectedVPDInfo(ctx); err != nil {
		return r, err
	}
	if r.DMIInfo, err = expectedDMIInfo(ctx); err != nil {
		return r, err
	}
	return r, nil
}
