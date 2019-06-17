// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hwsec

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
	defaultPreparationForEnrolmentTimeout int = 40 * 1000
	defaultTakingOwnershipTimeout         int = 40 * 1000
)

const (
	defaultACAType     int                    = 0
	defaultCertProfile apb.CertificateProfile = apb.CertificateProfile_ENTERPRISE_USER_CERTIFICATE
	defaultCertOrigin  string                 = ""
	defaultCertLabel   string                 = "aaa"
	defaultKeyPayload  string                 = "payload"
)

func sendPostRequestTo(body string, serverURL string) (string, error) {
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
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respBody), nil
}

func sendGetRequestTo(serverURL string) (string, error) {
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

func decodeBase64String(enc string) ([]byte, error) {
	return b64.StdEncoding.DecodeString(enc)
}

func encodeToBase64String(dec []byte) string {
	return b64.StdEncoding.EncodeToString(dec)
}

func escapeUrl(dec string) string {
	return url.QueryEscape(dec)
}

func unmarshalSignedData(d []byte) (*apb.SignedData, error) {
	var out apb.SignedData
	if err := proto.Unmarshal(d, &out); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal")
	}
	return &out, nil
}

func hexDecode(enc []byte) ([]byte, error) {
	dec := make([]byte, hex.DecodedLen(len(enc)))
	if n, err := hex.Decode(dec, enc); err != nil {
		return []byte{}, errors.Wrap(err, "failed to call hex.Decode")
	} else {
		return dec[:n], nil
	}
}
