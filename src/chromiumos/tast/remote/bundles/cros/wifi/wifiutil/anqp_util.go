// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifiutil

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/remote/wificell/hostapd"
)

// DecodeRoamingConsortiums extracts a roaming consortium OIs list from a buffer
// represented as a hex string.
func DecodeRoamingConsortiums(s string) ([]string, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode roaming consortiums string")
	}
	var rcs []string
	for i := 0; i < len(b); {
		// First byte is the length of the roaming consortium
		rcLen := int(b[i])
		i++
		// rcLen following bytes are the roaming consortium
		rcs = append(rcs, hex.EncodeToString(b[i:i+rcLen]))
		i += rcLen
	}
	return rcs, nil
}

// DecodeVenueGroupTypeName extracts the venue type, group and names from a
// buffer represented as a hex string.
func DecodeVenueGroupTypeName(s string) (info hostapd.VenueInfo, names []string, err error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return info, names, errors.Wrap(err, "failed to decode venue name string")
	}
	// The buffer should at least contain 3 bytes: the group, the type and the size of the name.
	if len(b) <= 3 {
		return info, names, errors.Errorf("buffer too short for group, type and size (size=%d)", len(b))
	}
	info.Group = b[0]
	info.Type = b[1]

	// The tail of the buffer is a list of venue names in the form [size, name].
	for i := 2; i < len(b); {
		size := int(b[i])
		i++
		if size == 0 {
			break
		}
		if size <= 3 || i+size > len(b) {
			return info, names, errors.New("venue name buffer too short to contain a valid name")
		}
		// The venue name is made of 3 chars of language code, and a name.
		names = append(names, fmt.Sprintf("%s:%s", string(b[i:i+3]), string(b[i+3:i+size])))
		i += size
	}
	return info, names, nil
}

// DecodeDomainNames decodes the list of domain names from a buffer of bytes
// represented as a hex string.
func DecodeDomainNames(s string) ([]string, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode domain names string")
	}
	var domains []string
	for i := 0; i < len(b); {
		// First byte is the length of the domain name.
		size := int(b[i])
		i++
		// 'size' following bytes are the actual domain name.
		domains = append(domains, string(b[i:i+size]))
		i += size
	}
	return domains, nil
}

// DecodeNAIRealms decodes a list of NAI Realms from a buffer represented as a
// hex string. The realms are decoded following the format described in IEEE
// 802.11u std 7.3.4.9.
func DecodeNAIRealms(s string) ([]hostapd.NAIRealm, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode NAI realms string")
	}
	r := bytes.NewReader(b)

	// Obtain the number of NAI Realms included in the buffer.
	var count uint16
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return nil, errors.Wrap(err, "failed to decode NAI realm count")
	}

	var realms []hostapd.NAIRealm
	for i := 0; i < int(count); i++ {
		realm, err := decodeNAIRealm(r)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse NAI Realm %d", i)
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

	// Encoding is ignred
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

	var realmBuf bytes.Buffer
	for i := 0; i < int(realmLen); {
		ch, sz, err := r.ReadRune()
		if err != nil {
			return realm, errors.Wrap(err, "failed to read realm")
		}
		realmBuf.WriteRune(ch)
		i += sz
	}
	realm.Domains = strings.Split(realmBuf.String(), ";")

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
