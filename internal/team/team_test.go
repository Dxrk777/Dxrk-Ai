package team

import (
	"sync"
	"testing"
	"time"
)

func TestAddGetUpdateRemoveMember(t *testing.T) {
	m := NewManager()

	member := Member{
		ID:    "1",
		Name:  "Alice",
		Email: "alice@example.com",
		Role:  RoleDeveloper,
	}

	err := m.AddMember(member)
	if err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}

	got, err := m.GetMember("1")
	if err != nil {
		t.Fatalf("GetMember failed: %v", err)
	}
	if got.Name != "Alice" {
		t.Errorf("expected name Alice, got %s", got.Name)
	}

	got.Name = "Updated"
	member.Name = "Updated"
	err = m.UpdateMember(member)
	if err != nil {
		t.Fatalf("UpdateMember failed: %v", err)
	}

	got, err = m.GetMember("1")
	if err != nil {
		t.Fatalf("GetMember after update failed: %v", err)
	}
	if got.Name != "Updated" {
		t.Errorf("expected name Updated, got %s", got.Name)
	}

	err = m.RemoveMember("1")
	if err != nil {
		t.Fatalf("RemoveMember failed: %v", err)
	}

	_, err = m.GetMember("1")
	if err == nil {
		t.Fatal("expected error getting removed member")
	}
}

func TestAddDuplicateMember(t *testing.T) {
	m := NewManager()
	member := Member{ID: "1", Name: "Alice"}

	if err := m.AddMember(member); err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if err := m.AddMember(member); err == nil {
		t.Fatal("expected error adding duplicate member")
	}
}

func TestRemoveMissingMember(t *testing.T) {
	m := NewManager()
	if err := m.RemoveMember("nonexistent"); err == nil {
		t.Fatal("expected error removing missing member")
	}
}

func TestGetMissingMember(t *testing.T) {
	m := NewManager()
	if _, err := m.GetMember("nonexistent"); err == nil {
		t.Fatal("expected error getting missing member")
	}
}

func TestListMembers(t *testing.T) {
	m := NewManager()
	members := []Member{
		{ID: "1", Name: "Alice", Role: RoleViewer},
		{ID: "2", Name: "Bob", Role: RoleDeveloper},
		{ID: "3", Name: "Carol", Role: RoleAdmin},
	}
	for _, member := range members {
		if err := m.AddMember(member); err != nil {
			t.Fatalf("AddMember failed: %v", err)
		}
	}

	list := m.ListMembers()
	if len(list) != 3 {
		t.Fatalf("expected 3 members, got %d", len(list))
	}
}

func TestListMembersByRole(t *testing.T) {
	m := NewManager()
	members := []Member{
		{ID: "1", Name: "Alice", Role: RoleViewer},
		{ID: "2", Name: "Bob", Role: RoleDeveloper},
		{ID: "3", Name: "Carol", Role: RoleViewer},
	}
	for _, member := range members {
		if err := m.AddMember(member); err != nil {
			t.Fatalf("AddMember failed: %v", err)
		}
	}

	viewers := m.ListMembersByRole(RoleViewer)
	if len(viewers) != 2 {
		t.Fatalf("expected 2 viewers, got %d", len(viewers))
	}

	devs := m.ListMembersByRole(RoleDeveloper)
	if len(devs) != 1 {
		t.Fatalf("expected 1 developer, got %d", len(devs))
	}

	owners := m.ListMembersByRole(RoleOwner)
	if len(owners) != 0 {
		t.Fatalf("expected 0 owners, got %d", len(owners))
	}
}

func TestRolePermissions(t *testing.T) {
	tests := []struct {
		role        Role
		has         []Permission
		doesNotHave []Permission
	}{
		{
			role:        RoleViewer,
			has:         []Permission{PermissionRead},
			doesNotHave: []Permission{PermissionWrite, PermissionExecute, PermissionManageMembers, PermissionManageRoles, PermissionDelete, PermissionAdmin},
		},
		{
			role:        RoleDeveloper,
			has:         []Permission{PermissionRead, PermissionWrite, PermissionExecute},
			doesNotHave: []Permission{PermissionManageMembers, PermissionManageRoles, PermissionDelete, PermissionAdmin},
		},
		{
			role:        RoleMaintainer,
			has:         []Permission{PermissionRead, PermissionWrite, PermissionExecute, PermissionManageMembers},
			doesNotHave: []Permission{PermissionManageRoles, PermissionDelete, PermissionAdmin},
		},
		{
			role:        RoleAdmin,
			has:         []Permission{PermissionRead, PermissionWrite, PermissionExecute, PermissionManageMembers, PermissionManageRoles, PermissionDelete},
			doesNotHave: []Permission{PermissionAdmin},
		},
		{
			role:        RoleOwner,
			has:         []Permission{PermissionRead, PermissionWrite, PermissionExecute, PermissionManageMembers, PermissionManageRoles, PermissionDelete, PermissionAdmin},
			doesNotHave: nil,
		},
	}

	for _, tt := range tests {
		perms := tt.role.Permissions()
		permMap := make(map[Permission]bool)
		for _, p := range perms {
			permMap[p] = true
		}

		for _, p := range tt.has {
			if !permMap[p] {
				t.Errorf("role %s should have permission %s", tt.role, p)
			}
		}
		for _, p := range tt.doesNotHave {
			if permMap[p] {
				t.Errorf("role %s should not have permission %s", tt.role, p)
			}
		}
	}
}

func TestHasPermission(t *testing.T) {
	m := NewManager()
	member := Member{ID: "1", Name: "Alice", Role: RoleViewer}
	if err := m.AddMember(member); err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}

	if !m.HasPermission("1", PermissionRead) {
		t.Error("viewer should have read permission")
	}
	if m.HasPermission("1", PermissionWrite) {
		t.Error("viewer should not have write permission")
	}
	if m.HasPermission("1", PermissionAdmin) {
		t.Error("viewer should not have admin permission")
	}

	if err := m.SetRole("1", RoleAdmin); err != nil {
		t.Fatalf("SetRole failed: %v", err)
	}

	allPerms := []Permission{PermissionRead, PermissionWrite, PermissionExecute, PermissionManageMembers, PermissionManageRoles, PermissionDelete}
	for _, p := range allPerms {
		if !m.HasPermission("1", p) {
			t.Errorf("admin should have permission %s", p)
		}
	}
	if m.HasPermission("1", PermissionAdmin) {
		t.Error("admin should not have admin permission")
	}

	if m.HasPermission("nonexistent", PermissionRead) {
		t.Error("nonexistent member should not have any permission")
	}
}

func TestSetRole(t *testing.T) {
	m := NewManager()
	member := Member{ID: "1", Name: "Alice", Role: RoleViewer}
	if err := m.AddMember(member); err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}

	if err := m.SetRole("1", RoleOwner); err != nil {
		t.Fatalf("SetRole failed: %v", err)
	}

	got, err := m.GetMember("1")
	if err != nil {
		t.Fatalf("GetMember failed: %v", err)
	}
	if got.Role != RoleOwner {
		t.Errorf("expected RoleOwner, got %v", got.Role)
	}

	if err := m.SetRole("nonexistent", RoleAdmin); err == nil {
		t.Fatal("expected error setting role on missing member")
	}
}

func TestSkillAssignmentAndLookup(t *testing.T) {
	m := NewManager()
	if err := m.AddMember(Member{ID: "1", Name: "Alice"}); err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}
	if err := m.AddMember(Member{ID: "2", Name: "Bob"}); err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}

	if err := m.AssignSkill("1", "Go"); err != nil {
		t.Fatalf("AssignSkill failed: %v", err)
	}
	if err := m.AssignSkill("1", "Kubernetes"); err != nil {
		t.Fatalf("AssignSkill failed: %v", err)
	}
	if err := m.AssignSkill("2", "Go"); err != nil {
		t.Fatalf("AssignSkill failed: %v", err)
	}

	skills, err := m.GetSkillsByMember("1")
	if err != nil {
		t.Fatalf("GetSkillsByMember failed: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}

	goMembers := m.GetMembersBySkill("Go")
	if len(goMembers) != 2 {
		t.Fatalf("expected 2 Go members, got %d", len(goMembers))
	}

	k8sMembers := m.GetMembersBySkill("Kubernetes")
	if len(k8sMembers) != 1 {
		t.Fatalf("expected 1 Kubernetes member, got %d", len(k8sMembers))
	}

	if err := m.RemoveSkill("1", "Go"); err != nil {
		t.Fatalf("RemoveSkill failed: %v", err)
	}

	skills, err = m.GetSkillsByMember("1")
	if err != nil {
		t.Fatalf("GetSkillsByMember failed: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill after removal, got %d", len(skills))
	}
	if skills[0] != "Kubernetes" {
		t.Errorf("expected Kubernetes, got %s", skills[0])
	}

	if err := m.AssignSkill("1", "Go"); err != nil {
		t.Fatalf("re-AssignSkill failed: %v", err)
	}

	if err := m.AssignSkill("nonexistent", "Go"); err == nil {
		t.Fatal("expected error assigning skill to missing member")
	}

	if _, err := m.GetSkillsByMember("nonexistent"); err == nil {
		t.Fatal("expected error getting skills for missing member")
	}

	if err := m.RemoveSkill("nonexistent", "Go"); err == nil {
		t.Fatal("expected error removing skill from missing member")
	}

	if err := m.RemoveSkill("1", "NonexistentSkill"); err == nil {
		t.Fatal("expected error removing nonexistent skill")
	}
}

func TestDuplicateSkill(t *testing.T) {
	m := NewManager()
	m.AddMember(Member{ID: "1", Name: "Alice"})

	m.AssignSkill("1", "Go")
	m.AssignSkill("1", "Go")

	skills, _ := m.GetSkillsByMember("1")
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (deduped), got %d", len(skills))
	}
}

func TestActiveFiltering(t *testing.T) {
	m := NewManager()
	members := []Member{
		{ID: "1", Name: "Alice", Active: true},
		{ID: "2", Name: "Bob", Active: false},
		{ID: "3", Name: "Carol", Active: true},
	}
	for _, member := range members {
		if err := m.AddMember(member); err != nil {
			t.Fatalf("AddMember failed: %v", err)
		}
	}

	active := m.GetActiveMembers()
	if len(active) != 2 {
		t.Fatalf("expected 2 active members, got %d", len(active))
	}
}

func TestRecordActivity(t *testing.T) {
	m := NewManager()
	member := Member{ID: "1", Name: "Alice"}
	if err := m.AddMember(member); err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}

	before := time.Now()
	m.RecordActivity("1")

	got, err := m.GetMember("1")
	if err != nil {
		t.Fatalf("GetMember failed: %v", err)
	}

	if got.LastActiveAt.Before(before) {
		t.Error("LastActiveAt should be updated to after before time")
	}
}

func TestRecordActivityMissingMember(t *testing.T) {
	m := NewManager()
	m.RecordActivity("nonexistent")
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('A' + i))
			m.AddMember(Member{ID: id, Name: id, Active: i%2 == 0})
		}(i)
	}
	wg.Wait()

	if len(m.ListMembers()) != 100 {
		t.Fatalf("expected 100 members, got %d", len(m.ListMembers()))
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('A' + i))
			m.HasPermission(id, PermissionRead)
			m.RecordActivity(id)
		}(i)
	}
	wg.Wait()

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := string(rune('A' + i))
			m.GetMember(id)
		}(i)
	}
	wg.Wait()
}

func TestRoleString(t *testing.T) {
	tests := []struct {
		role Role
		want string
	}{
		{RoleViewer, "viewer"},
		{RoleDeveloper, "developer"},
		{RoleMaintainer, "maintainer"},
		{RoleAdmin, "admin"},
		{RoleOwner, "owner"},
		{Role(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.role.String(); got != tt.want {
			t.Errorf("Role(%d).String() = %s, want %s", tt.role, got, tt.want)
		}
	}
}

func TestUpdateMissingMember(t *testing.T) {
	m := NewManager()
	if err := m.UpdateMember(Member{ID: "nonexistent"}); err == nil {
		t.Fatal("expected error updating missing member")
	}
}
