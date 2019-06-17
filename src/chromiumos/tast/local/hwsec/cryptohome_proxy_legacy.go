// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"

	"github.com/godbus/dbus"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
)

const (
	dbusName      = "org.chromium.Cryptohome"
	dbusPath      = "/org/chromium/Cryptohome"
	dbusInterface = "org.chromium.CryptohomeInterface"
)

// CryptohomeProxyLegacy is used to interact with the cryptohomed process over
// legacy D-Bus interface.  For detailed spec of each D-Bus method, please find
// src/platform2/cryptohome/dbus_bindings/org.chromium.CryptohomeInterface.xml.
// The function names are consistent with the ones in the XML files.
// Additionally, syncrhonous calls to the asynchronous interfaces are suffixed
// with 'Sync'.
type CryptohomeProxyLegacy struct {
	ctx  context.Context
	conn *dbus.Conn
	obj  dbus.BusObject
}

// NewCryptohomeProxyLegacy is a factory function to create a
// CryptohomeProxyLegacy instance.
func NewCryptohomeProxyLegacy(ctx context.Context) (*CryptohomeProxyLegacy, error) {
	conn, obj, err := dbusutil.Connect(ctx, dbusName, dbusPath)
	if err != nil {
		return nil, err
	}
	return &CryptohomeProxyLegacy{ctx, conn, obj}, nil
}

// TpmIsReady calls |TpmisReady| legacy cryptohome interface.
func (c *CryptohomeProxyLegacy) TpmIsReady() (bool, error) {
	call := c.obj.CallWithContext(c.ctx, "org.chromium.CryptohomeInterface.TpmIsReady", 0)
	if call.Err != nil {
		return false, errors.Wrap(call.Err, "failed calling to cryptohomed TpmIsReady")
	}
	var result bool
	if err := call.Store(&result); err != nil {
		return false, errors.Wrap(err, "failed to read dbus response")
	}
	return result, nil
}

// TpmIsAttestationPrepared calls |TpmIsAttestationPrepared| legacy
// cryptohome interface.
func (c *CryptohomeProxyLegacy) TpmIsAttestationPrepared() (bool, error) {
	call := c.obj.CallWithContext(
		c.ctx,
		"org.chromium.CryptohomeInterface.TpmIsAttestationPrepared",
		0)
	if call.Err != nil {
		return false, errors.Wrap(call.Err, "failed calling to TpmIsAttestationPrepared")
	}
	var result bool
	if err := call.Store(&result); err != nil {
		return false, errors.Wrap(err, "failed to read dbus response")
	}
	return result, nil
}

// TpmIsAttestationEnrolled calls |TpmIsAttestationEnrolled| legacy
// cryptohome interface.
func (c *CryptohomeProxyLegacy) TpmIsAttestationEnrolled() (bool, error) {
	call := c.obj.CallWithContext(
		c.ctx, "org.chromium.CryptohomeInterface.TpmIsAttestationEnrolled", 0)
	if call.Err != nil {
		return false, errors.Wrap(call.Err, "failed calling to TpmIsAttestationEnrolled")
	}
	var result bool
	if err := call.Store(&result); err != nil {
		return false, errors.Wrap(err, "failed to read dbus response")
	}
	return result, nil
}

// TpmCanAttemptOwnership calls |TpmCanAttemptOwnership| legacy cryptohome interface.
func (c *CryptohomeProxyLegacy) TpmCanAttemptOwnership() error {
	call := c.obj.CallWithContext(c.ctx, "org.chromium.CryptohomeInterface.TpmCanAttemptOwnership", 0)
	if call.Err != nil {
		return errors.Wrap(call.Err, "failed calling to cryptohomed TpmCanAttemptOwnership")
	}
	var result bool
	if err := call.Store(&result); err != nil {
		return errors.Wrap(err, "failed to read dbus response")
	}
	return nil
}

// TpmAttestationCreateEnrollRequest calls |TpmAttestationCreateEnrollRequest|
// legacy cryptohome interface.
func (c *CryptohomeProxyLegacy) TpmAttestationCreateEnrollRequest(PCAType int) (string, error) {
	call := c.obj.CallWithContext(c.ctx, "org.chromium.CryptohomeInterface.TpmAttestationCreateEnrollRequest", 0, PCAType)
	if call.Err != nil {
		return "", errors.Wrap(call.Err, "failed calling to cryptohomed TpmAttestationCreateEnrollRequest")
	}
	var resultRequest []uint8
	if err := call.Store(&resultRequest); err != nil {
		return "", errors.Wrap(err, "failed to read dbus response")
	}
	return string(resultRequest), nil
}

// TpmAttestationEnroll calls |TpmAttestationEnroll| legacy cryptohome interface.
func (c *CryptohomeProxyLegacy) TpmAttestationEnroll(PCAType int, resp string) (bool, error) {
	call := c.obj.CallWithContext(c.ctx, "org.chromium.CryptohomeInterface.TpmAttestationEnroll", 0, PCAType, []uint8(resp))
	if call.Err != nil {
		return false, errors.Wrap(call.Err, "failed calling to cryptohomed TpmAttestationEnroll")
	}
	var result bool
	if err := call.Store(&result); err != nil {
		return false, errors.Wrap(err, "failed to read dbus response")
	}
	return result, nil
}

// TpmAttestationCreateCertRequest calls |TpmAttestationCreateCertRequest| legacy cryptohome interface.
func (c *CryptohomeProxyLegacy) TpmAttestationCreateCertRequest(
	PCAType int,
	profile int,
	username string,
	origin string) (string, error) {
	call := c.obj.CallWithContext(c.ctx,
		"org.chromium.CryptohomeInterface.TpmAttestationCreateCertRequest",
		0,
		PCAType,
		profile,
		username,
		origin)
	if call.Err != nil {
		return "", errors.Wrap(call.Err, "failed calling to cryptohomed TpmAttestationCreateCertRequest")
	}
	var resultRequest []uint8
	if err := call.Store(&resultRequest); err != nil {
		return "", errors.Wrap(err, "failed to read dbus response")
	}
	return string(resultRequest), nil
}

// TpmAttestationFinishCertRequest calls |TpmAttestationFinishCertRequest| legacy cryptohome interface.
func (c *CryptohomeProxyLegacy) TpmAttestationFinishCertRequest(
	resp string,
	isUserSpecific bool,
	username string,
	label string) (string, error) {
	call := c.obj.CallWithContext(
		c.ctx,
		"org.chromium.CryptohomeInterface.TpmAttestationFinishCertRequest",
		0,
		[]uint8(resp),
		isUserSpecific,
		username,
		label)
	if call.Err != nil {
		return "", errors.Wrap(call.Err, "failed calling to cryptohomed TpmAttestationFinishCertRequest")
	}
	var resultCert []uint8
	var resultSuccessful bool
	if err := call.Store(&resultCert, &resultSuccessful); err != nil {
		return "", errors.Wrap(err, "failed to read dbus response")
	}
	if resultSuccessful == false {
		return "", errors.New("failed to finish the certiticate")
	}
	return string(resultCert), nil
}

// TpmAttestationSignEnterpriseVaChallenge calls
// |TpmAttestationSignEnterpriseVaChallenge| legacy cryptohome interface.
func (c *CryptohomeProxyLegacy) TpmAttestationSignEnterpriseVaChallenge(
	VAType int,
	isUserSpecific bool,
	username string,
	label string,
	domain string,
	deviceID string,
	includeSignedPublicKey bool,
	challenge []byte) (int, error) {
	call := c.obj.CallWithContext(
		c.ctx,
		"org.chromium.CryptohomeInterface.TpmAttestationSignEnterpriseVaChallenge",
		0,
		VAType,
		isUserSpecific,
		username,
		label,
		domain,
		[]uint8(deviceID),
		includeSignedPublicKey,
		challenge)
	if call.Err != nil {
		return -1, errors.Wrap(call.Err, "failed calling to cryptohomed TpmAttestationSignEnterpriseVaChallenge")
	}
	var asyncID int
	if err := call.Store(&asyncID); err != nil {
		return -1, errors.Wrap(err, "failed to read dbus response")
	}
	return asyncID, nil
}

// TpmAttestationSignEnterpriseVaChallengeSync calls
// |TpmAttestationSignEnterpriseVaChallenge| and returns the result
// corresponding to the returned result id.
func (c *CryptohomeProxyLegacy) TpmAttestationSignEnterpriseVaChallengeSync(
	VAType int,
	isUserSpecific bool,
	username string,
	label string,
	domain string,
	deviceID string,
	includeSignedPublicKey bool,
	challenge []byte) (string, error) {
	spec := dbusutil.MatchSpec{
		Type:      "signal",
		Path:      dbusPath,
		Interface: dbusInterface,
		Member:    "AsyncCallStatusWithData",
	}
	// use embedded function to close the context ASAP.
	asyncID, signal, err := func() (int, *dbus.Signal, error) {
		sw, err := dbusutil.NewSignalWatcher(c.ctx, c.conn, spec)
		if err != nil {
			return -1, nil, errors.Wrap(err, "failed call to dbusutil.NewSignalWatcher")
		}
		defer sw.Close(c.ctx)
		asyncID, err := c.TpmAttestationSignEnterpriseVaChallenge(
			VAType,
			isUserSpecific,
			username,
			label,
			domain,
			deviceID,
			includeSignedPublicKey,
			challenge)
		if err != nil {
			return -1, nil, err
		}
		for {
			select {
			case sig := <-sw.Signals:
				return asyncID, sig, nil
			case <-c.ctx.Done():
				return -1, nil, c.ctx.Err()
			}
		}
	}()
	if err != nil {
		return "", errors.Wrap(err, "failed to get result from async call")
	}
	if signalAsyncID, ok := signal.Body[0].(int32); !ok {
		return "", errors.Wrap(err, "failed to slice the async id from signal body")
	} else if asyncID != int(signalAsyncID) {
		return "", errors.New("mismatched async id")
	} else if returnedStatus, ok := signal.Body[1].(bool); !ok {
		return "", errors.Wrap(err, "failed to slice the return status from signal body")
	} else if returnedStatus == false {
		return "", errors.New("failed to sign challenge")
	} else if signedChallenge, ok := signal.Body[2].([]uint8); !ok {
		return "", errors.Wrap(err, "failed to slice the signed challenge from signal body")
	} else {
		return string(signedChallenge), nil
	}
}
