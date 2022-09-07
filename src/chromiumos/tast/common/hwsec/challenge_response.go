// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"crypto"
	"crypto/rsa"
	"reflect"

	"github.com/godbus/dbus/v5"
	"github.com/golang/protobuf/proto"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/errors"
)

// LogFunc represent the type of logging function, such as `s.Logf`.
type LogFunc func(string, ...interface{})

// CryptohomeKeyDelegate is a testing implementation of the
// CryptohomeKeyDelegate D-Bus object defined here:
// src/platform2/cryptohome/dbus_bindings/org.chromium.CryptohomeKeyDelegateInterface.xml .
// This D-Bus service is getting called by cryptohomed during the test.
type CryptohomeKeyDelegate struct {
	Lf               LogFunc
	DBusConn         *dbus.Conn
	DBusPath         string
	DBusIface        string
	User             string
	KeyAlgs          []cpb.ChallengeSignatureAlgorithm
	RsaKey           *rsa.PrivateKey
	PubKeySPKIDER    []byte
	ChallengeCallCnt int
}

// ChallengeKey handles the incoming ChallengeKey D-Bus call. It returns the
// KeyChallengeResponse proto with the challenge's signature calculated using
// the testing private key.
func (d *CryptohomeKeyDelegate) ChallengeKey(
	marshAccountID, marshChallReq []byte) (
	marshChallResp []byte, error *dbus.Error) {
	d.ChallengeCallCnt++
	localMarshChallResp, err := handleChallengeKey(
		d.User, d.KeyAlgs, d.RsaKey, d.PubKeySPKIDER, marshAccountID, marshChallReq)
	if err != nil {
		d.Lf("ChallengeKey handler failed: %s", err)
		return nil, dbus.MakeFailedError(err)
	}
	return localMarshChallResp, nil
}

// NewCryptohomeKeyDelegate creates CryptohomeKeyDelegate and exports this as a
// D-Bus service running on the given bus.
func NewCryptohomeKeyDelegate(
	lf LogFunc, dbusConn *dbus.Conn, testUser string,
	keyAlgs []cpb.ChallengeSignatureAlgorithm, rsaKey *rsa.PrivateKey,
	pubKeySPKIDER []byte) (*CryptohomeKeyDelegate, error) {
	const (
		dbusPath  = "/org/chromium/CryptohomeKeyDelegate"
		dbusIface = "org.chromium.CryptohomeKeyDelegateInterface"
	)
	keyDelegate := CryptohomeKeyDelegate{
		lf, dbusConn, dbusPath, dbusIface, testUser, keyAlgs, rsaKey, pubKeySPKIDER,
		0 /* ChallengeCallCnt */}
	if err := dbusConn.Export(&keyDelegate, dbusPath, dbusIface); err != nil {
		return nil, err
	}
	return &keyDelegate, nil
}

// Close unexports the CryptohomeKeyDelegate instance as a D-Bus object.
func (d *CryptohomeKeyDelegate) Close() {
	d.DBusConn.Export(nil, dbus.ObjectPath(d.DBusPath), d.DBusIface)
}

func isExpectedKeyAlg(alg cpb.ChallengeSignatureAlgorithm, expectedAlgs []cpb.ChallengeSignatureAlgorithm) bool {
	for _, candidate := range expectedAlgs {
		if candidate == alg {
			return true
		}
	}
	return false
}

// handleChallengeKey is the actual implementation of the ChallengeKey D-Bus.
func handleChallengeKey(
	testUser string, keyAlgs []cpb.ChallengeSignatureAlgorithm,
	rsaKey *rsa.PrivateKey, pubKeySPKIDER, marshAccountID, marshChallReq []byte) (
	marshChallResp []byte, err error) {
	var accountID cpb.AccountIdentifier
	if err := proto.Unmarshal(marshAccountID, &accountID); err != nil {
		return nil, errors.Wrap(err, "failed unmarshaling AccountIdentifier")
	}
	var challReq cpb.KeyChallengeRequest
	if err := proto.Unmarshal(marshChallReq, &challReq); err != nil {
		return nil, errors.Wrap(err, "failed unmarshaling KeyChallengeRequest")
	}
	if accountID.AccountId == nil {
		return nil, errors.New("missing account_id")
	}
	if *accountID.AccountId != testUser {
		return nil, errors.Errorf("wrong account_id: expected %q, got %q", testUser, *accountID.AccountId)
	}
	if challReq.ChallengeType == nil ||
		*challReq.ChallengeType != cpb.KeyChallengeRequest_CHALLENGE_TYPE_SIGNATURE {
		return nil, errors.Errorf("wrong challenge_type: %s", challReq.ChallengeType)
	}
	sigReqData := challReq.SignatureRequestData
	if sigReqData == nil {
		return nil, errors.New("missing signature_request_data")
	}
	if sigReqData.DataToSign == nil {
		return nil, errors.New("missing data_to_sign")
	}
	if sigReqData.PublicKeySpkiDer == nil ||
		!reflect.DeepEqual(sigReqData.PublicKeySpkiDer, pubKeySPKIDER) {
		return nil, errors.Errorf("bad public_key_spki_der: expected %s, got %s", pubKeySPKIDER,
			sigReqData.PublicKeySpkiDer)
	}
	if sigReqData.SignatureAlgorithm == nil ||
		!isExpectedKeyAlg(*sigReqData.SignatureAlgorithm, keyAlgs) {
		return nil, errors.Errorf("wrong signature_algorithm: expected one of %s, got %s",
			keyAlgs, sigReqData.SignatureAlgorithm)
	}
	hashFunction, err := getHashFunction(*sigReqData.SignatureAlgorithm)
	hash := hashFunction.New()
	hash.Write(sigReqData.DataToSign)
	sig, err := rsa.SignPKCS1v15(nil, rsaKey, hashFunction, hash.Sum(nil))
	if err != nil {
		return nil, errors.Wrap(err, "failed generating signature")
	}
	localMarshChallResp, err := proto.Marshal(
		&cpb.KeyChallengeResponse{
			SignatureResponseData: &cpb.SignatureKeyChallengeResponseData{
				Signature: sig,
			},
		})
	if err != nil {
		return nil, errors.Wrap(err, "failed marshaling KeyChallengeResponse")
	}
	return localMarshChallResp, nil
}

// getHashFunction returns the hash function to be used for the given challenge algorithm.
func getHashFunction(keyAlg cpb.ChallengeSignatureAlgorithm) (crypto.Hash, error) {
	switch keyAlg {
	case cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1:
		return crypto.SHA1, nil
	case cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA256:
		return crypto.SHA256, nil
	case cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA384:
		return crypto.SHA384, nil
	case cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA512:
		return crypto.SHA512, nil
	}
	return crypto.SHA1, errors.Errorf("unexpected key algorithm %v", keyAlg)
}
