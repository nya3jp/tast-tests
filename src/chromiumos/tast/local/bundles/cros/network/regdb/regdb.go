// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package regdb supports parsing the regulatory database used by the Linux kernel's WiFi framework.
package regdb

import (
	"bytes"
	"encoding/binary"
	"io/ioutil"
	"unicode"

	"chromiumos/tast/errors"
)

const (
	regdbPath = "/lib/firmware/regulatory.db"
	// NB: the regulatory database layout is not documented, but it serves as a contract between the "wireless-regdb" (regulatory.db)
	// project and the kernel. Its layout was determined from the source code.
	headerMagic   = 0x52474442 // RGDB
	headerVersion = 20
)

// Country represents a country entry in the regulatory database.
type Country struct {
	Alpha string
}

// RegulatoryDb holds country info from the WiFi regulatory database.
type RegulatoryDb struct {
	Countries []*Country
}

func parseHeader(r *bytes.Reader) error {
	var header struct {
		Magic   uint32
		Version uint32
	}

	if err := binary.Read(r, binary.BigEndian, &header); err != nil {
		return err
	}

	if header.Magic != headerMagic {
		return errors.Errorf("invalid regdb magic: %#x", header.Magic)
	}
	if header.Version != headerVersion {
		return errors.Errorf("invalid regdb version: %d", header.Version)
	}

	return nil
}

func parseCountry(r *bytes.Reader) (c *Country, done bool, err error) {
	var country struct {
		Alpha   [2]byte
		CollPtr uint16
	}

	if err = binary.Read(r, binary.BigEndian, &country); err != nil {
		err = errors.Wrap(err, "failed reading country")
		return
	}

	if country.CollPtr == 0 {
		done = true
		return
	}

	str := string(country.Alpha[:])
	for _, b := range str {
		if !unicode.IsLetter(b) && !unicode.IsNumber(b) {
			err = errors.Errorf("invalid country string: %q", str)
			return
		}
	}
	if len(str) != 2 {
		err = errors.Errorf("invalid country string: %q", str)
		return
	}

	c = &Country{
		Alpha: str,
	}

	return
}

func parseCountries(r *bytes.Reader) ([]*Country, error) {
	var countries []*Country
	for {
		country, done, err := parseCountry(r)
		if err != nil {
			return nil, err
		} else if done {
			return countries, nil
		}
		countries = append(countries, country)
	}
}

func newRegulatoryDb(b []byte) (*RegulatoryDb, error) {
	r := bytes.NewReader(b)

	if err := parseHeader(r); err != nil {
		return nil, err
	}

	countries, err := parseCountries(r)
	if err != nil {
		return nil, err
	}

	return &RegulatoryDb{
		Countries: countries,
	}, nil
}

// NewRegulatoryDb retrieves and parses the system's WiFi regulatory database.
func NewRegulatoryDb() (*RegulatoryDb, error) {
	b, err := ioutil.ReadFile(regdbPath)
	if err != nil {
		return nil, err
	}

	return newRegulatoryDb(b)
}
