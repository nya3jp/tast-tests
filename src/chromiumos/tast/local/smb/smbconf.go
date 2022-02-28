// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package smb

import (
	"fmt"
	"strings"
)

// Config represents the enter smb.conf file with a [global] section and 1 or
// more file shares.
type Config struct {
	global *Section
	shares []*Section
}

// Section represents either the [global] section or a file share.
// Each section is made up of parameters of the format:
//    key = value
type Section struct {
	name   string
	params []*Parameter
}

// Parameter contains the key value pairs defining various options for either
// the [global] section or a file share.
type Parameter struct {
	key   string
	value string
}

// NewConfig creates a Config which represents a smb.conf file.
func NewConfig() *Config {
	return &Config{
		global: &Section{
			name:   "global",
			params: []*Parameter{},
		},
		shares: []*Section{},
	}
}

// SetGlobalParam sets a key value pair for the [global] section.
func (c *Config) SetGlobalParam(key, value string) {
	c.global.SetParam(key, value)
}

// AddFileShare ads a share as a section to the Config object.
func (c *Config) AddFileShare(share *Section) {
	c.shares = append(c.shares, share)
}

// String returns a string representation of the samba config file as per:
// https://www.samba.org/samba/docs/current/man-html/smb.conf.5.html
func (c *Config) String() string {
	var smbConf strings.Builder
	if len(c.global.params) > 0 {
		smbConf.WriteString(c.global.String())
		smbConf.WriteString("\n")

	}
	if len(c.shares) > 0 {
		for _, section := range c.shares {
			smbConf.WriteString(section.String())
			smbConf.WriteString("\n")
		}
	}
	return smbConf.String()
}

// NewFileShare creates a subsection that starts with [name] and has key value
// pairs representing information about a file share.
func NewFileShare(name string) *Section {
	return &Section{
		name:   name,
		params: []*Parameter{},
	}
}

// SetParam sets a key value pair on a section, these are individual
// configuration items.
func (s *Section) SetParam(key, value string) {
	param := &Parameter{
		key:   key,
		value: value,
	}
	s.params = append(s.params, param)
}

// String returns the string representation of a section, starting with the
// section name as [name] then each configuration item as key = value.
func (s *Section) String() string {
	var section strings.Builder
	section.WriteString(fmt.Sprintf("[%s]\n", s.name))
	for _, param := range s.params {
		section.WriteString(fmt.Sprintf("\t%s = %s\n", param.key, param.value))
	}
	return section.String()
}
