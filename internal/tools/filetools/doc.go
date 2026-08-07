package filetools

import "github.com/Dxrk777/Dxrk/internal/tools"

func RegisterAll(reg *tools.Registry) error {
	for _, fn := range []func(*tools.Registry) error{
		registerFileRead,
		registerFileWrite,
		registerFileEdit,
		registerGlob,
		registerGrep,
	} {
		if err := fn(reg); err != nil {
			return err
		}
	}
	return nil
}

func boolPtr(b bool) *bool { return &b }
