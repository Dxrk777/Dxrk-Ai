// SPDX-License-Identifier: MIT
package marketplace

import (
	"context"
	"fmt"
	"time"
)

type InstallOptions struct {
	Name    string
	Version string
	Type    Type
	Tags    []string
}

type Installer struct {
	store Store
}

func NewInstaller(store Store) *Installer {
	return &Installer{store: store}
}

func (inst *Installer) Install(ctx context.Context, source string, opts InstallOptions) (*Listing, error) {
	listing := Listing{
		ID:        fmt.Sprintf("listing_%d", time.Now().UnixNano()),
		Name:      opts.Name,
		Version:   opts.Version,
		Type:      opts.Type,
		Tags:      opts.Tags,
		SourceURL: source,
		UpdatedAt: time.Now(),
	}
	if err := inst.store.Register(ctx, listing); err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}
	return &listing, nil
}

func (inst *Installer) Uninstall(ctx context.Context, id string) error {
	return inst.store.Delete(ctx, id)
}

func (inst *Installer) ListInstalled(ctx context.Context, filter Filter) ([]Listing, error) {
	return inst.store.List(ctx, filter)
}
