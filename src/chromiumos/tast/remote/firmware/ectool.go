// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to interact with the DUT's embedded controller (EC)
// via the host command `ectool`.

package firmware

import (
	"context"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// UnmarshalerECTool is the interface implemented by types that can unmarshal
// the ectool output of themselves.
//
// This is inspired by json.Unmarshaler.
type UnmarshalerECTool interface {
	UnmarshalECTool([]byte) error
}

// UnmarshalECTool parses the output from ectool into v.
func UnmarshalECTool(data []byte, v UnmarshalerECTool) error {
	return v.UnmarshalECTool(data)
}

// FWCopy is the firmware image copy (RO or RW).
type FWCopy string

// The different firmware image copies.
const (
	FWCopyUnknown FWCopy = "unknown"
	FWCopyRO      FWCopy = "RO"
	FWCopyRW      FWCopy = "RW"
)

// UnmarshalECTool unmarshal part of ectool's output into an FWCopy.
func (u *FWCopy) UnmarshalECTool(data []byte) error {
	switch active := FWCopy(data); active {
	case FWCopyRO, FWCopyRW, FWCopyUnknown:
		*u = active
		return nil
	default:
		return errors.Errorf("received unrecognized image copy %q", active)
	}
}

// ECToolName specifies which of the many Chromium EC based MCUs ectool will
// be communicated with.
// Some options are cros_ec, cros_fp, cros_pd, cros_scp, and cros_ish.
type ECToolName string

const (
	// ECToolNameMain selects the main EC using cros_ec.
	ECToolNameMain ECToolName = "cros_ec"
	// ECToolNameFingerprint selects the FPMCU using cros_fp.
	ECToolNameFingerprint ECToolName = "cros_fp"
)

// ECTool allows for interaction with the host command `ectool`.
type ECTool struct {
	dut  *dut.DUT
	name ECToolName
}

// NewECTool creates an ECTool.
func NewECTool(d *dut.DUT, name ECToolName) *ECTool {
	return &ECTool{dut: d, name: name}
}

// Command return the prebuilt ssh Command with options and args applied.
func (ec *ECTool) Command(args ...string) *ssh.Cmd {
	args = append([]string{"--name=" + string(ec.name)}, args...)
	return ec.dut.Conn().Command("ectool", args...)
}

// ECToolVersion holds the version parts that are returned by the
// ectool version command.
type ECToolVersion struct {
	Active      FWCopy
	ROVersion   string
	RWVersion   string
	BuildInfo   string
	ToolVersion string
}

// String composes a multi-line print friendly string of ECToolVersion.
func (ver *ECToolVersion) String() string {
	var b strings.Builder
	b.WriteString("Active Image: " + string(ver.Active) + "\n")
	b.WriteString("RO Version:   " + ver.ROVersion + "\n")
	b.WriteString("RW Version:   " + ver.RWVersion + "\n")
	b.WriteString("Build Info:   " + ver.BuildInfo + "\n")
	b.WriteString("Tool Version: " + ver.ToolVersion)
	return b.String()
}

// UnmarshalECTool unmarshals ectool output into an ECToolVersion.
func (ver *ECToolVersion) UnmarshalECTool(data []byte) error {
	values := parseColonDelimited(string(data))

	var v ECToolVersion

	active, ok := values["Firmware copy"]
	if !ok {
		return errors.New("missing firmware copy")
	}
	if err := UnmarshalECTool([]byte(active), &v.Active); err != nil {
		return err
	}

	if v.ROVersion, ok = values["RO version"]; !ok {
		return errors.New("missing RO version")
	}
	if v.RWVersion, ok = values["RW version"]; !ok {
		return errors.New("missing RW version")
	}
	if v.BuildInfo, ok = values["Build info"]; !ok {
		return errors.New("missing build info")
	}
	if v.ToolVersion, ok = values["Tool version"]; !ok {
		return errors.New("missing tool version")
	}

	*ver = v
	return nil
}

// Version returns the EC version of the active firmware.
func (ec *ECTool) Version(ctx context.Context) (ECToolVersion, error) {
	output, err := ec.Command("version").Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return ECToolVersion{},
			errors.Wrap(err, "running 'ectool version' on DUT")
	}

	var ver ECToolVersion
	if err := UnmarshalECTool(output, &ver); err != nil {
		testing.ContextLogf(ctx, "Bad ectool output: %q", string(output))
		return ECToolVersion{},
			errors.Wrap(err, "failed to unmarshal ectool version")
	}

	return ver, nil
}

// VersionActive returns the EC version of the active firmware.
func (ec *ECTool) VersionActive(ctx context.Context) (string, error) {
	ver, err := ec.Version(ctx)
	if err != nil {
		return "", err
	}

	switch ver.Active {
	case FWCopyRO:
		return ver.ROVersion, nil
	case FWCopyRW:
		return ver.RWVersion, nil
	default:
		return "", errors.Errorf("unknown active image %q", string(ver.Active))
	}
}

// parseColonDelimited parses colon delimited key values into a map.
func parseColonDelimited(text string) map[string]string {
	ret := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		// Note that the build info line uses ':'s as time of date delimiters.
		splits := strings.SplitN(line, ":", 2)
		if len(splits) != 2 {
			continue
		}
		ret[strings.TrimSpace(splits[0])] = strings.TrimSpace(splits[1])
	}
	return ret
}
