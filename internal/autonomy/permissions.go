// SPDX-License-Identifier: MIT
package autonomy

import (
	"fmt"
	"strings"
	"sync"
)

type Capability string

const (
	CapFSRead     Capability = "fs.read"
	CapFSWrite    Capability = "fs.write"
	CapGit        Capability = "git"
	CapNetHTTP    Capability = "net.http"
	CapDocker     Capability = "docker"
	CapSudo       Capability = "sudo"
	CapPkgInstall Capability = "pkg.install"
	CapExec       Capability = "exec"
)

type PermissionLevel int

const (
	PermAllowed PermissionLevel = iota
	PermAskBefore
	PermDenied
)

type PermissionStore struct {
	mu       sync.RWMutex
	allowed  map[Capability]bool
	askFirst map[Capability]bool
	denied   map[Capability]bool
	granted  map[string]bool

	requestFn func(capability Capability, reason string) (bool, error)
}

func NewPermissionStore(caps, askBefore []string) *PermissionStore {
	ps := &PermissionStore{
		allowed:  make(map[Capability]bool),
		askFirst: make(map[Capability]bool),
		denied:   make(map[Capability]bool),
		granted:  make(map[string]bool),
	}

	for _, c := range caps {
		c = strings.TrimSpace(c)
		if c != "" {
			ps.allowed[Capability(c)] = true
		}
	}

	for _, c := range askBefore {
		c = strings.TrimSpace(c)
		if c != "" {
			ps.askFirst[Capability(c)] = true
			delete(ps.allowed, Capability(c))
		}
	}

	return ps
}

func (ps *PermissionStore) SetRequestHandler(fn func(capability Capability, reason string) (bool, error)) {
	ps.requestFn = fn
}

func (ps *PermissionStore) Check(capability Capability, reason string) error {
	ps.mu.RLock()
	if ps.denied[capability] {
		ps.mu.RUnlock()
		return fmt.Errorf("capability %q permanently denied", capability)
	}
	if ps.allowed[capability] {
		ps.mu.RUnlock()
		return nil
	}
	if ps.askFirst[capability] {
		ps.mu.RUnlock()
		return ps.requestPermission(capability, reason)
	}
	ps.mu.RUnlock()
	return fmt.Errorf("capability %q not granted", capability)
}

func (ps *PermissionStore) Grant(capability Capability) {
	ps.mu.Lock()
	ps.granted[string(capability)] = true
	ps.allowed[capability] = true
	delete(ps.askFirst, capability)
	ps.mu.Unlock()
}

func (ps *PermissionStore) Deny(capability Capability, permanent bool) {
	ps.mu.Lock()
	if permanent {
		ps.denied[capability] = true
	}
	delete(ps.allowed, capability)
	delete(ps.askFirst, capability)
	ps.mu.Unlock()
}

func (ps *PermissionStore) requestPermission(capability Capability, reason string) error {
	if ps.requestFn == nil {
		return fmt.Errorf("capability %q requires approval: %s", capability, reason)
	}

	key := string(capability) + ":" + reason
	ps.mu.RLock()
	if ps.granted[key] {
		ps.mu.RUnlock()
		return nil
	}
	ps.mu.RUnlock()

	ok, err := ps.requestFn(capability, reason)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("capability %q denied: %s", capability, reason)
	}

	ps.mu.Lock()
	ps.granted[key] = true
	ps.mu.Unlock()
	return nil
}

func (ps *PermissionStore) AllGranted() []Capability {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	caps := make([]Capability, 0, len(ps.allowed)+len(ps.granted))
	for c := range ps.allowed {
		caps = append(caps, c)
	}
	for c := range ps.granted {
		caps = append(caps, Capability(c))
	}
	return caps
}
