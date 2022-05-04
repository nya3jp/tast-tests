// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"unsafe"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/croshealthd"
	"chromiumos/tast/local/jsontypes"
	"chromiumos/tast/testing"
)

type temperatureChannelInfo struct {
	Label              *string `json:"label"`
	TemperatureCelsius int32   `json:"temperature_celsius"`
}

type cStateInfo struct {
	Name                       string           `json:"name"`
	TimeInStateSinceLastBootUs jsontypes.Uint64 `json:"time_in_state_since_last_boot_us"`
}

type logicalCPUInfo struct {
	UserTimeUserHz             jsontypes.Uint64 `json:"user_time_user_hz"`
	SystemTimeUserHz           jsontypes.Uint64 `json:"system_time_user_hz"`
	MaxClockSpeedKhz           jsontypes.Uint32 `json:"max_clock_speed_khz"`
	ScalingMaxFrequencyKhz     jsontypes.Uint32 `json:"scaling_max_frequency_khz"`
	ScalingCurrentFrequencyKhz jsontypes.Uint32 `json:"scaling_current_frequency_khz"`
	IdleTimeUserHz             jsontypes.Uint64 `json:"idle_time_user_hz"`
	CStates                    []cStateInfo     `json:"c_states"`
}

type cpuVirtualizationInfo struct {
	Type      string `json:"type"`
	IsEnabled bool   `json:"is_enabled"`
	IsLocked  bool   `json:"is_locked"`
}

type physicalCPUInfo struct {
	ModelName         *string                `json:"model_name"`
	LogicalCPUs       []logicalCPUInfo       `json:"logical_cpus"`
	Flags             []string               `json:"flags"`
	CPUVirtualization *cpuVirtualizationInfo `json:"cpu_virtualization"`
}

type keylockerinfo struct {
	KeylockerConfigured bool `json:"keylocker_configured"`
}

type virtualizationInfo struct {
	HasKvmDevice bool   `json:"has_kvm_device"`
	IsSmtActive  bool   `json:"is_smt_active"`
	SmtControl   string `json:"smt_control"`
}

type vulnerabilityInfo struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type cpuInfo struct {
	Architecture        string                       `json:"architecture"`
	NumTotalThreads     jsontypes.Uint32             `json:"num_total_threads"`
	TemperatureChannels []temperatureChannelInfo     `json:"temperature_channels"`
	PhysicalCPUs        []physicalCPUInfo            `json:"physical_cpus"`
	KeylockerInfo       *keylockerinfo               `json:"keylocker_info"`
	Virtualization      virtualizationInfo           `json:"virtualization"`
	Vulnerabilities     map[string]vulnerabilityInfo `json:"vulnerabilities"`
}

type cpuInfoTestParams struct {
	// Whether to check vulnerabilities.
	checkVulnerability bool
	// Whether to check virtualization.
	checkVirtualization bool
	// Whether to check cpu virtualization.
	checkCPUVirtualization bool
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ProbeCPUInfo,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Check that we can probe cros_healthd for CPU info",
		Contacts: []string{
			"cros-tdm-tpe-eng@google.com",
			"pathan.jilani@intel.com",
			"intel-chrome-system-automation-team@intel.com",
		},
		Attr: []string{"group:mainline"},
		SoftwareDeps: []string{"chrome", "diagnostics",
			// TODO(b/210950844): Reenable after plumbing through cpu frequency info.
			"no_manatee"},
		Params: []testing.Param{{
			Fixture: "crosHealthdRunning",
			Val: cpuInfoTestParams{
				checkVulnerability:     false,
				checkVirtualization:    false,
				checkCPUVirtualization: false,
			},
		}, {
			Name:      "vulnerability",
			ExtraAttr: []string{"informational"},
			Val: cpuInfoTestParams{
				checkVulnerability:     true,
				checkVirtualization:    false,
				checkCPUVirtualization: false,
			},
		}, {
			Name:      "virtualization",
			ExtraAttr: []string{"informational"},
			Val: cpuInfoTestParams{
				checkVulnerability:     false,
				checkVirtualization:    true,
				checkCPUVirtualization: false,
			},
		}, {
			Name:      "cpu_virtualization",
			ExtraAttr: []string{"informational"},
			Val: cpuInfoTestParams{
				checkVulnerability:     false,
				checkVirtualization:    false,
				checkCPUVirtualization: true,
			},
		}},
	})
}

func readMsr(msrReg int64) (uint64, error) {
	// msr read/write always comes at 8 bytes: https://man7.org/linux/man-pages/man4/msr.4.html
	const msrSize = 8
	bytes := make([]byte, msrSize)
	msrFile, err := os.Open(fmt.Sprintf("/dev/cpu/0/msr"))
	if err != nil {
		return 0, errors.New("could not open msr file")
	}
	msrFile.Seek(msrReg, 0)
	readSize, err := msrFile.Read(bytes)
	if err != nil || readSize != msrSize {
		return 0, errors.New("could not read msr file")
	}

	// To align with ReadMsr tool implementation, return a uint64
	return *(*uint64)(unsafe.Pointer(&bytes[0])), nil
}

func getFlags() (map[string]bool, error) {
	cpuinfoFile, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return nil, errors.New("could not read /proc/cpuinfo")
	}
	defer cpuinfoFile.Close()

	// We assume all CPUs have the same CPU flag, as no chromebook possess multiple
	// physical CPUs and even those with multiple physical CPUs usually have the
	// same CPU model and same CPU flags enabled.

	flags := make(map[string]bool)
	flagFound := false

	scanner := bufio.NewScanner(cpuinfoFile)
	for scanner.Scan() {
		keyValue := strings.Split(scanner.Text(), ":")
		keyValue[0] = strings.TrimSpace(keyValue[0])
		if keyValue[0] == "flags" || keyValue[0] == "Features" {
			flagFound = true
			flagValues := strings.Fields(keyValue[1])
			for _, val := range flagValues {
				flags[val] = true
			}
		}
	}

	if !flagFound {
		return nil, errors.New("no flags found in /proc/cpuinfo")
	}

	return flags, nil
}

func getCPUVirtualization(flags map[string]bool) (*cpuVirtualizationInfo, error) {
	var cpuVirtualization cpuVirtualizationInfo

	const ia32FeatureLocked = 1 << 0
	const ia32FeatureEnableVmxInsideSmx = 1 << 1
	const ia32FeatureEnableVmxOutsideSmx = 1 << 2
	const vmCrLockedBit = 1 << 3
	const vmCrSvmeDisabledBit = 1 << 4
	// The msr address for IA32_FEATURE_CONTROL (0x3A), used to report vmx
	// virtualization data.
	const vmxMsrReg = 0x3A
	// The msr address for VM_CRl (C001_0114), used to report svm virtualization
	// data.
	const svmMsrReg = 0xC0010114

	_, vmxPresent := flags["vmx"]
	_, svmPresent := flags["svm"]

	if vmxPresent {
		cpuVirtualization.Type = "VMX"
	} else if svmPresent {
		cpuVirtualization.Type = "SVM"
	} else {
		return nil, nil
	}

	if vmxPresent {
		val, err := readMsr(int64(vmxMsrReg))
		if err != nil {
			return nil, err
		}

		cpuVirtualization.IsLocked = uint(val&ia32FeatureLocked) > 0
		cpuVirtualization.IsEnabled = uint(val&ia32FeatureEnableVmxInsideSmx) > 0 ||
			uint(val&ia32FeatureEnableVmxOutsideSmx) > 0
	}

	if svmPresent {
		val, err := readMsr(int64(svmMsrReg))
		if err != nil {
			return nil, err
		}
		cpuVirtualization.IsLocked = uint(val&vmCrLockedBit) > 0
		cpuVirtualization.IsEnabled = !(uint(val&vmCrSvmeDisabledBit) > 0)
	}

	return &cpuVirtualization, nil
}

func verifyPhysicalCPU(physicalCPU *physicalCPUInfo, checkCPUVirtualization bool) error {
	if len(physicalCPU.LogicalCPUs) < 1 {
		return errors.Errorf("invalid LogicalCPUs, got %d; want 1+", len(physicalCPU.LogicalCPUs))
	}

	for _, logicalCPU := range physicalCPU.LogicalCPUs {
		if err := verifyLogicalCPU(&logicalCPU); err != nil {
			return errors.Wrap(err, "failed to verify logical CPU")
		}
	}

	if *physicalCPU.ModelName == "" {
		return errors.New("empty CPU model name")
	}

	if checkCPUVirtualization {
		expectedFlags, err := getFlags()
		if err != nil {
			return err
		}

		receivedFlags := make(map[string]bool)
		for _, flag := range physicalCPU.Flags {
			receivedFlags[flag] = true
		}

		if !reflect.DeepEqual(receivedFlags, expectedFlags) {
			return errors.Errorf("Flag reported incorrectly, expect: %v got: %v", expectedFlags, receivedFlags)
		}

		expectedPhysicalCPUVirtualization, err := getCPUVirtualization(receivedFlags)
		if err != nil {
			return err
		}

		if !reflect.DeepEqual(expectedPhysicalCPUVirtualization, physicalCPU.CPUVirtualization) {
			return errors.Errorf("CPU virtualization reported incorrectly, expect: %v got: %v",
				expectedPhysicalCPUVirtualization, physicalCPU.CPUVirtualization)
		}
	}

	return nil
}

func verifyLogicalCPU(logicalCPU *logicalCPUInfo) error {
	for _, cState := range logicalCPU.CStates {
		if err := verifyCState(cState); err != nil {
			return errors.Wrap(err, "failed to verify c_state")
		}
	}

	return nil
}

func verifyCState(cState cStateInfo) error {
	if cState.Name == "" {
		return errors.New("empty name")
	}

	return nil
}

func validateCPUData(info *cpuInfo, checkCPUVirtualization bool) error {
	// Every board should have at least one physical CPU
	if len(info.PhysicalCPUs) < 1 {
		return errors.Errorf("invalid PhysicalCPUs, got %d; want 1+", len(info.PhysicalCPUs))
	}

	if info.NumTotalThreads <= 0 {
		return errors.Errorf("invalid NumTotalThreads, got %d; want 1+", info.NumTotalThreads)
	}
	if info.Architecture == "" {
		return errors.New("empty architecture")
	}

	for _, physicalCPU := range info.PhysicalCPUs {
		if err := verifyPhysicalCPU(&physicalCPU, checkCPUVirtualization); err != nil {
			return errors.Wrap(err, "failed to verify physical CPU")
		}
	}

	return nil
}

func validateKeyLocker(keylocker *keylockerinfo) error {
	if !keylocker.KeylockerConfigured {
		return errors.Errorf("failed to configure keylocker: %t", keylocker)
	}
	return nil
}

func getVirtualization() (virtualizationInfo, error) {
	var virtualization virtualizationInfo

	virtualization.HasKvmDevice = true
	if _, err := os.Stat("/dev/kvm"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			virtualization.HasKvmDevice = false
		} else {
			return virtualization, err
		}
	}

	if _, err := os.Stat("/sys/devices/system/cpu/smt"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			virtualization.IsSmtActive = false
			virtualization.SmtControl = "notimplemented"
			return virtualization, nil
		}
		return virtualization, err
	}

	smtActiveContent, err := ioutil.ReadFile("/sys/devices/system/cpu/smt/active")
	if err != nil {
		return virtualization, errors.New("could not read /sys/devices/system/cpu/smt/active")
	}
	smtActiveMap := map[string]bool{
		"1": true,
		"0": false,
	}
	smtActive, ok := smtActiveMap[strings.TrimSpace(string(smtActiveContent))]
	if !ok {
		return virtualization, errors.Errorf("error parsing /sys/devices/system/cpu/smt/active: %v", smtActiveContent)
	}
	virtualization.IsSmtActive = smtActive

	smtControl, err := ioutil.ReadFile("/sys/devices/system/cpu/smt/control")
	if err != nil {
		return virtualization, errors.New("could not read /sys/devices/system/cpu/smt/control")
	}
	virtualization.SmtControl = strings.TrimSpace(string(smtControl))

	return virtualization, nil
}

func validateVulnerabilities(vulnerabilities map[string]vulnerabilityInfo) error {
	for name, vulnerability := range vulnerabilities {
		if out, err := ioutil.ReadFile("/sys/devices/system/cpu/vulnerabilities/" + name); err != nil {
			return errors.Errorf("failed to read vulnerability: %s", name)
		} else if strings.TrimSpace(string(out)) != vulnerability.Message ||
			vulnerability.Status == "Unrecognized" {
			return errors.Errorf("vulnerability reported incorrectly: %s", name)
		}
	}

	return nil
}

func ProbeCPUInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryCPU}
	testParam := s.Param().(cpuInfoTestParams)

	var info cpuInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		s.Fatal("Failed to run telem command: ", err)
	}

	if err := validateCPUData(&info, testParam.checkCPUVirtualization); err != nil {
		s.Fatal("Failed to validate cpu data: ", err)
	}

	if testParam.checkVirtualization {
		virtualization, err := getVirtualization()
		if err != nil {
			s.Fatalf("Failed to get virtualization, err [%v]", err)
		}
		if virtualization != info.Virtualization {
			s.Fatalf("Failed to validate virtualization, expected %+v got %+v ", virtualization, info.Virtualization)
		}
	}

	if testParam.checkVulnerability {
		if err := validateVulnerabilities(info.Vulnerabilities); err != nil {
			s.Fatal("Failed to validate cpu vulnerabilities: ", err)
		}
	}

	out, err := ioutil.ReadFile("/proc/crypto")
	if err != nil {
		s.Fatal("Failed to read /proc/crypto file: ", err)
	}
	// Check whether the system supports vPro feature or not.
	if strings.Contains(string(out), "aeskl") {
		if err := validateKeyLocker(info.KeylockerInfo); err != nil {
			s.Fatal("Failed to validate KeyLocker: ", err)
		}
	} else {
		if info.KeylockerInfo != nil {
			s.Fatal("Failed to validate empty memory keyLockerdata")
		}
	}
}
