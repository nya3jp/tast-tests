// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package attenuator provides support for control of the Mini-circuits
// variable attenuator.
package attenuator

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
)

// Attenuator controls attenuator
type Attenuator struct {
	conn              *ssh.Conn
	ctx               context.Context
	hostname          string
	model             string
	maxFreq           string
	maxAtten          string
	channels          uint
	fixedAttenuations map[uint]map[uint]float64 //[chnnel][freq]:attenuation
}

// unpack is shamelessly borrowed from https://stackoverflow.com/questions/19832189/unpack-slices-on-assignment
func unpack(s []string, vars ...*string) {
	for i, str := range s {
		*vars[i] = str
	}
}

// sendCmd is local helper for sending actual command to attenuator.
func (a *Attenuator) sendCmd(cmd string) (string, error) {
	res, err := a.conn.Command("wget", "-q", "-O", "-",
		"http://"+a.hostname+"/:"+cmd).Output(a.ctx)
	if err != nil {
		return "", errors.Wrapf(err, "failed to run command %s", cmd)
	}

	return string(res), nil
}

// Open access to the attenuator.
func Open(ctx context.Context, host string, conn *ssh.Conn) (att *Attenuator, errRet error) {
	a := &Attenuator{}

	fixedAttenuations, found := HostFixedAttenuations[host]
	if !found {
		return nil, errors.New("Attenuator data not found for host " + host)
	}

	a.fixedAttenuations = fixedAttenuations

	a.conn = conn
	a.hostname = host
	a.ctx = ctx

	ret, err := a.sendCmd("MN?")
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
	unpack(retSlice, &a.model, &a.maxFreq, &a.maxAtten)
	if a.model == "RC4DAT" {
		a.channels = 4
	} else {
		a.channels = 1
	}

	return a, nil
}

// Close is used for cleaning up resources.
func (a *Attenuator) Close() {
}

// GetAtten returns attenuation of particular attenuator channel.
func (a *Attenuator) GetAtten(channel uint) (float32, error) {
	if channel > a.channels {
		return 0, errors.New("bad channel")
	}
	// This command is not quite documented, but surprisingly, it works!
	ret, err := a.sendCmd(fmt.Sprintf("CHAN:%d:ATT?", channel+1))
	if err != nil {
		return 0, err
	}

	retFloat, err := strconv.ParseFloat(ret, 32)
	if err == nil {
		return 0, err
	}
	return float32(retFloat), nil
}

// SetAtten sets attenuation on particular channel.
func (a *Attenuator) SetAtten(channel uint, val float64) error {
	if channel > a.channels {
		return errors.New("bad channel")
	}
	if ret, _ := strconv.ParseFloat(a.maxAtten, 64); val > float64(ret) {
		return errors.New("bad attenuation value")
	}
	ret, err := a.sendCmd(fmt.Sprintf("CHAN:%d:SETATT:%f", channel+1, val))
	if err != nil {
		return err
	}

	if ret != "1" {
		return errors.New("failed to set given attenuation")
	}
	return nil
}

// approximateFrequency finds an approximate frequency to freq.
//
// In case freq is not present in fixedAttenuations, we use a value
// from a nearby channel as an approximation.
//
// @param attenuator_num: attenuator in question on the remote host.  Each
// 		attenuator has a different fixed path loss per frequency.
// @param freq: int frequency in MHz.
// @returns int approximate frequency from fixedAttenuations.
func (a *Attenuator) approximateFrequency(channel, freq uint) uint {
	oldOffset := 0
	approxFreq := uint(0)
	for definedFreq := range a.fixedAttenuations[channel] {
		newOffset := int(math.Abs(float64(definedFreq - freq)))
		if (oldOffset == 0) || (newOffset < oldOffset) {
			oldOffset = newOffset
			approxFreq = definedFreq
		}
	}

	// TODO: Add logging
	// logging.debug("Approximating attenuation for frequency %d with " +
	// 	"constants for frequency %d.", freq, approxFreq)
	return approxFreq
}

// SetTotalAttenuation sets the total line attenuation on one or all attenuators.
//
// @param atten_db: int level of attenuation in dB.  This must be
// higher than the fixed attenuation level of the affected
// attenuators.
// @param frequency_mhz: int frequency for which to calculate the
// total attenuation.  The fixed component of attenuation
// varies with frequency.
// @param attenuator_num: int attenuator to change, or None to
// set all variable attenuators.
func (a *Attenuator) SetTotalAttenuation(channel uint, attenDb float64, frequencyMhz uint) {
	freqToFixedLoss := a.fixedAttenuations[channel]
	approxFreq := a.approximateFrequency(channel, frequencyMhz)
	variableAttenDb := attenDb - freqToFixedLoss[approxFreq]
	a.SetAtten(channel, variableAttenDb)
}

// GetMinimalTotalAttenuation Gets attenuator's maximum fixed attenuation value.
//
// This is pulled from the current attenuator's lines and becomes the
// minimal total attenuation when stepping through attenuation levels.
//
// @return maximum starting attenuation value
func (a *Attenuator) GetMinimalTotalAttenuation(channel uint) float64 {
	var maxAtten float64
	for _, atten := range a.fixedAttenuations[channel] {
		maxAtten = math.Max(atten, maxAtten)
	}
	return maxAtten
}

// GetMaximumAttenuation gets attenuator's maximum attenuator's attenuation value.
func (a *Attenuator) GetMaximumAttenuation() float64 {
	// TODO
	return 90
}
