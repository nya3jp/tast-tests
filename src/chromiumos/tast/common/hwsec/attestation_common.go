// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/protobuf/proto"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
)

// SendPostRequestTo sends POST request with |body| to |serverURL|.
func SendPostRequestTo(ctx context.Context, body string, serverURL string) (string, error) {
	req, err := http.NewRequest("POST", serverURL, strings.NewReader(body))
	req.WithContext(ctx)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	return sendHTTPRequest(req)
}

// SendGetRequestTo sends GET request to |serverURL|
func SendGetRequestTo(ctx context.Context, serverURL string) (string, error) {
	req, err := http.NewRequest("GET", serverURL, strings.NewReader(""))
	req.WithContext(ctx)
	if err != nil {
		return "", err
	}
	return sendHTTPRequest(req)
}

func sendHTTPRequest(req *http.Request) (string, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.Errorf("%v %v", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respBody), nil
}

// UnmarshalSignedData unmarshal |d| into apb.SignedData; also returns encountered  error if any
func UnmarshalSignedData(d []byte) (*apb.SignedData, error) {
	var out apb.SignedData
	if err := proto.Unmarshal(d, &out); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal")
	}
	return &out, nil
}

// HexDecode decode the hex-encoded |enc| into []byte; also returns encountered error if any
func HexDecode(enc []byte) ([]byte, error) {
	dec := make([]byte, hex.DecodedLen(len(enc)))
	n, err := hex.Decode(dec, enc)
	if err != nil {
		return []byte{}, errors.Wrap(err, "failed to call hex.Decode")
	}
	return dec[:n], nil
}

type attestationClient interface {
	// IsEnrolled returns the flag to indicate if the DUT is
	// enrolled and any encountered error during the operation.
	IsEnrolled(ctx context.Context) (bool, error)
	// Creates an enroll request that is sent to the corresponding pca server of |pcaType|
	// later, and any error encountered during the operation.
	CreateEnrollRequest(ctx context.Context, pcaType PCAType) (string, error)
	// Finishes the enroll with |resp| from pca server of |pcaType|. Returns any
	// encountered error during the operation.
	FinishEnroll(ctx context.Context, pcaType PCAType, resp string) error
	// Creates a certificate request that is sent to the corresponding pca server
	// of |pcaType| later, and any error encountered during the operation.
	CreateCertRequest(ctx context.Context, pcaType PCAType, profile apb.CertificateProfile, username string, origin string) (string, error)
	// Finishes the certified key creation with |resp| from PCA server. Returns any encountered error during the operation.
	FinishCertRequest(ctx context.Context, response string, username string, label string) error
	// SignEnterpriseVAChallenge performs SPKAC for the challenge.
	SignEnterpriseVAChallenge(
		ctx context.Context,
		vaType VAType,
		username string,
		label string,
		domain string,
		deviceID string,
		includeSignedPublicKey bool,
		challenge []byte) (string, error)
	// SignSimpleChallenge signs the challenge with the specified key.
	SignSimpleChallenge(ctx context.Context, username string, label string, challenge []byte) (string, error)
	// GetPublicKey gets the public part of the key.
	GetPublicKey(ctx context.Context, username string, label string) (string, error)
}

func caServerURL(pcaType PCAType) string {
	switch pcaType {
	case DefaultPCA:
		return "https://chromeos-ca.gstatic.com"
	case TestPCA:
		return "https://asbestos-qa.corp.google.com"
	}
	panic(fmt.Sprintf("Unexpected PCA type: %v", pcaType))
}

func enrollURL(pcaType PCAType) string {
	return caServerURL(pcaType) + "/enroll"
}

func certURL(pcaType PCAType) string {
	return caServerURL(pcaType) + "/sign"
}

// AttestationTest provides the complex operations in the attestaion flow along with validations
type AttestationTest struct {
	ac      attestationClient
	pcaType PCAType
}

// NewAttestaionTest creates a new AttestationTest instance
func NewAttestaionTest(ac attestationClient, pcaType PCAType) *AttestationTest {
	return &AttestationTest{ac, pcaType}
}

// Enroll creates the enroll request, sends it to the corresponding PCA server, and finishes the request with the received response.
func (at *AttestationTest) Enroll(ctx context.Context) error {
	req, err := at.ac.CreateEnrollRequest(ctx, DefaultPCA)
	if err != nil {
		return errors.Wrap(err, "failed to create enroll request")
	}
	resp, err := SendPostRequestTo(ctx, req, enrollURL(at.pcaType))
	if err != nil {
		return errors.Wrap(err, "failed to send request to CA")
	}
	if err := at.ac.FinishEnroll(ctx, DefaultPCA, resp); err != nil {
		return errors.Wrap(err, "failed to finish enrollment")
	}
	isEnrolled, err := at.ac.IsEnrolled(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get enrollment status")
	}
	if !isEnrolled {
		return errors.New("inconsistent reported status: after enrollment, status shows 'not enrolled'")
	}
	return nil
}

// GetCertificate creates the cert request, sends it to the corresponding PCA server, and finishes the request with the received response.
func (at *AttestationTest) GetCertificate(ctx context.Context, username string, label string) error {
	req, err := at.ac.CreateCertRequest(ctx, DefaultPCA, DefaultCertProfile, username, DefaultCertOrigin)
	if err != nil {
		return errors.Wrap(err, "failed to create certificate request")
	}
	resp, err := SendPostRequestTo(ctx, req, certURL(at.pcaType))
	if err != nil {
		return errors.Wrap(err, "failed to send request to CA")
	}
	if len(resp) == 0 {
		return errors.New("unexpected empty cert")
	}
	err = at.ac.FinishCertRequest(ctx, resp, username, label)
	if err != nil {
		return errors.Wrap(err, "failed to finish cert request")
	}
	return nil
}

// SignEnterpriseChallenge gets the challenge from default VA server, perform SPKAC, and sends the signed challenge back to verify it
func (at *AttestationTest) SignEnterpriseChallenge(ctx context.Context, username string, label string) error {
	resp, err := SendGetRequestTo(ctx, "https://test-dvproxy-server.sandbox.google.com/dvproxy/getchallenge")
	if err != nil {
		return errors.Wrap(err, "failed to send request to VA")
	}
	challenge, err := base64.StdEncoding.DecodeString(resp)
	if err != nil {
		return errors.Wrap(err, "failed to base64-decode challenge")
	}
	signedChallenge, err := at.ac.SignEnterpriseVAChallenge(
		ctx,
		0,
		username,
		label,
		username,
		"fake_device_id",
		true,
		challenge)
	if err != nil {
		return errors.Wrap(err, "failed to sign VA challenge")
	}
	b64SignedChallenge := base64.StdEncoding.EncodeToString([]byte(signedChallenge))
	urlForVerification := "https://test-dvproxy-server.sandbox.google.com/dvproxy/verifychallengeresponse?signeddata=" + url.QueryEscape(b64SignedChallenge)
	resp, err = SendGetRequestTo(ctx, urlForVerification)
	if err != nil {
		return errors.Wrap(err, "failed to verify challenge")
	}
	return nil
}

// SignSimpleChallenge signs a known, short data with the cert, and verify it using its public key
func (at *AttestationTest) SignSimpleChallenge(ctx context.Context, username string, label string) error {
	signedChallenge, err := at.ac.SignSimpleChallenge(ctx, username, label, []byte{})
	if err != nil {
		return errors.Wrap(err, "failed to sign simple challenge")
	}
	signedChallengeProto, err := UnmarshalSignedData([]byte(signedChallenge))
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal simple challenge reply")
	}
	publicKeyHex, err := at.ac.GetPublicKey(ctx, username, label)
	if err != nil {
		return errors.Wrap(err, "failed to get public key")
	}
	publicKeyDer, err := HexDecode([]byte(publicKeyHex))
	if err != nil {
		return errors.Wrap(err, "failed to hex-decode public key")
	}
	publicKey, err := x509.ParsePKIXPublicKey(publicKeyDer)
	if err != nil {
		return errors.Wrap(err, "failed to construct rsa public key")
	}
	hashValue := sha256.Sum256(signedChallengeProto.GetData())
	if err := rsa.VerifyPKCS1v15(publicKey.(*rsa.PublicKey), crypto.SHA256, hashValue[:], signedChallengeProto.GetSignature()); err != nil {
		return errors.Wrap(err, "failed to verify signature")
	}
	return nil
}
