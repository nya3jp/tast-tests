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

// cryptohomeKeyDelegate is a testing implementation of the
// CryptohomeKeyDelegate D-Bus object defined here:
// src/platform2/cryptohome/dbus_bindings/org.chromium.CryptohomeInterface.xml
type cryptohomeKeyDelegate struct {
	testingState  *testing.State
	user          string
	keyAlg        cpb.ChallengeSignatureAlgorithm
	rsaKey        *rsa.PrivateKey
	pubKeySPKIDER []byte
}

// ChallengeKey handles the incoming ChallengeKey D-Bus call. It returns the
// KeyChallengeResponse proto with the challenge's signature calculated using
// the testing private key.
func (d *cryptohomeKeyDelegate) ChallengeKey(
	marshAccountID, marshChallReq []byte) (
	marshChallResp []byte, error *dbus.Error) {
	localMarshChallResp, err := handleChallengeKey(
		d.user, d.keyAlg, d.rsaKey, d.pubKeySPKIDER, marshAccountID, marshChallReq)
	if err != nil {
		d.testingState.Logf("ChallengeKey handler failed: %s", err)
		return nil, dbus.MakeFailedError(err)
	}
	return localMarshChallResp, nil
}

// handleChallengeKey is the actual implementation of the ChallengeKey D-Bus.
func handleChallengeKey(
	testUser string, keyAlg cpb.ChallengeSignatureAlgorithm,
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
		*challReq.ChallengeType != cpb.KeyChallengeRequest_CHALLENGE_TYPE_SIGNATURE {
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
		return nil, errors.Wrap(err, "failed marshaling KeyChallengeResponse")
	}
	return localMarshChallResp, nil
}

func ChallengeResponseMount(ctx context.Context, s *testing.State) {
	const (
		testUser             = "cryptohome_test@chromium.org"
		keyLabel             = "testkey"
		keySizeBits          = 2048
		keyAlg               = cpb.ChallengeSignatureAlgorithm_CHALLENGE_RSASSA_PKCS1_V1_5_SHA1
		keyDelegateDbusPath  = "/org/chromium/CryptohomeKeyDelegate"
		keyDelegateDbusIface = "org.chromium.CryptohomeKeyDelegateInterface"
	)

	// Make sure the test starts from a missing cryptohome.
	cryptohome.RemoveUserDir(ctx, testUser)
	// Clean up the cryptohome created by this test, if any, during shutdown.
	defer cryptohome.RemoveUserDir(ctx, testUser)
	defer cryptohome.UnmountVault(ctx, testUser)

	// Use a pseudorandom generator with a fixed seed, to make the values used by
	// the test predictable.
	randReader := rand.New(rand.NewSource(0 /* seed */))

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

	keyDelegate := cryptohomeKeyDelegate{s, testUser, keyAlg, rsaKey, pubKeySPKIDER}
	if err := dbusConn.Export(
		&keyDelegate, keyDelegateDbusPath, keyDelegateDbusIface); err != nil {
		s.Fatalf("Failed to export D-Bus key delegate: %s", err)
	}
	defer dbusConn.Export(nil, keyDelegateDbusPath, keyDelegateDbusIface)

	cryptohomeDbus, err := cryptohome.NewDbus(ctx)
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
	if err := cryptohomeDbus.Mount(ctx, testUser, authReq, mountReq); err != nil {
		s.Fatalf("Failed to create cryptohome: %s", err)
	}

	if err := cryptohome.UnmountVault(ctx, testUser); err != nil {
		s.Fatalf("Failed to unmount cryptohome: %s", err)
	}

	// Mount the existing challenge-response protected cryptohome.
	mountReq.Create = nil
	if err := cryptohomeDbus.Mount(
		ctx, testUser, authReq, mountReq); err != nil {
		s.Fatalf("Failed to mount existing cryptohome: %s", err)
	}
}
