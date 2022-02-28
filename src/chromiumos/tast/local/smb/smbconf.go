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
	params map[string]string
}

// NewConfig creates a Config which represents a smb.conf file.
func NewConfig() *Config {
	return &Config{
		global: &Section{
			name:   "global",
			params: map[string]string{},
		},
		shares: []*Section{},
	}
}

// SetGlobalParam sets a key value pair for the [global] section.
func (c *Config) SetGlobalParam(key, value string) {
	c.global.SetParam(key, value)
}

// AddFileShare adds a share as a section to the Config object.
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
		params: map[string]string{},
	}
}

// SetParam sets a key value pair on a section, these are individual
// configuration items.
func (s *Section) SetParam(key, value string) {
	s.params[key] = value
}

// String returns the string representation of a section, starting with the
// section name as [name] then each configuration item as key = value.
func (s *Section) String() string {
	var section strings.Builder
	section.WriteString(fmt.Sprintf("[%s]\n", s.name))
	for key, value := range s.params {
		section.WriteString(fmt.Sprintf("\t%s = %s\n", key, value))
	}
	return section.String()
}

// CreateBasicShare creates a file share Section with common parameters shared
// by all file shares.
func CreateBasicShare(name, path string) *Section {
	guestshare := NewFileShare(name)
	guestshare.SetParam("path", path)
	guestshare.SetParam("writeable", "yes")
	guestshare.SetParam("create mask", "0644")
	guestshare.SetParam("directory mask", "0755")
	guestshare.SetParam("read only", "no")

	return guestshare
}
