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
	"chromiumos/tast/local/session"
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
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"tpm"},
	})
}

type logFunc func(string, ...interface{})

// cryptohomeKeyDelegate is a testing implementation of the
// CryptohomeKeyDelegate D-Bus object defined here:
// src/platform2/cryptohome/dbus_bindings/org.chromium.CryptohomeKeyDelegateInterface.xml .
// This D-Bus service is getting called by cryptohomed during the test.
type cryptohomeKeyDelegate struct {
	lf               logFunc
	dbusConn         *dbus.Conn
	dbusPath         string
	dbusIface        string
	user             string
	keyAlg           cpb.ChallengeSignatureAlgorithm
	rsaKey           *rsa.PrivateKey
	pubKeySPKIDER    []byte
	challengeCallCnt int
}

// ChallengeKey handles the incoming ChallengeKey D-Bus call. It returns the
// KeyChallengeResponse proto with the challenge's signature calculated using
// the testing private key.
func (d *cryptohomeKeyDelegate) ChallengeKey(
	marshAccountID, marshChallReq []byte) (
	marshChallResp []byte, error *dbus.Error) {
	d.challengeCallCnt++
	localMarshChallResp, err := handleChallengeKey(
		d.user, d.keyAlg, d.rsaKey, d.pubKeySPKIDER, marshAccountID, marshChallReq)
	if err != nil {
		d.lf("ChallengeKey handler failed: %s", err)
		return nil, dbus.MakeFailedError(err)
	}
	return localMarshChallResp, nil
}

// newCryptohomeKeyDelegate creates cryptohomeKeyDelegate and exports this as a
// D-Bus service running on the given bus.
func newCryptohomeKeyDelegate(
	lf logFunc, dbusConn *dbus.Conn, testUser string,
	keyAlg cpb.ChallengeSignatureAlgorithm, rsaKey *rsa.PrivateKey,
	pubKeySPKIDER []byte) (*cryptohomeKeyDelegate, error) {
	const (
		dbusPath  = "/org/chromium/CryptohomeKeyDelegate"
		dbusIface = "org.chromium.CryptohomeKeyDelegateInterface"
	)
	keyDelegate := cryptohomeKeyDelegate{
		lf, dbusConn, dbusPath, dbusIface, testUser, keyAlg, rsaKey, pubKeySPKIDER,
		0 /* challengeCallCnt */}
	if err := dbusConn.Export(&keyDelegate, dbusPath, dbusIface); err != nil {
		return nil, err
	}
	return &keyDelegate, nil
}

// close unexports the cryptohomeKeyDelegate instance as a D-Bus object.
func (d *cryptohomeKeyDelegate) close() {
	d.dbusConn.Export(nil, dbus.ObjectPath(d.dbusPath), d.dbusIface)
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
	if accountID.AccountId == nil {
		return nil, errors.New("missing account_id")
	}
	if *accountID.AccountId != testUser {
		return nil, errors.Errorf(
			"wrong account_id: expected %q, got %q", testUser, *accountID.AccountId)
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

func clearDevicePolicy(ctx context.Context) error {
	if err := session.SetUpDevice(ctx); err != nil {
		return errors.Wrap(err, "failed resetting device ownership")
	}
	return nil
}

func ChallengeResponseMount(ctx context.Context, s *testing.State) {
	const (
		dbusName    = "org.chromium.TestingCryptohomeKeyDelegate"
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
	randReader := rand.New(rand.NewSource(0 /* seed */))

	// Make sure the ephemeral users device policy is not set.
	// TODO(crbug.com/1054004); Remove after Tast starts to guarantee that.
	err := clearDevicePolicy(ctx)
	if err != nil {
		s.Fatal("Failed to clear device policy: ", err)
	}

	rsaKey, err := rsa.GenerateKey(randReader, keySizeBits)
	if err != nil {
		s.Fatal("Failed to generate RSA key: ", err)
	}
	pubKeySPKIDER, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		s.Fatal("Failed to generate SubjectPublicKeyInfo: ", err)
	}

	dbusConn, err := dbus.SystemBus()
	if err != nil {
		s.Fatal("Failed to connect to system D-Bus bus: ", err)
	}
	if _, err := dbusConn.RequestName(dbusName, 0 /* flags */); err != nil {
		s.Fatal("Failed to request the well-known D-Bus name: ", err)
	}
	defer dbusConn.ReleaseName(dbusName)

	keyDelegate, err := newCryptohomeKeyDelegate(
		s.Logf, dbusConn, testUser, keyAlg, rsaKey, pubKeySPKIDER)
	if err != nil {
		s.Fatal("Failed to export D-Bus key delegate: ", err)
	}
	defer keyDelegate.close()

	cryptohomeClient, err := cryptohome.NewClient(ctx)
	if err != nil {
		s.Fatal("Failed to connect to cryptohome D-Bus object: ", err)
	}

	// Create the challenge-response protected cryptohome.
	keyType := cpb.KeyData_KEY_TYPE_CHALLENGE_RESPONSE
	localKeyLabel := keyLabel
	dbusServiceName := dbusName
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
			DbusServiceName: &dbusServiceName,
			DbusObjectPath:  &keyDelegate.dbusPath,
		},
	}
	copyAuthKey := true
	mountReq := cpb.MountRequest{
		Create: &cpb.CreateRequest{
			CopyAuthorizationKey: &copyAuthKey,
		},
	}
	if err := cryptohomeClient.Mount(ctx, testUser, &authReq, &mountReq); err != nil {
		s.Fatal("Failed to create cryptohome: ", err)
	}
	if keyDelegate.challengeCallCnt == 0 {
		s.Fatal("No key challenges made during mount")
	}

	// Authenticate while the cryptohome is still mounted (modeling the case of
	// the user unlocking the device from the Lock Screen).
	if err := cryptohomeClient.CheckKey(ctx, testUser, &authReq); err != nil {
		s.Fatal("Failed to check the key for the mounted cryptohome: ", err)
	}

	if err := cryptohome.UnmountVault(ctx, testUser); err != nil {
		s.Fatal("Failed to unmount cryptohome: ", err)
	}

	// Mount the existing challenge-response protected cryptohome.
	mountReq.Create = nil
	keyDelegate.challengeCallCnt = 0
	if err := cryptohomeClient.Mount(ctx, testUser, &authReq, &mountReq); err != nil {
		s.Fatal("Failed to mount existing cryptohome: ", err)
	}
	if keyDelegate.challengeCallCnt == 0 {
		s.Fatal("No key challenges made during mount")
	}
}
