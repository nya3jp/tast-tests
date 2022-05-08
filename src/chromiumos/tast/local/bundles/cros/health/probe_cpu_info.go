// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package health

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"
	"unsafe"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

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
		Fixture: "crosHealthdRunning",
		Timeout: 3 * time.Minute,
		Params: []testing.Param{{
			Val: cpuInfoTestParams{
				checkVulnerability:     false,
				checkVirtualization:    false,
				checkCPUVirtualization: false,
			},
		}, {
			Name: "vulnerability",
			// TODO(b/231537546): Promote to critical once tests are stable.
			ExtraAttr: []string{"informational"},
			Val: cpuInfoTestParams{
				checkVulnerability:     true,
				checkVirtualization:    false,
				checkCPUVirtualization: false,
			},
		}, {
			Name: "virtualization",
			// TODO(b/231537546): Promote to critical once tests are stable.
			ExtraAttr: []string{"informational"},
			Val: cpuInfoTestParams{
				checkVulnerability:     false,
				checkVirtualization:    true,
				checkCPUVirtualization: false,
			},
		}, {
			Name: "cpu_virtualization",
			// TODO(b/231537546): Promote to critical once tests are stable.
			ExtraAttr: []string{"informational"},
			Val: cpuInfoTestParams{
				checkVulnerability:     false,
				checkVirtualization:    false,
				checkCPUVirtualization: true,
			},
		}},
	})
}

func readMsr(msrReg int64, logicalID int) (uint64, error) {
	// msr read/write always comes at 8 bytes: https://man7.org/linux/man-pages/man4/msr.4.html
	const msrSize = 8
	bytes := make([]byte, msrSize)
	msrFile, err := os.Open(fmt.Sprintf("/dev/cpu/%v/msr", logicalID))
	if err != nil {
		return 0, errors.Wrap(err, "could not open msr file")
	}
	defer msrFile.Close()
	msrFile.Seek(msrReg, 0)
	readSize, err := msrFile.Read(bytes)
	if err != nil {
		return 0, errors.Wrap(err, "could not read msr file")
	}
	if readSize != msrSize {
		return 0, errors.Errorf("msr read size not match, expect: %v got: %v", msrSize, readSize)
	}

	// As seen in the ReadMsr tool implementation, return value from reading the
	// register is directly type-casted to uint64. We use pointer casting to
	// achieve the same result.
	//
	// https://github.com/intel/msr-tools/blob/eec71d977a83f8dc76bc3ccc6de5cbd3be378572/rdmsr.c#L235
	return *(*uint64)(unsafe.Pointer(&bytes[0])), nil
}

func getFlags() (map[string]bool, error) {
	cpuinfoContent, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		return nil, errors.Wrap(err, "could not read /proc/cpuinfo")
	}

	// TODO(b/231673454): Change so that getFlags() accept a physical CPU id, and
	// can get flag specific to that physical CPU.
	//
	// Until then, report the first set of flag found in /proc/cpuinfo.
	scanner := bufio.NewScanner(bytes.NewReader(cpuinfoContent))
	for scanner.Scan() {
		keyValue := strings.Split(scanner.Text(), ":")
		keyValue[0] = strings.TrimSpace(keyValue[0])
		if keyValue[0] == "flags" || keyValue[0] == "Features" {
			flags := make(map[string]bool)
			flagValues := strings.Fields(keyValue[1])
			for _, val := range flagValues {
				flags[val] = true
			}
			return flags, nil
		}
	}

	return nil, errors.New("no flags found in /proc/cpuinfo")
}

func getExpectedCPUVirtualization(flags map[string]bool) (*cpuVirtualizationInfo, error) {
	var cpuVirtualization cpuVirtualizationInfo

	const (
		ia32FeatureLocked              = 1 << 0
		ia32FeatureEnableVmxInsideSmx  = 1 << 1
		ia32FeatureEnableVmxOutsideSmx = 1 << 2
		vmCrLockedBit                  = 1 << 3
		vmCrSvmeDisabledBit            = 1 << 4
		// The msr address for IA32_FEATURE_CONTROL (0x3A), used to report vmx
		// virtualization data.
		vmxMsrReg = 0x3A
		// The msr address for VM_CRl (C001_0114), used to report svm virtualization
		// data.
		svmMsrReg = 0xC0010114
	)

	// TODO(b/231673454): Read CPU virtualization data specific to a particular
	// physical CPU.
	//
	// Until then, only check the first physical cpu and assume all others are the
	//  same.
	if _, vmxPresent := flags["vmx"]; vmxPresent {
		cpuVirtualization.Type = "VMX"
		val, err := readMsr(vmxMsrReg, 0)
		if err != nil {
			return nil, err
		}
		cpuVirtualization.IsLocked = val&ia32FeatureLocked != 0
		cpuVirtualization.IsEnabled = val&ia32FeatureEnableVmxInsideSmx != 0 ||
			val&ia32FeatureEnableVmxOutsideSmx != 0
		return &cpuVirtualization, nil
	}
	if _, svmPresent := flags["svm"]; svmPresent {
		cpuVirtualization.Type = "SVM"
		val, err := readMsr(svmMsrReg, 0)
		if err != nil {
			return nil, err
		}
		cpuVirtualization.IsLocked = val&vmCrLockedBit != 0
		cpuVirtualization.IsEnabled = val&vmCrSvmeDisabledBit == 0
		return &cpuVirtualization, nil
	}
	return nil, nil
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

		if diff := cmp.Diff(expectedFlags, receivedFlags); diff != "" {
			return errors.Errorf("Flag reported incorrectly: (-got +want) %s", diff)
		}

		expectedPhysicalCPUVirtualization, err := getExpectedCPUVirtualization(expectedFlags)
		if err != nil {
			return err
		}

		if diff := cmp.Diff(expectedPhysicalCPUVirtualization, physicalCPU.CPUVirtualization); diff != "" {
			return errors.Errorf("CPU virtualization reported incorrectly: (-got +want) %s", diff)
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

	// TODO(b/231673454): Delete this test once we can access each physical CPU
	// separately.
	//
	// Until then, we check to see if all CPU report the same information (flags
	// and msr).
	if checkCPUVirtualization {
		if err := validateCPUEquality(info.PhysicalCPUs); err != nil {
			return err
		}
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

func getExpectedVirtualization() (virtualizationInfo, error) {
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
		return virtualization, errors.Wrap(err, "could not read /sys/devices/system/cpu/smt/active")
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
		return virtualization, errors.Wrap(err, "could not read /sys/devices/system/cpu/smt/control")
	}
	virtualization.SmtControl = strings.TrimSpace(string(smtControl))

	return virtualization, nil
}

func validateVirtualization(gotVirtualization virtualizationInfo) error {
	expectedVirtualization, err := getExpectedVirtualization()
	if err != nil {
		return errors.Wrap(err, "failed to get virtualization")
	}
	if expectedVirtualization != gotVirtualization {
		return errors.Errorf("failed to validate Virtualization, expected %+v got %+v ", expectedVirtualization, gotVirtualization)
	}
	return nil
}

func validateVulnerabilities(gotVulnerabilities map[string]vulnerabilityInfo) error {
	expectedVulnerabilities := make(map[string]vulnerabilityInfo)
	vulnerabilityFiles, err := ioutil.ReadDir("/sys/devices/system/cpu/vulnerabilities")
	if err != nil {
		return errors.Wrap(err, "failed to read vulnerabilities directory")
	}
	for _, vulnerabilityFile := range vulnerabilityFiles {
		name := vulnerabilityFile.Name()
		out, err := ioutil.ReadFile("/sys/devices/system/cpu/vulnerabilities/" + name)
		if err != nil {
			return errors.Wrapf(err, "failed to read vulnerability: %s", name)
		}
		expectedVulnerabilities[name] =
			vulnerabilityInfo{Message: strings.TrimSpace(string(out))}
	}

	ignoreOpt := cmpopts.IgnoreFields(vulnerabilityInfo{}, "Status")
	if diff := cmp.Diff(gotVulnerabilities, expectedVulnerabilities, ignoreOpt); diff != "" {
		return errors.Errorf("Vulnerability reported differently: (-got +want) %s", diff)
	}

	for name, vulnerability := range gotVulnerabilities {
		if vulnerability.Status == "Unrecognized" {
			return errors.Errorf("unrecognized vulnerability status, name: %v message: %v", name, vulnerability.Message)
		}
	}

	return nil
}

func validateCPUEquality(physicalCPUs []physicalCPUInfo) error {
	// Compare each physical CPU for equality of flag and virtualization.
	if len(physicalCPUs) < 1 {
		return errors.New("no physical CPU present on the device")
	}
	expectedFlags := physicalCPUs[0].Flags
	sort.Strings(expectedFlags)
	expectedCPUVirtualization := physicalCPUs[0].CPUVirtualization
	for _, physicalCPU := range physicalCPUs {
		receivedFlags := physicalCPU.Flags
		sort.Strings(receivedFlags)
		if diff := cmp.Diff(receivedFlags, expectedFlags); diff != "" {
			return errors.Errorf("flags different across CPU, : (-got +want) %s", diff)
		}
		if diff := cmp.Diff(physicalCPU.CPUVirtualization, expectedCPUVirtualization); diff != "" {
			return errors.Errorf("CPU virtualization different across CPU, : (-got +want) %s", diff)
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
		if err := validateVirtualization(info.Virtualization); err != nil {
			s.Fatal("Failed to validate virtualization: ", err)
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
