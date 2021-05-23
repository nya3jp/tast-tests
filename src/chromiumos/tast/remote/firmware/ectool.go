// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This file implements functions to interact with the DUT's embedded controller (EC)
// via the host command `ectool`.

package firmware

import (
	"context"
	"reflect"
	"strings"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
)

// Unmarshaler
type Unmarshaler interface {
	UnmarshalECTool([]byte) error
}

// Unmarshal parses the output from ectool into v.
func Unmarshal(data []byte, v interface{}) error {
	switch value := v.(type) {
	case Unmarshaler:
		return value.UnmarshalECTool((data))
	case *string:
		*value = string(data)
		return nil
	default:
		// If the type is a *struct, we will try to use ':' to denote the fields.
		// This does not cover all ectool output and will need to be overridden
		// when the output is of a different type.
		if reflect.TypeOf(v).Kind() == reflect.Ptr &&
			reflect.TypeOf(v).Elem().Kind() == reflect.Struct {
			values := parseColonDelimited(string(data))

			stType := reflect.TypeOf(v).Elem()
			stVal := reflect.ValueOf(v).Elem()

			// The temporary new value that we will unmarshal to before
			// completion.
			newStPtr := reflect.New(stType)

			// Use each struct field's ectool tag to find its parsed value.
			// We then call Unmarshal with the struct field and parsed value
			// to actually set the struct field.
			for f := 0; f < stType.NumField(); f++ {
				stTypeField := stType.Field(f)
				tag, ok := stTypeField.Tag.Lookup("ectool")
				if !ok {
					return errors.New("Struct field " + stTypeField.Name +
						" doesn't contain ectool tag.")
				}

				value, ok := values[tag]
				if !ok {
					return errors.New("Failed to parse " + stTypeField.Name)
				}
				delete(values, tag)

				// Recursively call Unmarshal on each struct field to set value.
				ret := reflect.ValueOf(Unmarshal).Call([]reflect.Value{
					reflect.ValueOf([]byte(value)),
					newStPtr.Elem().Field(f).Addr(),
				})
				if err := ret[0].Interface(); err != nil {
					return err.(error)
				}
			}
			if len(values) != 0 {
				return errors.New("Extra parsed items remain")
			}
			stVal.Set(newStPtr.Elem())
			return nil
		}
		return errors.Errorf("Cannot unmarshal type %T.", v)
	}
}

// FWImageType is the type of firmware (RO or RW).
type FWImageType string

// The different firmware image type.
const (
	FWImageTypeUnknown FWImageType = "unknown"
	FWImageTypeRO      FWImageType = "RO"
	FWImageTypeRW      FWImageType = "RW"
)

func (u *FWImageType) UnmarshalECTool(data []byte) error {
	switch active := FWImageType(data); active {
	case FWImageTypeRO, FWImageTypeRW, FWImageTypeUnknown:
		*u = active
		return nil
	default:
		return errors.Errorf("received unrecognized image type %q", active)
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
	Active      FWImageType `ectool:"Firmware copy"`
	ROVersion   string      `ectool:"RO version"`
	RWVersion   string      `ectool:"RW version"`
	BuildInfo   string      `ectool:"Build info"`
	ToolVersion string      `ectool:"Tool version"`
}

func (ver *ECToolVersion) String() string {
	var b strings.Builder
	b.WriteString("Active Image: " + string(ver.Active) + "\n")
	b.WriteString("RO Version:   " + ver.ROVersion + "\n")
	b.WriteString("RW Version:   " + ver.RWVersion + "\n")
	b.WriteString("Build Info:   " + ver.BuildInfo + "\n")
	b.WriteString("Tool Version: " + ver.ToolVersion)
	return b.String()
}

// Version returns the EC version of the active firmware.
func (ec *ECTool) Version(ctx context.Context) (ECToolVersion, error) {
	output, err := ec.Command("version").Output(ctx, ssh.DumpLogOnError)
	if err != nil {
		return ECToolVersion{}, errors.Wrap(err, "running 'ectool version' on DUT")
	}

	var ver ECToolVersion
	return ver, Unmarshal(output, &ver)
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
