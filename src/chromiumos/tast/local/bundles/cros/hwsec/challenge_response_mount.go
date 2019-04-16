// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"math/rand"
	"reflect"

	"github.com/godbus/dbus"
	"github.com/golang/protobuf/proto"

	cpb "chromiumos/system_api/cryptohome_proto"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChallengeResponseMount,
		Desc: "Checks that the cryptohome challenge-response mount works",
		Contacts: []string{
			"emaxx@chromium.org", // Test author
			"cros-hwsec@google.com",
		},
		Attr: []string{"informational"},
	})
}

// callCryptohomeMountEx calls the MountEx cryptohomed D-Bus method.
func callCryptohomeMountEx(
	ctx context.Context, cryptohomeObj dbus.BusObject, accountID string,
	authReq cpb.AuthorizationRequest, mountReq cpb.MountRequest) error {
	marshAccountID, err := proto.Marshal(
		&cpb.AccountIdentifier{
			AccountId: &accountID,
		})
	if err != nil {
		return errors.Wrap(err, "failed marshaling AccountIdentifier")
	}
	marshAuthReq, err := proto.Marshal(&authReq)
	if err != nil {
		return errors.Wrap(err, "failed marshaling AuthorizationRequest")
	}
	marshMountReq, err := proto.Marshal(&mountReq)
	if err != nil {
		return errors.Wrap(err, "failed marshaling MountRequest")
	}
	call := cryptohomeObj.CallWithContext(
		ctx, "org.chromium.CryptohomeInterface.MountEx", 0, marshAccountID,
		marshAuthReq, marshMountReq)
	if call.Err != nil {
		return errors.Wrap(call.Err, "failed calling cryptohomed MountEx")
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

// handleChallengeKey handles the incoming ChallengeKey D-Bus call. It returns
// the KeyChallengeResponse proto with the challenge's signature calculated
// using the testing private key.
func handleChallengeKey(
	ctx context.Context, testUser string, keyAlg cpb.ChallengeSignatureAlgorithm,
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
	if accountID.AccountId == nil || *accountID.AccountId != testUser {
		return nil, errors.Errorf(
			"wrong account_id: expected %s, got %s", testUser, accountID.AccountId)
	}
	if challReq.ChallengeType == nil ||
		*challReq.ChallengeType !=
			cpb.KeyChallengeRequest_CHALLENGE_TYPE_SIGNATURE {
		return nil, errors.Errorf(
			"wrong challenge_type: %s", challReq.ChallengeType)
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
		return nil, errors.Errorf(
			"bad public_key_spki_der: expected %s, got %s", pubKeySPKIDER,
			sigReqData.PublicKeySpkiDer)
	}
	if sigReqData.SignatureAlgorithm == nil ||
		*sigReqData.SignatureAlgorithm != keyAlg {
		return nil, errors.Errorf(
			"wrong signature_algorithm: expected %s, got %s", keyAlg,
			sigReqData.SignatureAlgorithm)
	}
	dataToSignHash := sha1.Sum(sigReqData.DataToSign)
	sig, err := rsa.SignPKCS1v15(nil, rsaKey, crypto.SHA1, dataToSignHash[:])
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
		return nil, errors.Wrap(err, "failed marshaling AuthorizationRequest")
	}
	return localMarshChallResp, nil
}

func ChallengeResponseMount(ctx context.Context, s *testing.State) {
	const (
		testUser    = "cryptohome_test@chromium.org"
		keyLabel    = "testkey"
		keySizeBits = 2048
		keyAlg      = cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1
	)

	// Make sure the test starts from a missing cryptohome.
	cryptohome.RemoveUserDir(ctx, testUser)
	// Clean up the cryptohome created by this test, if any, during shutdown.
	defer cryptohome.RemoveUserDir(ctx, testUser)
	defer cryptohome.UnmountVault(ctx, testUser)

	// Use a pseudorandom generator with a fixed seed, to make the values used by
	// the test predictable.
	randReader := rand.New(rand.NewSource(0))

	rsaKey, err := rsa.GenerateKey(randReader, keySizeBits)
	if err != nil {
		s.Fatalf("Failed to generate RSA key: %s", err)
	}
	pubKeySPKIDER, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		s.Fatalf("Failed to generate SubjectPublicKeyInfo: %s", err)
	}

	dbusConn, err := dbus.SystemBus()
	if err != nil {
		s.Fatalf("Failed to connect to system D-Bus bus: %s", err)
	}
	selfDbusObjName := dbusConn.Names()[0]

	dbusConn.ExportMethodTable(
		map[string]interface{}{
			"ChallengeKey": func(marshAccountId, marshChallReq []byte) (
				marshChallResp []byte, error *dbus.Error) {
				localMarschChallResp, err := handleChallengeKey(
					ctx, testUser, keyAlg, rsaKey, pubKeySPKIDER, marshAccountId,
					marshChallReq)
				if err != nil {
					return nil, dbus.MakeFailedError(err)
				}
				return localMarschChallResp, nil
			},
		}, "/org/chromium/CryptohomeKeyDelegate",
		"org.chromium.CryptohomeKeyDelegateInterface")

	_, cryptohomeObj, err := dbusutil.Connect(
		ctx, "org.chromium.Cryptohome", dbus.ObjectPath("/org/chromium/Cryptohome"))
	if err != nil {
		s.Fatalf("Failed to connect to cryptohome D-Bus object: %s", err)
	}

	// Create the challenge-response protected cryptohome.
	keyType := cpb.KeyData_KEY_TYPE_CHALLENGE_RESPONSE
	localKeyLabel := keyLabel
	keyDelegateDbusObjPath := "/org/chromium/CryptohomeKeyDelegate"
	authReq := cpb.AuthorizationRequest{
		Key: &cpb.Key{
			Data: &cpb.KeyData{
				Type:  &keyType,
				Label: &localKeyLabel,
				ChallengeResponseKey: []*cpb.ChallengePublicKeyInfo{
					&cpb.ChallengePublicKeyInfo{
						PublicKeySpkiDer: pubKeySPKIDER,
						SignatureAlgorithm: []cpb.ChallengeSignatureAlgorithm{
							keyAlg,
						},
					},
				},
			},
		},
		KeyDelegate: &cpb.KeyDelegate{
			DbusServiceName: &selfDbusObjName,
			DbusObjectPath:  &keyDelegateDbusObjPath,
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
		s.Fatalf("Failed to create cryptohome: %s", err)
	}

	if err := cryptohome.UnmountVault(ctx, testUser); err != nil {
		s.Fatalf("Failed to unmount cryptohome: %s", err)
	}

	// Mount the existing challenge-response protected cryptohome.
	mountReq.Create = nil
	if err := callCryptohomeMountEx(
		ctx, cryptohomeObj, testUser, authReq, mountReq); err != nil {
		s.Fatalf("Failed to mount existing cryptohome: %s", err)
	}
}
