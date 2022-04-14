// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

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
	Status     string `json:"status"`
	Mitigation string `json:"mitigation"`
}

type cpuInfo struct {
	Architecture        string                       `json:"architecture"`
	NumTotalThreads     jsontypes.Uint32             `json:"num_total_threads"`
	TemperatureChannels []temperatureChannelInfo     `json:"temperature_channels"`
	PhysicalCPUs        []physicalCPUInfo            `json:"physical_cpus"`
	KeylockerInfo       *keylockerinfo               `json:"keylocker_info"`
	Virtualization      *virtualizationInfo          `json:"virtualization"`
	Vulnerabilities     map[string]vulnerabilityInfo `json:"vulnerabilities"`
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
		Fixture: "crosHealthdRunning",
	})
}

func readMsr(msrReg uint32) ([]byte, error) {
	// msr read/write always comes at 8 bytes: https://man7.org/linux/man-pages/man4/msr.4.html
	const msrSize = 8
	bytes := make([]byte, msrSize)
	msrFile, err := os.Open(fmt.Sprintf("/dev/cpu/0/msr"))
	if err != nil {
		return nil, errors.New("could not open msr file")
	}
	msrFile.Seek(int64(msrReg), 0)
	readSize, err := msrFile.Read(bytes)
	if err != nil || readSize != msrSize {
		return nil, errors.New("could not read msr file")
	}

	return bytes, nil
}

func verifyPhysicalCPU(physicalCPU *physicalCPUInfo) error {
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

	cpuinfoContent, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		return errors.New("could not read /proc/cpuinfo")
	}
	for _, flag := range physicalCPU.Flags {
		if !strings.Contains(string(cpuinfoContent), flag) {
			return errors.Errorf("Nonexistent flag: %s", flag)
		}
	}

	if physicalCPU.CPUVirtualization != nil {
		var isLocked bool
		var isEnabled bool

		if physicalCPU.CPUVirtualization.Type == "VMX" {
			if !strings.Contains(string(cpuinfoContent), "vmx") {
				return errors.New("vmx flag does not exist")
			}
			// The msr address for IA32_FEATURE_CONTROL (0x3A), used to report vmx
			// virtualization data.
			vmxMsrReg := 0x3A
			bytes, err := readMsr(uint32(vmxMsrReg))
			if err != nil {
				return err
			}
			isLocked = uint(bytes[0]&(1<<0)) > 0
			isEnabled = uint(bytes[0]&(1<<1)) > 0 || uint(bytes[0]&(1<<2)) > 0
		}

		if physicalCPU.CPUVirtualization.Type == "SVM" {
			if !strings.Contains(string(cpuinfoContent), "svm") {
				return errors.New("svm flag does not exist")
			}
			// The msr address for VM_CRl (C001_0114), used to report svm virtualization
			// data.
			var svmMsrReg uint32 = 0xC0010114
			bytes, err := readMsr(uint32(svmMsrReg))
			if err != nil {
				return err
			}
			isLocked = uint(bytes[0]&(1<<3)) > 0
			isEnabled = !(uint(bytes[0]&(1<<4)) > 0)
		}

		if isLocked != physicalCPU.CPUVirtualization.IsLocked {
			return errors.Errorf("cpu virtualization locked status got: %t, want: %t",
				physicalCPU.CPUVirtualization.IsLocked, isLocked)
		}
		if isEnabled != physicalCPU.CPUVirtualization.IsEnabled {
			return errors.Errorf("cpu virtualization enabled status got: %t, want: %t",
				physicalCPU.CPUVirtualization.IsEnabled, isEnabled)
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

func validateCPUData(info *cpuInfo) error {
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
		if err := verifyPhysicalCPU(&physicalCPU); err != nil {
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

func validateVirtualization(virtualization *virtualizationInfo) error {

	if virtualization == nil {
		return nil
	}

	kvmExists := true
	if _, err := os.Stat("/dev/kvm"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			kvmExists = false
		} else {
			return errors.New(err.Error())
		}
	}
	if kvmExists != virtualization.HasKvmDevice {
		return errors.Errorf("kvm device existence got: %t, want: %t", virtualization.HasKvmDevice, kvmExists)
	}

	if _, err := os.Stat("/sys/devices/system/cpu/smt"); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if virtualization.IsSmtActive != false {
				return errors.Errorf("smt active got: %t, want: %t", virtualization.IsSmtActive, false)
			}
			if virtualization.SmtControl != "Not Implemented" {
				return errors.Errorf("smt control got: %v, want: %v", virtualization.SmtControl, "Not Implemented")
			}
			return nil
		}
		return errors.New(err.Error())
	}
	smtActiveContent, err := ioutil.ReadFile("/sys/devices/system/cpu/smt/active")
	if err != nil {
		return errors.New("could not read /sys/devices/system/cpu/smt/active")
	}
	smtActiveMap := map[string]bool{
		"1": true,
		"0": false,
	}
	if smtActive, ok := smtActiveMap[strings.TrimSpace(string(smtActiveContent))]; !ok {
		return errors.New("error parsing /sys/devices/system/cpu/smt/active")
	} else if virtualization.IsSmtActive != smtActive {
		return errors.Errorf("smt active got: %t, want: %t", virtualization.IsSmtActive, smtActive)
	}

	smtControlContent, err := ioutil.ReadFile("/sys/devices/system/cpu/smt/control")
	if err != nil {
		return errors.New("could not read /sys/devices/system/cpu/smt/control")
	}
	smtControlMap := map[string]string{
		"on":             "On",
		"off":            "Off",
		"forceoff":       "Force Off",
		"notsupported":   "Not Supported",
		"notimplemented": "Not Implemented",
	}
	if smtControl, ok := smtControlMap[strings.TrimSpace(string(smtControlContent))]; !ok {
		return errors.New("error parsing /sys/devices/system/cpu/smt/control")
	} else if virtualization.SmtControl != smtControl {
		return errors.Errorf("smt control got: %s, want: %s", virtualization.SmtControl, smtControl)
	}

	return nil
}

func validateVulnerabilities(vulnerabilities map[string]vulnerabilityInfo) error {
	for name, vulnerability := range vulnerabilities {
		if out, err := ioutil.ReadFile("/sys/devices/system/cpu/vulnerabilities/" + name); err != nil {
			return errors.Errorf("failed to read vulnerability: %s", name)
		} else if !(strings.Contains(string(out), vulnerability.Status) &&
			strings.Contains(string(out), vulnerability.Mitigation)) {
			return errors.Errorf("vulnerability reporter incorrectly: %s", name)
		}
	}

	return nil
}

func ProbeCPUInfo(ctx context.Context, s *testing.State) {
	params := croshealthd.TelemParams{Category: croshealthd.TelemCategoryCPU}
	var info cpuInfo
	if err := croshealthd.RunAndParseJSONTelem(ctx, params, s.OutDir(), &info); err != nil {
		s.Fatal("Failed to run telem command: ", err)
	}

	if err := validateCPUData(&info); err != nil {
		s.Fatalf("Failed to validate cpu data, err [%v]", err)
	}

	if err := validateVirtualization(info.Virtualization); err != nil {
		s.Fatalf("Failed to validate virtualization, err [%v]", err)
	}

	if err := validateVulnerabilities(info.Vulnerabilities); err != nil {
		s.Fatalf("Failed to validate cpu vulnerabilities, err [%v]", err)
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
