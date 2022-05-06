// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mojo

import (
	"context"
	"unicode/utf16"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
)

// ProfileState is the current state of an ESimProfile.
type ProfileState int32

// ProfileInstallResult is the result code for ESimProfile installation.
type ProfileInstallResult int32

// ESimOperationResult is the result code for operations on Euicc and ESimProfile.
type ESimOperationResult int32

const (
	// ProfileStatePending : The profile is not installed on the device.
	ProfileStatePending ProfileState = iota
	// ProfileStateInstalling : The profile is being installed.
	ProfileStateInstalling
	// ProfileStateInactive : The profile is installed but inactive.
	ProfileStateInactive
	// ProfileStateActive : The profile is installed and active.
	ProfileStateActive
)

const (
	// ProfileInstallSuccess : The profile installation succeeded.
	ProfileInstallSuccess ProfileInstallResult = iota
	// ProfileInstallFailure : the profile installation failed.
	ProfileInstallFailure
	// ProfileInstallErrorNeedsConfirmationCode : the installation requires a valid confirmation code.
	ProfileInstallErrorNeedsConfirmationCode
	// ProfileInstallErrorInvalidActivationCode : the given activation code is invalid.
	ProfileInstallErrorInvalidActivationCode
)

const (
	// ESimOperationSuccess : the operation succeeded.
	ESimOperationSuccess ESimOperationResult = iota
	// ESimOperationFailure : the operation failed.
	ESimOperationFailure
)

// ESimManager provides access to the Mojo eSIM management methods and holds the
// underlying JS config object.
type ESimManager struct {
	js *chrome.JSObject
}

// Euicc represents an EUICC (Embedded Universal Integrated
// Circuit Card) hardware available on the device and provides operations
// on the EUICC.
type Euicc struct {
	manager *ESimManager
	Eid     string
}

// EuiccProperties are the properties for an Euicc object.
type EuiccProperties struct {
	Eid      string `json:"eid"`
	IsActive bool   `json:"isActive"`
}

// QRCode represents a QRCode image.
type QRCode struct {
	Size uint8   `json:"size"`
	Data []uint8 `json:"data"`
}

// ESimProfile represents an eSIM profile and provides operations
// on the profile.
type ESimProfile struct {
	manager *ESimManager
	Iccid   string
}

// String16 represents a UTF-16 string.
type String16 struct {
	Data []uint16 `json:"data"`
}

// ESimProfileProperties are the properties of an eSIM profile object.
type ESimProfileProperties struct {
	Eid             string       `json:"eid"`
	Iccid           string       `json:"iccid"`
	Name            String16     `json:"name"`
	Nickname        String16     `json:"nickname"`
	ServiceProvider String16     `json:"serviceProvider"`
	State           ProfileState `json:"state"`
	ActivationCode  string       `json:"activationCode"`
}

/*
   Wrapper functions around eSIM mojo JS calls.
*/

// GetAvailableEuicc returns a list of Euiccs available on the device.
func (m *ESimManager) GetAvailableEuicc(ctx context.Context) ([]Euicc, error) {
	var result []string

	js := "function() {return this.getAvailableEuiccEids()}"
	if err := m.js.Call(ctx, &result, js); err != nil {
		return nil, errors.Wrap(err, "getAvailableEuiccs call failed")
	}

	euiccs := make([]Euicc, len(result))
	for i, id := range result {
		euiccs[i] = Euicc{manager: m, Eid: id}
	}

	return euiccs, nil
}

// GetProperties returns properties struct for this Euicc.
func (e *Euicc) GetProperties(ctx context.Context) (EuiccProperties, error) {
	var result EuiccProperties

	js := `function(eid) {return this.getEuiccProperties(eid)}`
	if err := e.manager.js.Call(ctx, &result, js, e.Eid); err != nil {
		return result, errors.Wrap(err, "getProperties call failed")
	}

	return result, nil
}

// GetProfileList returns a list of all profiles installed or pending on this Euicc.
func (e *Euicc) GetProfileList(ctx context.Context) ([]ESimProfile, error) {
	var result []string

	js := "function(eid) {return this.getProfileIccids(eid)}"
	if err := e.manager.js.Call(ctx, &result, js, e.Eid); err != nil {
		return nil, errors.Wrap(err, "getProfileIccids call failed")
	}

	profiles := make([]ESimProfile, len(result))
	for i, id := range result {
		profiles[i] = ESimProfile{manager: e.manager, Iccid: id}
	}

	return profiles, nil
}

// RequestPendingProfiles starts a request for pending profiles for this
// Euicc from SMDS. Returns a status indicating result of the operation.
func (e *Euicc) RequestPendingProfiles(ctx context.Context) (ESimOperationResult, error) {
	var result ESimOperationResult

	js := "function(eid) {return this.requestPendingProfiles(eid)}"
	if err := e.manager.js.Call(ctx, &result, js, e.Eid); err != nil {
		return result, errors.Wrap(err, "requestPendingProfiles call failed")
	}

	return result, nil
}

// InstallProfileFromActivationCode installs a profile with given activation_code
// and confirmation_code on this Euicc. Returns the  result code for the operation
func (e *Euicc) InstallProfileFromActivationCode(
	ctx context.Context,
	activationCode, confirmationCode string) (ProfileInstallResult, *ESimProfile, error) {
	var result struct {
		Result ProfileInstallResult
		Iccid  string
	}

	js := "function(eid, ac, cc) {return this.installProfileFromActivationCode(eid, ac, cc)}"
	if err := e.manager.js.Call(ctx, &result, js, e.Eid, activationCode, confirmationCode); err != nil {
		return result.Result, nil, errors.Wrap(err, "installProfileFromActivationCode call failed")
	}

	return result.Result, &ESimProfile{manager: e.manager, Iccid: result.Iccid}, nil
}

// GetEidQRCode returns a QR Code image representing the EID of this Euicc.
// A null value is returned if the QR Code could not be generated.
func (e *Euicc) GetEidQRCode(ctx context.Context) (QRCode, error) {
	var result QRCode

	js := "function(eid) {return this.getEidQrCode(eid)}"
	if err := e.manager.js.Call(ctx, &result, js, e.Eid); err != nil {
		return result, errors.Wrap(err, "getEidQrCode call failed")
	}

	return result, nil
}

// NewString16 creates a String16 from a string.
func NewString16(s string) String16 {
	return String16{Data: utf16.Encode([]rune(s))}
}

func (s String16) String() string {
	return string(utf16.Decode(s.Data))
}

// GetProperties returns properties struct for this ESimProfile.
func (e *ESimProfile) GetProperties(ctx context.Context) (ESimProfileProperties, error) {
	var result ESimProfileProperties

	js := "function(iccid) {return this.getProfileProperties(iccid)}"
	if err := e.manager.js.Call(ctx, &result, js, e.Iccid); err != nil {
		return result, errors.Wrap(err, "getProfileProperties call failed")
	}

	return result, nil
}

// InstallProfile installs this eSIM profile with given confirmationCode.
// A non success result code is returned in case of errors.
func (e *ESimProfile) InstallProfile(ctx context.Context, confirmationCode string) (ProfileInstallResult, error) {
	var result ProfileInstallResult

	js := "function(iccid, cc) {return this.installProfile(iccid, cc)}"
	if err := e.manager.js.Call(ctx, &result, js, e.Iccid, confirmationCode); err != nil {
		return result, errors.Wrap(err, "installProfile call failed")
	}

	return result, nil
}

// UninstallProfile uninstalls this eSIM profile. Returns the result code for the operation.
func (e *ESimProfile) UninstallProfile(ctx context.Context) (ESimOperationResult, error) {
	var result ESimOperationResult

	js := "function(iccid) {return this.uninstallProfile(iccid)}"
	if err := e.manager.js.Call(ctx, &result, js, e.Iccid); err != nil {
		return result, errors.Wrap(err, "uninstallProfile call failed")
	}

	return result, nil
}

// SetProfileNickname sets a nickname for this eSIM profile. Returns
// the result code for the operation.
func (e *ESimProfile) SetProfileNickname(ctx context.Context, nickname String16) (ESimOperationResult, error) {
	var result ESimOperationResult

	js := "function(iccid, name) {return this.setProfileNickname(iccid, name)}"
	if err := e.manager.js.Call(ctx, &result, js, e.Iccid, nickname); err != nil {
		return result, errors.Wrap(err, "setProfileNickname call failed")
	}

	return result, nil
}
