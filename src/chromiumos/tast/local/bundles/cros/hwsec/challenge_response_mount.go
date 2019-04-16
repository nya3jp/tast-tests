// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"reflect"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

const (
	testUser    = "cryptohome_test@chromium.org"
	keyLabel    = "testkey"
	keySizeBits = 2048
	keyAlg      = cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChallengeResponseMount,
		Desc: "Checks that the cryptohome challenge-response mount works",
		Contacts: []string{
			"emaxx@chromium.org",
		},
		Attr: []string{"informational"},
	})
}

func callCryptohomeMountEx(
	ctx context.Context,
	cryptohomeObj dbus.BusObject,
	accountID string,
	authReq cpb.AuthorizationRequest,
	mountReq cpb.MountRequest) error {
	marshAccountID, err := proto.Marshal(
		&cpb.AccountIdentifier{
			AccountId: &accountID,
		})
	if err != nil {
		return errors.Wrap(err, "failed marshaling AccountIdentifier proto")
	}
	marshAuthReq, err := proto.Marshal(&authReq)
	if err != nil {
		return errors.Wrap(err, "failed marshaling AuthorizationRequest proto")
	}
	marshMountReq, err := proto.Marshal(&mountReq)
	if err != nil {
		return errors.Wrap(err, "failed marshaling MountRequest proto")
	}
	call := cryptohomeObj.CallWithContext(
		ctx, "org.chromium.CryptohomeInterface.MountEx", 0, marshAccountID,
		marshAuthReq, marshMountReq)
	if call.Err != nil {
		return errors.Wrap(
			call.Err,
			"failed calling org.chromium.CryptohomeKeyDelegateInterface.MountEx")
	}
	var marshMountReply []byte
	if err := call.Store(&marshMountReply); err != nil {
		return errors.Wrap(err, "failed reading BaseReply")
	}
	var mountReply cpb.BaseReply
	if err := proto.Unmarshal(marshMountReply, &mountReply); err != nil {
		return errors.Wrap(err, "failed unmarshaling BaseReply")
	}
	if mountReply.Error != nil {
		return errors.Errorf("MountEx call failed with %s", mountReply.Error)
	}
	return nil
}

func handleChallengeKey(
	ctx context.Context, s *testing.State, rsaKey *rsa.PrivateKey,
	pubKeySpkiDer, marshAccountID, marshChallReq []byte) (
	marshChallResp []byte, error *dbus.Error) {
	var accountID cpb.AccountIdentifier
	if err := proto.Unmarshal(marshAccountID, &accountID); err != nil {
		return nil, dbus.MakeFailedError(errors.Wrap(
			err, "failed unmarshaling AccountIdentifier"))
	}
	var challReq cpb.KeyChallengeRequest
	if err := proto.Unmarshal(marshChallReq, &challReq); err != nil {
		return nil, dbus.MakeFailedError(errors.Wrap(
			err, "failed unmarshaling KeyChallengeRequest"))
	}
	if accountID.AccountId == nil || *accountID.AccountId != testUser {
		return nil, dbus.MakeFailedError(errors.Errorf(
			"Wrong account_id: expected %s, got %s", testUser, accountID.AccountId))
	}
	if challReq.ChallengeType == nil ||
		*challReq.ChallengeType !=
			cpb.KeyChallengeRequest_CHALLENGE_TYPE_SIGNATURE {
		return nil, dbus.MakeFailedError(errors.Errorf(
			"Wrong challenge_type: %s", challReq.ChallengeType))
	}
	sigReqData := challReq.SignatureRequestData
	if sigReqData == nil {
		return nil, dbus.MakeFailedError(errors.New(
			"Missing signature_request_data"))
	}
	if sigReqData.DataToSign == nil {
		return nil, dbus.MakeFailedError(errors.Errorf(
			"Bad data_to_sign: %s", sigReqData.DataToSign))
	}
	if sigReqData.PublicKeySpkiDer == nil ||
		!reflect.DeepEqual(sigReqData.PublicKeySpkiDer, pubKeySpkiDer) {
		return nil, dbus.MakeFailedError(errors.Errorf(
			"Bad public_key_spki_der: %s", sigReqData.PublicKeySpkiDer))
	}
	if sigReqData.SignatureAlgorithm == nil ||
		*sigReqData.SignatureAlgorithm != keyAlg {
		return nil, dbus.MakeFailedError(errors.Errorf(
			"Wrong signature_algorithm: expected %s, got %s", keyAlg,
			sigReqData.SignatureAlgorithm))
	}
	dataToSignHash := sha1.Sum(sigReqData.DataToSign)
	sig, err := rsa.SignPKCS1v15(
		rand.Reader, rsaKey, crypto.SHA1, dataToSignHash[:])
	if err != nil {
		return nil, dbus.MakeFailedError(errors.Wrap(
			err, "Failed to generate signature"))
	}
	localMarshChallResp, err := proto.Marshal(
		&cpb.KeyChallengeResponse{
			SignatureResponseData: &cpb.SignatureKeyChallengeResponseData{
				Signature: sig,
			},
		})
	if err != nil {
		return nil, dbus.MakeFailedError(errors.Wrap(
			err, "failed marshaling AuthorizationRequest proto"))
	}
	return localMarshChallResp, nil
}

func ChallengeResponseMount(ctx context.Context, s *testing.State) {
	cryptohome.RemoveUserDir(ctx, testUser)
	defer cryptohome.RemoveUserDir(ctx, testUser)

	rsaKey, err := rsa.GenerateKey(rand.Reader, keySizeBits)
	if err != nil {
		s.Fatalf("Failed to generate RSA key: %s", err)
	}
	pubKeySpkiDer, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		s.Fatalf("Failed to generate SubjectPublicKeyInfo: %s", err)
	}

	dbusConn, err := dbus.SystemBus()
	if err != nil {
		s.Fatalf("Failed to connect to system D-Bus bus: %s", err)
	}
	selfDbusObjName := dbusConn.Names()[0]

	exportedMethods := make(map[string]interface{})
	exportedMethods["ChallengeKey"] = func(
		marshAccountId, marshChallReq []byte) (
		challengeResponse []byte, error *dbus.Error) {
		return handleChallengeKey(ctx, s, rsaKey, pubKeySpkiDer, marshAccountId,
			marshChallReq)
	}
	dbusConn.ExportMethodTable(
		exportedMethods, "/org/chromium/CryptohomeKeyDelegate",
		"org.chromium.CryptohomeKeyDelegateInterface")

	_, cryptohomeObj, err := dbusutil.Connect(
		ctx, "org.chromium.Cryptohome",
		dbus.ObjectPath("/org/chromium/Cryptohome"))
	if err != nil {
		s.Fatalf("Failed to connect to cryptohome D-Bus object: %s", err)
	}

	// Create the challenge-response protected cryptohome.
	keyType := cpb.KeyData_KEY_TYPE_CHALLENGE_RESPONSE
	localKeyLabel := keyLabel
	keyDelegateDbusObjectPath := "/org/chromium/CryptohomeKeyDelegate"
	authReq := cpb.AuthorizationRequest{
		Key: &cpb.Key{
			Data: &cpb.KeyData{
				Type:  &keyType,
				Label: &localKeyLabel,
				ChallengeResponseKey: []*cpb.ChallengePublicKeyInfo{
					&cpb.ChallengePublicKeyInfo{
						PublicKeySpkiDer: pubKeySpkiDer,
						SignatureAlgorithm: []cpb.ChallengeSignatureAlgorithm{
							keyAlg,
						},
					},
				},
			},
		},
		KeyDelegate: &cpb.KeyDelegate{
			DbusServiceName: &selfDbusObjName,
			DbusObjectPath:  &keyDelegateDbusObjectPath,
		},
	}
	copyAuthKey := true
	mountReq := cpb.MountRequest{
		Create: &cpb.CreateRequest{
			CopyAuthorizationKey: &copyAuthKey,
		},
	}
	if err := callCryptohomeMountEx(
		ctx, cryptohomeObj, testUser, authReq, mountReq); err != nil {
		s.Fatalf("Failed to call MountEx cryptohome D-Bus method: %s", err)
	}

	cryptohome.UnmountVault(ctx, testUser)

	// Mount the existing challenge-response protected cryptohome.
	mountReq.Create = nil
	if err := callCryptohomeMountEx(
		ctx, cryptohomeObj, testUser, authReq, mountReq); err != nil {
		s.Fatalf("Failed to call MountEx cryptohome D-Bus method: %s", err)
	}

	cryptohome.UnmountVault(ctx, testUser)
}
