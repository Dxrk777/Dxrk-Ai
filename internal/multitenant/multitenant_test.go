// SPDX-License-Identifier: MIT
package multitenant

import (
	"context"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager(t.TempDir())
	if m == nil {
		t.Fatal("expected non-nil Manager")
	}
}

func TestCreateProject(t *testing.T) {
	m := NewManager(t.TempDir())
	p, err := m.CreateProject(context.Background(), "test-project", "alice")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.Name != "test-project" {
		t.Fatalf("expected 'test-project', got %q", p.Name)
	}
	if p.Owner != "alice" {
		t.Fatalf("expected owner 'alice', got %q", p.Owner)
	}
	if p.ID == "" {
		t.Fatal("expected non-empty project ID")
	}
	if !p.Active {
		t.Fatal("expected new project to be active")
	}
}

func TestGetProject(t *testing.T) {
	m := NewManager(t.TempDir())
	created, err := m.CreateProject(context.Background(), "my-proj", "bob")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	got, ok := m.GetProject(created.ID)
	if !ok {
		t.Fatal("expected to find project")
	}
	if got.Name != "my-proj" {
		t.Fatalf("expected 'my-proj', got %q", got.Name)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	m := NewManager(t.TempDir())
	_, ok := m.GetProject("nonexistent")
	if ok {
		t.Fatal("expected false for nonexistent project")
	}
}

func TestListProjects(t *testing.T) {
	m := NewManager(t.TempDir())
	_, _ = m.CreateProject(context.Background(), "alpha", "alice")
	_, _ = m.CreateProject(context.Background(), "beta", "bob")

	projects := m.ListProjects()
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
}

func TestListProjectsEmpty(t *testing.T) {
	m := NewManager(t.TempDir())
	projects := m.ListProjects()
	if len(projects) != 0 {
		t.Fatalf("expected 0 projects, got %d", len(projects))
	}
}

func TestUpdateProject(t *testing.T) {
	m := NewManager(t.TempDir())
	created, err := m.CreateProject(context.Background(), "original", "alice")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	err = m.UpdateProject(created.ID, func(p *Project) {
		p.Name = "updated"
		p.Description = "new description"
	})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	got, _ := m.GetProject(created.ID)
	if got.Name != "updated" {
		t.Fatalf("expected 'updated', got %q", got.Name)
	}
	if got.Description != "new description" {
		t.Fatalf("expected 'new description', got %q", got.Description)
	}
}

func TestUpdateProjectNotFound(t *testing.T) {
	m := NewManager(t.TempDir())
	err := m.UpdateProject("nonexistent", func(p *Project) {
		p.Name = "nope"
	})
	if err == nil {
		t.Fatal("expected error when updating nonexistent project")
	}
}

func TestDeleteProject(t *testing.T) {
	m := NewManager(t.TempDir())
	p, _ := m.CreateProject(context.Background(), "delete-me", "alice")

	err := m.DeleteProject(p.ID)
	if err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	_, ok := m.GetProject(p.ID)
	if ok {
		t.Fatal("expected project to be deleted")
	}
}

func TestDeleteProjectNotFound(t *testing.T) {
	m := NewManager(t.TempDir())
	err := m.DeleteProject("nonexistent")
	if err == nil {
		t.Fatal("expected error when deleting nonexistent project")
	}
}

func TestSetActiveProject(t *testing.T) {
	m := NewManager(t.TempDir())
	p1, _ := m.CreateProject(context.Background(), "first", "alice")
	p2, _ := m.CreateProject(context.Background(), "second", "alice")

	active := m.GetActiveProject()
	if active == nil {
		t.Fatal("expected an active project")
	}

	err := m.SetActiveProject(p2.ID)
	if err != nil {
		t.Fatalf("SetActiveProject: %v", err)
	}

	active = m.GetActiveProject()
	if active.ID != p2.ID {
		t.Fatalf("expected active to be %q, got %q", p2.ID, active.ID)
	}

	err = m.SetActiveProject(p1.ID)
	if err != nil {
		t.Fatalf("SetActiveProject back: %v", err)
	}
}

func TestSetActiveProjectNotFound(t *testing.T) {
	m := NewManager(t.TempDir())
	err := m.SetActiveProject("nonexistent")
	if err == nil {
		t.Fatal("expected error when setting nonexistent active project")
	}
}

func TestDeleteActiveProjectSwitchesToNext(t *testing.T) {
	m := NewManager(t.TempDir())
	p1, _ := m.CreateProject(context.Background(), "first", "alice")
	p2, _ := m.CreateProject(context.Background(), "second", "bob")

	m.SetActiveProject(p1.ID)
	m.DeleteProject(p1.ID)

	active := m.GetActiveProject()
	if active == nil {
		t.Fatal("expected active to switch to remaining project")
	}
	if active.ID != p2.ID {
		t.Fatalf("expected active to be %q, got %q", p2.ID, active.ID)
	}
}

func TestGetProjectConfig(t *testing.T) {
	m := NewManager(t.TempDir())
	p, err := m.CreateProject(context.Background(), "cfg-test", "alice")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	cfg, err := m.GetProjectConfig(p.ID)
	if err != nil {
		t.Fatalf("GetProjectConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Project.Name != "cfg-test" {
		t.Fatalf("expected 'cfg-test', got %q", cfg.Project.Name)
	}
}

func TestGetProjectConfigNotFound(t *testing.T) {
	m := NewManager(t.TempDir())
	_, err := m.GetProjectConfig("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent project config")
	}
}

func TestLabels(t *testing.T) {
	m := NewManager(t.TempDir())
	p, _ := m.CreateProject(context.Background(), "labeled", "alice")

	_ = m.UpdateProject(p.ID, func(proj *Project) {
		proj.Labels["env"] = "staging"
		proj.Labels["team"] = "platform"
	})

	got, _ := m.GetProject(p.ID)
	if got.Labels["env"] != "staging" {
		t.Fatalf("expected label 'staging', got %q", got.Labels["env"])
	}
	if got.Labels["team"] != "platform" {
		t.Fatalf("expected label 'platform', got %q", got.Labels["team"])
	}
}
