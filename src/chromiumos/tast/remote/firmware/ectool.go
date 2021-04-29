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
)

// FWImageType is the type of firmware (RO or RW).
type FWImageType string

// The different firmware image type.
const (
	FWImageTypeUnknown FWImageType = "unknown"
	FWImageTypeRO      FWImageType = "RO"
	FWImageTypeRW      FWImageType = "RW"
)

func (t *FWImageType) String() string {
	return string(*t)
}

// ECToolName specifies which of the many Chromium EC based MCUs ectool will
// be communicated with.
// Some options are cros_ec, cros_fp, cros_pd, cros_scp, and cros_ish.
type ECToolName string

const (
	// ECToolNameMain selects the main EC using cros_ec.
	ECToolNameMain = ECToolName("cros_ec")
	// ECToolNameFingerprint selects the FPMCU using cros_fp.
	ECToolNameFingerprint = ECToolName("cros_fp")
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

// Hello pings the EC as a communication check.
// It returns nil if communication was established successfully and an
// error otherwise.
func (ec *ECTool) Hello(ctx context.Context) error {
	_, err := ec.Command("hello").Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return errors.Wrap(err, "running 'ectool hello' on DUT")
	}
	return nil
}

// ECToolVersion holds the version parts that are returned by the
// ectool version command.
type ECToolVersion struct {
	Active      FWImageType
	ROVersion   string
	RWVersion   string
	BuildInfo   string
	ToolVersion string
}

func (ver *ECToolVersion) String() string {
	var b strings.Builder
	b.WriteString("Active Image: " + string(ver.Active) + "\n")
	b.WriteString("RO Version:   " + ver.ROVersion + "\n")
	b.WriteString("RW Version:   " + ver.RWVersion + "\n")
	b.WriteString("Build Info:   " + ver.BuildInfo + "\n")
	b.WriteString("Tool Version: " + ver.ToolVersion + "\n")
	return b.String()
}

// Version returns the EC version of the active firmware.
func (ec *ECTool) Version(ctx context.Context) (ECToolVersion, error) {
	ver := ECToolVersion{}

	output, err := ec.Command("version").Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return ver, errors.Wrap(err, "running 'ectool version' on DUT")
	}

	values := parseColonDelimited(string(output))

	var ok bool

	var active string
	if active, ok = values["Firmware copy"]; !ok {
		return ver, errors.New("parsing firmware copy")
	}
	switch ver.Active = FWImageType(active); ver.Active {
	case FWImageTypeRO, FWImageTypeRW, FWImageTypeUnknown:
	default:
		return ver, errors.Errorf("received unrecognized image type %q", active)
	}

	if ver.ROVersion, ok = values["RO version"]; !ok {
		return ver, errors.New("parsing RO version")
	}
	if ver.RWVersion, ok = values["RW version"]; !ok {
		return ver, errors.New("parsing RW version")
	}
	if ver.BuildInfo, ok = values["Build info"]; !ok {
		return ver, errors.New("parsing build info")
	}
	if ver.ToolVersion, ok = values["RW version"]; !ok {
		return ver, errors.New("parsing tool version")
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
	case FWImageTypeRO:
		return ver.ROVersion, nil
	case FWImageTypeRW:
		return ver.RWVersion, nil
	default:
		return "", errors.New("unknown active image")
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
