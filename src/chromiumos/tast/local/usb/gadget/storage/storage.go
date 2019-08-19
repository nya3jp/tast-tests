package storage

import (
	"context"

	"chromiumos/tast/local/usb/gadget"
)

// dummy function for testing
type StorageFunction struct {
	file     string
	instance string
	config   gadget.ConfigFragment
}

// NewStorage creates an instance of mass storage gadget.
func NewStorage(instance, file string) *StorageFunction {
	return &StorageFunction { instance: instance, file: file }
}

// Start implements usbg.Function interface
func (f *StorageFunction) Start(ctx context.Context, config gadget.ConfigFragment) error {
	f.config = config
	config.Set("lun.0/removable", 1)
	return config.Set("lun.0/file", f.file)
}

// Stop implements usbg.Function interface
func (c *StorageFunction) Stop() (err error) {
	err = c.config.Set("lun.0/file", "")
	c.config = nil
	return
}

// Name implements usbg.Function interface
func (f *StorageFunction) Name() string {
	return "mass_storage." + f.instance
}
