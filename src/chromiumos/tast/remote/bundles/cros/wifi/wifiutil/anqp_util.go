// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/hostapd"
)

// Domains list separator for NAI Realms.
const realmDomainsSep = ";"

func decodeError(err error, in, msg string, args ...interface{}) error {
	m := fmt.Sprintf(msg, args...)
	if err != nil {
		return errors.Wrapf(err, "%s (input: %q)", m, in)
	}
	return errors.Errorf("%s (input %q)", m, in)
}

// DecodeRoamingConsortiums extracts a roaming consortium OIs list from a buffer
// represented as a hex string.
func DecodeRoamingConsortiums(s string) ([]string, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, decodeError(err, s, "failed to decode roaming consortiums hex string")
	}
	r := bytes.NewReader(b)

	var rcs []string
	for {
		// First byte is the length of the roaming consortium.
		rcLen, err := r.ReadByte()
		if err == io.EOF {
			// End of buffer reached, no more roaming consortiums.
			break
		}
		if err != nil {
			return nil, decodeError(err, s, "failed to read roaming consortium length")
		}
		// rcLen following bytes are the roaming consortium.
		rc := make([]byte, rcLen)
		if _, err := io.ReadFull(r, rc); err != nil {
			return nil, decodeError(nil, s, "failed to read roaming consortium")
		}
		rcs = append(rcs, hex.EncodeToString(rc))
	}
	return rcs, nil
}

// DecodeVenueGroupTypeName extracts the venue type, group and names from a
// buffer represented as a hex string.
func DecodeVenueGroupTypeName(s string) (info hostapd.VenueInfo, names []hostapd.VenueName, err error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return info, names, decodeError(err, s, "failed to decode venue name hex string")
	}
	r := bytes.NewReader(b)

	info.Group, err = r.ReadByte()
	if err != nil {
		return info, names, decodeError(err, s, "failed to read venue group")
	}

	info.Type, err = r.ReadByte()
	if err != nil {
		return info, names, decodeError(err, s, "failed to decode venue type")
	}

	// The tail of the buffer is a list of venue names in the form [size, name].
	for {
		size, err := r.ReadByte()
		if err == io.EOF {
			// End of the venue names list.
			break
		}
		if err != nil {
			return info, names, decodeError(err, s, "failed to read venue name length")
		}
		// The venue name must contain at least the language code (3 bytes) and a name
		if size < 4 {
			return info, names, decodeError(err, s, "invalid venue name length: got %d want at least 4", size)
		}

		lang := make([]byte, 3)
		if _, err := io.ReadFull(r, lang); err != nil {
			return info, names, decodeError(err, s, "failed to read venue name language")
		}

		name := make([]byte, size-3)
		if _, err := io.ReadFull(r, name); err != nil {
			return info, names, decodeError(err, s, "failed to read venue name")
		}

		names = append(names, hostapd.VenueName{
			Lang: string(lang),
			Name: string(name),
		})
	}
	return info, names, nil
}

// DecodeDomainNames decodes the list of domain names from a buffer of bytes
// represented as a hex string.
func DecodeDomainNames(s string) ([]string, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, decodeError(err, s, "failed to decode domain names string")
	}
	r := bytes.NewReader(b)

	var domains []string
	for {
		// First byte is the length of the domain name.
		size, err := r.ReadByte()
		if err == io.EOF {
			// End of domains list.
			break
		}
		if err != nil {
			return nil, decodeError(err, s, "failed to read domain length")
		}

		// 'size' following bytes are the domain name
		domain := make([]byte, size)
		if _, err := io.ReadFull(r, domain); err != nil {
			return nil, decodeError(err, s, "failed to read domain name")
		}
		// 'size' following bytes are the actual domain name.
		domains = append(domains, string(domain))
	}
	return domains, nil
}

// DecodeNAIRealms decodes a list of NAI Realms from a buffer represented as a
// hex string. The realms are decoded following the format described in IEEE
// 802.11u std 7.3.4.9.
func DecodeNAIRealms(s string) ([]hostapd.NAIRealm, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, decodeError(err, s, "failed to decode NAI realms string")
	}
	r := bytes.NewReader(b)

	// Obtain the number of NAI Realms included in the buffer.
	var count uint16
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return nil, decodeError(err, s, "failed to decode NAI realm count")
	}

	var realms []hostapd.NAIRealm
	for i := 0; i < int(count); i++ {
		realm, err := decodeNAIRealm(r)
		if err != nil {
			return nil, decodeError(err, s, "failed to parse NAI Realm %d", i)
		}
		realms = append(realms, realm)
	}

	return realms, nil
}

// decodeNAIRealm extracts a single NAI Realm from the provided bytes reader.
// The format of a single realm is described in IEEE 802.11u std - Figure 7-95af.
func decodeNAIRealm(r *bytes.Reader) (realm hostapd.NAIRealm, err error) {
	var dataLen uint16
	if err := binary.Read(r, binary.LittleEndian, &dataLen); err != nil {
		return realm, errors.New("failed to read NAI Realm data field length")
	}
	// NAI Realm buffer contains at least the encoding, the realm length and the
	// method count (see IEEE802.11u - figure 7-95af)
	if dataLen < 3 {
		return realm, errors.Errorf("NAI Realm invalid data length, got %d, expected at least 3", dataLen)
	}

	// Encoding is ignored
	realmEncoding, err := r.ReadByte()
	if err != nil {
		return realm, errors.Wrap(err, "failed to read NAI realm encoding")
	}
	switch realmEncoding {
	case 0:
		realm.Encoding = hostapd.RealmEncodingRFC4282
	case 1:
		realm.Encoding = hostapd.RealmEncodingUTF8
	default:
		return realm, errors.Errorf("invalid NAI Realm encoding %d", realmEncoding)
	}

	realmLen, err := r.ReadByte()
	if err != nil {
		return realm, errors.Wrap(err, "failed to read NAI realm length")
	}

	realmBuf := make([]byte, realmLen)
	if _, err := io.ReadFull(r, realmBuf); err != nil {
		return realm, errors.Wrap(err, "failed to read realm")
	}
	realm.Domains = strings.Split(string(realmBuf), realmDomainsSep)

	count, err := r.ReadByte()
	if err != nil {
		return realm, errors.Wrap(err, "failed to read EAP method count")
	}

	for i := 0; i < int(count); i++ {
		method, err := decodeEAPMethod(r)
		if err != nil {
			return realm, errors.Wrapf(err, "failed to parse EAP method %d", i)
		}
		realm.Methods = append(realm.Methods, method)
	}

	return realm, nil
}

// decodeEAPMethod extracts a EAP method of a realm from the provided bytes reader.
// The format of a EAP method is described in IEEE 802.11u std - Figure 7-95ah.
func decodeEAPMethod(r *bytes.Reader) (m hostapd.EAPMethod, err error) {
	methodLen, err := r.ReadByte()
	if err != nil {
		return m, errors.Wrap(err, "failed to read EAP method length")
	}
	// EAP method subfied contains at least the EAP method and the authentication
	// parameter count (see IEEE802.11u - figure 7-95ah).
	if methodLen < 2 {
		return m, errors.Errorf("EAP method: invalid subfield length, got %d, expected at least 2", methodLen)
	}

	methodType, err := r.ReadByte()
	if err != nil {
		return m, errors.Wrap(err, "failed to read EAP method identifier")
	}
	m.Type = hostapd.EAPMethodType(methodType)

	count, err := r.ReadByte()
	if err != nil {
		return m, errors.Wrap(err, "failed to read EAP authentication parameter count")
	}

	for i := 0; i < int(count); i++ {
		p, err := decodeEAPAuthenticationParameter(r)
		if err != nil {
			return m, errors.Wrapf(err, "EAP method %d: failed to parse EAP authentication parameter %d", methodType, i)
		}
		m.Params = append(m.Params, p)
	}

	return m, nil
}

// decodeEAPAuthenticationParameter decodes a EAP authentication parameter from
// the provided bytes reader. The format of a EAP authentication parameter is
// described in IEEE 802.11u std - Figure 7-95ai.
func decodeEAPAuthenticationParameter(r *bytes.Reader) (p hostapd.EAPAuthParam,
	err error) {
	t, err := r.ReadByte()
	if err != nil {
		return p, errors.Wrap(err, "failed to read authentication parameter type")
	}
	p.Type = hostapd.EAPAuthParamType(t)

	paramLen, err := r.ReadByte()
	if err != nil {
		return p, errors.Wrap(err, "failed to read authentication parameter length")
	}

	// For now only single byte parameters are supported (see IEEE802.11u - table 7-43bo).
	if paramLen > 1 {
		return p, errors.Errorf("authentication parameter %d (length %d) is not supported", t, paramLen)
	}

	v, err := r.ReadByte()
	if err != nil {
		return p, errors.Wrap(err, "failed to read EAP authentication parameter value")
	}
	p.Value = v

	return p, nil
}
