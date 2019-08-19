// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gadget

import (
	"os"
	"fmt"
	"path/filepath"
	"io/ioutil"

	"chromiumos/tast/local/usb"
)

const (
	gadgetPath = "/sys/kernel/config/usb_gadget"
	gadgetLang = "strings/0x409"
)

// ConfigFragment interface
type ConfigFragment interface {
	Name() string
	Set(key string, value interface{}) error
	SetString(key string, value string) error
}

// Config holds gadget configuration data.
type Config struct {
	root sysfsConfig
}

// NewConfig creates named gadget configuration instance.
func NewConfig(name string) (*Config, error) {
	path := filepath.Join(gadgetPath, name)
	if err := os.Mkdir(path, os.ModeDir); err != nil {
		return nil, err
	} else {
		return &Config{ root: sysfsConfig{ path: path, lang: gadgetLang} }, nil
	}
}

// NewTempConfig creates unique gadget configuration instance.
func NewTempConfig() (*Config, error) {
	if path, err := ioutil.TempDir(gadgetPath, "g."); err != nil {
		return nil, err
	} else {
		return &Config{ root: sysfsConfig{ path: path, lang: gadgetLang} }, nil
	}
}

// Name returns gadget unique name.
func (c *Config) Name() string {
	return c.root.Name()
}

// Set updates top-level gadget key:value configuration pair.
func (c *Config) Set(key string, value interface{}) error {
	return c.root.Set(key, value)
}

// Set updates top-level gadget key:value string configuration pair.
func (c *Config) SetString(key string, value string) error {
	return c.root.SetString(key, value)
}

// Function provides an accessor interface to function configuration fragment.
func (c *Config) Function(name string) (ConfigFragment, error) {
	return c.fragment("functions", name, false)
}

// Config provides an accessor interface to gadget configuration fragment.
func (c *Config) Config(name string) (ConfigFragment, error) {
	return c.fragment("configs", name, true)
}

func (c *Config) removeDirs(path string) {
	files, _ := ioutil.ReadDir(path)
	for _, f := range files {
		if f.IsDir() {
			c.removeDirs(filepath.Join(path, f.Name()))
			os.Remove(filepath.Join(path, f.Name()))
		} else if f.Mode() & os.ModeSymlink != 0 {
			os.Remove(filepath.Join(path, f.Name()))
		}
	}
}

// Remove gadget configuration and all related fragments.
func (c *Config) Remove() error {
	c.removeDirs(filepath.Join(c.root.path, "configs"))
	c.removeDirs(filepath.Join(c.root.path, "functions"))
	c.removeDirs(filepath.Join(c.root.path, "strings"))
	return os.Remove(c.root.path)
}

func (c *Config) fragment(dir string, name string, link bool) (ConfigFragment, error) {
	f := &sysfsConfig{
		path: filepath.Join(c.root.path, dir, name),
		lang: c.root.lang,
		link: link,
	}
	if stat, err := os.Stat(f.path); os.IsNotExist(err) {
		if err = os.MkdirAll(f.path, os.ModeDir); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else if !stat.IsDir() {
		return nil, os.ErrExist
	}
	return f, nil
}

type sysfsConfig struct {
	path string
	lang string
	link bool
}

// Name returns configuration fragment name.
func (c *sysfsConfig) Name() string {
	return filepath.Base(c.path)
}

// Set updates configuration fragment key:value pair.
func (c *sysfsConfig) Set(key string, value interface{}) error {
	path := filepath.Join(c.path, key)
	var b []byte
	switch v := value.(type) {
	case []byte:
		return ioutil.WriteFile(path, v, 0644)
	case *sysfsConfig:
		if c.link {
			return os.Symlink(v.path, path)
		} else {
			return os.ErrInvalid
		}
	case uint8:
		b = []byte(fmt.Sprintf("0x%x", uint64(v)))
	case uint16:
		b = []byte(fmt.Sprintf("0x%x", uint64(v)))
	case uint32:
		b = []byte(fmt.Sprintf("0x%x", uint64(v)))
	case string:
		b = []byte(v)
	case usb.BCD:
		b = []byte(fmt.Sprintf("0x%04x", uint64(v)))
	default:
		b = []byte(fmt.Sprintf("%v", v))
	}
	return ioutil.WriteFile(path, b, 0644)
}

// Set updates configuration fragment key:value pair.
func (c *sysfsConfig) SetString(key, value string) error {
	path := filepath.Join(c.path, c.lang)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(path, os.ModeDir)
	}
	return ioutil.WriteFile(filepath.Join(path, key), []byte(value), 0644)
}
