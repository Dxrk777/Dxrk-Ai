package team

import (
	"time"

	"github.com/Dxrk777/Dxrk-Ai/internal/strconst"
)

type Role int

const (
	RoleViewer Role = iota
	RoleDeveloper
	RoleMaintainer
	RoleAdmin
	RoleOwner
)

func (r Role) String() string {
	switch r {
	case RoleViewer:
		return "viewer"
	case RoleDeveloper:
		return "developer"
	case RoleMaintainer:
		return "maintainer"
	case RoleAdmin:
		return "admin"
	case RoleOwner:
		return "owner"
	default:
		return strconst.StrUnknown
	}
}

func (r Role) Permissions() []Permission {
	switch r {
	case RoleViewer:
		return []Permission{PermissionRead}
	case RoleDeveloper:
		return []Permission{PermissionRead, PermissionWrite, PermissionExecute}
	case RoleMaintainer:
		return []Permission{PermissionRead, PermissionWrite, PermissionExecute, PermissionManageMembers}
	case RoleAdmin:
		return []Permission{PermissionRead, PermissionWrite, PermissionExecute, PermissionManageMembers, PermissionManageRoles, PermissionDelete}
	case RoleOwner:
		return []Permission{PermissionRead, PermissionWrite, PermissionExecute, PermissionManageMembers, PermissionManageRoles, PermissionDelete, PermissionAdmin}
	default:
		return nil
	}
}

type Permission string

const (
	PermissionRead          Permission = "read"
	PermissionWrite         Permission = "write"
	PermissionExecute       Permission = "execute"
	PermissionManageMembers Permission = "manage_members"
	PermissionManageRoles   Permission = "manage_roles"
	PermissionDelete        Permission = "delete"
	PermissionAdmin         Permission = "admin"
)

type Member struct {
	ID           string
	Name         string
	Email        string
	Role         Role
	Skills       []string
	JoinedAt     time.Time
	LastActiveAt time.Time
	Active       bool
}
