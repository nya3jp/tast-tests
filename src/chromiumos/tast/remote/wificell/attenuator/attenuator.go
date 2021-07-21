// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package attenuator controls of the Mini-Circuits RC4DAT
// programmable attenuator. It provides also a best-effort support for
// RCDAT, but due to lack the test sample, nothing is guaranteed.
package attenuator

import (
	"context"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/testing"
)

// Attenuator stores properties of a programmable attenuator.
type Attenuator struct {
	proxyConn           *ssh.Conn
	hostName            string
	hostIP              string
	model               string // Either RC4DAT or RCDAT.
	maxFreq             string
	maxAtten            float64
	channels            int
	fixedAttenuations   map[int]map[int]float64 // [channel][freq]:attenuation.
	minTotalAttenuation []float64
}

// unpack is shamelessly borrowed from https://stackoverflow.com/questions/19832189/unpack-slices-on-assignment
func unpack(s []string, vars ...*string) {
	for i, str := range s {
		if i == len(vars) {
			break // Soft fail, fill only provided vars.
		}
		*vars[i] = str
	}

}

// sendCmd sends a command to the attenuator via proxy.
func (a *Attenuator) sendCmd(ctx context.Context, cmd string) (string, error) {
	ret, err := a.proxyConn.CommandContext(ctx, "wget", "-q", "-O", "-",
		"http://"+a.hostIP+"/:"+cmd).Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to run command %s", cmd)
	}

	return string(ret), nil
}

// Open access to the attenuator.
func Open(ctx context.Context, host string, proxyConn *ssh.Conn) (att *Attenuator, errRet error) {
	a := &Attenuator{}

	fixedAttenuations, found := HostFixedAttenuations[strings.TrimSuffix(host, ".cros")]
	if !found {
		return nil, errors.Errorf("Attenuator data not found for host %s", host)
	}
	hostIPs, err := net.LookupIP(host)
	if err != nil {
		// For some reason, .cros is not always in search domain.
		hostIPs, err = net.LookupIP(fmt.Sprintf("%s.cros", host))
		if err != nil {
			return nil, errors.Errorf("could not resolve IP for host %s", host)
		}
	}

	a.fixedAttenuations = fixedAttenuations

	a.proxyConn = proxyConn
	a.hostName = host
	a.hostIP = hostIPs[0].String()

	// Get model number.
	ret, err := a.sendCmd(ctx, "MN?")
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(ret, "MN=") {
		return nil, errors.New("unexpected response")
	}

	retSlice := strings.Split(strings.TrimPrefix(ret, "MN="), "-")
	if len(retSlice) != 3 {
		return nil, errors.New("bad model format")
	}

	var maxAtten string
	unpack(retSlice, &a.model, &a.maxFreq, &maxAtten)
	if a.model == "RC4DAT" {
		a.channels = 4
	} else {
		a.channels = 1
	}

	a.maxAtten, err = strconv.ParseFloat(maxAtten, 64)
	if err != nil {
		return nil, errors.Wrapf(err,
			"Unable to parse attenuation from model number %s", ret)
	}

	if len(a.fixedAttenuations) < a.channels {
		return nil, errors.Errorf("not enough data entries (%d) in database for %d channels",
			len(a.fixedAttenuations), a.channels)
	}
	a.minTotalAttenuation = make([]float64, a.channels)
	for channel := 0; channel < a.channels; channel++ {
		for _, atten := range a.fixedAttenuations[channel] {
			a.minTotalAttenuation[channel] =
				math.Max(atten, a.minTotalAttenuation[channel])
		}
	}
	return a, nil
}

// Close is used for cleaning up resources.
func (a *Attenuator) Close() {
	// TODO: Check for resources to clean.
	// Unlike Telnet, HTTP over SSH seems to have no need for cleanup.
}

// Attenuation returns attenuation of the particular attenuator channel.
func (a *Attenuator) Attenuation(ctx context.Context, channel int) (float64, error) {
	if channel > a.channels {
		return 0, errors.Errorf(
			"invalid channel %d (valid channels: [%d, %d] for model %s)",
			channel, 0, a.channels-1, a.model)
	}

	// This command is not quite documented, but surprisingly, it works!
	ret, err := a.sendCmd(ctx, fmt.Sprintf("CHAN:%d:ATT?", channel+1))
	if err != nil {
		return 0, err
	}

	return strconv.ParseFloat(strings.TrimSpace(ret), 64)
}

// SetAttenuation sets attenuation on particular channel.
func (a *Attenuator) SetAttenuation(ctx context.Context, channel int, val float64) error {
	if channel >= a.channels {
		return errors.Errorf(
			"invalid channel %d (valid channels: [%d, %d] for model %s)",
			channel, 0, a.channels-1, a.model)
	}
	if val > a.maxAtten || val < 0 {
		return errors.Errorf("bad attenuation value %f", val)
	}
	ret, err := a.sendCmd(ctx, fmt.Sprintf("CHAN:%d:SETATT:%f", channel+1, val))
	if err != nil {
		return err
	}

	if ret != "1" {
		return errors.Errorf("failed to set attenuation %f for channel %d, ret: %s",
			val, channel, ret)
	}

	testing.ContextLogf(ctx, "%ddb attenuation set successfully on attenautor %d", int(val), channel)

	return nil
}

// approximateFrequency finds an approximate frequency to freq.
//
// In case freq is not present in fixedAttenuations, we use a value
// from a nearby channel as an approximation.
func (a *Attenuator) approximateFrequency(ctx context.Context, channel, freq int) int {
	minOffset := math.MaxInt64
	approxFreq := 0
	for definedFreq := range a.fixedAttenuations[channel] {
		offset := int(math.Abs(float64(definedFreq - freq)))
		if offset < minOffset {
			minOffset = offset
			approxFreq = definedFreq
		}
	}

	// TODO: Add logging
	// logging.debug("Approximating attenuation for frequency %d with " +
	// 	"constants for frequency %d.", freq, approxFreq)
	return approxFreq
}

// SetTotalAttenuation sets attenuation level for the specified frequency on the given channel.
//
// Each channel of the attenuator has different fixed attenuation/loss for different
// frequency. This function finds out the fixed attenuation of the given
// frequency and channel and adds a variable attenuation on it.
func (a *Attenuator) SetTotalAttenuation(ctx context.Context, channel int, attenDb float64, frequencyMhz int) error {
	if channel >= a.channels {
		return errors.Errorf(
			"invalid channel %d (valid channels: [%d, %d] for model %s)",
			channel, 0, a.channels-1, a.model)
	}
	freqToFixedLoss := a.fixedAttenuations[channel]
	approxFreq := a.approximateFrequency(ctx, channel, frequencyMhz)
	variableAttenDb := attenDb - freqToFixedLoss[approxFreq]
	return a.SetAttenuation(ctx, channel, variableAttenDb)
}

// MinTotalAttenuation returns the minimal attenuation the attenuator can be set for the given channel.
//
// This is obtained by finding the maximum fixed loss of all frequencies the channel has.
func (a *Attenuator) MinTotalAttenuation(channel int) (float64, error) {
	if channel >= a.channels {
		return 0, errors.Errorf(
			"invalid channel %d (valid channels: [%d, %d] for model %s)",
			channel, 0, a.channels-1, a.model)
	}
	return a.minTotalAttenuation[channel], nil
}

// MaximumAttenuation gets attenuator's maximum attenuation value.
func (a *Attenuator) MaximumAttenuation() float64 {
	return a.maxAtten
}
