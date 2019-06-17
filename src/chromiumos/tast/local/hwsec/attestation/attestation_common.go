// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package attestation

import (
	b64 "encoding/base64"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/protobuf/proto"

	apb "chromiumos/system_api/attestation_proto"
	"chromiumos/tast/errors"
)

const (
	// DBPath is the path of attestation database.
	DBPath string = "/mnt/stateful_partition/unencrypted/preserve/attestation.epb"
)

const (
	// DefaultACAType is the default ACA type in integral type passed into dbus message.
	DefaultACAType int = 0
	// DefaultCertProfile is the default cert profile we use when tesing.
	DefaultCertProfile apb.CertificateProfile = apb.CertificateProfile_ENTERPRISE_USER_CERTIFICATE
	// DefaultCertOrigin is the default value of the certificate origin.
	DefaultCertOrigin string = ""
	// DefaultCertLabel is the default label to identify the cert.
	DefaultCertLabel string = "aaa"
	// DefaultKeyPayload is the default key playload used for testing.
	DefaultKeyPayload string = "payload"
)

// SendPostRequestTo sends POST request with |body| to |serverURL|.
func SendPostRequestTo(body string, serverURL string) (string, error) {
	req, err := http.NewRequest("POST", serverURL, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New(string(resp.StatusCode))
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respBody), nil
}

// SendGetRequestTo sends GET request to |serverURL|
func SendGetRequestTo(serverURL string) (string, error) {
	resp, err := http.Get(serverURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respBody), nil
}

// DecodeBase64String decodes base64-encoded |enc| []bytes; also returns encountered error if any
func DecodeBase64String(enc string) ([]byte, error) {
	return b64.StdEncoding.DecodeString(enc)
}

// EncodeToBase64String base64-encodes |dec|
func EncodeToBase64String(dec []byte) string {
	return b64.StdEncoding.EncodeToString(dec)
}

// EscapeURL escapes the URL |s|
func EscapeURL(s string) string {
	return url.QueryEscape(s)
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
