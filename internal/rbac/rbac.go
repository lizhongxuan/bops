package rbac

type Permission string

const (
	PermPlan    Permission = "plan"
	PermApply   Permission = "apply"
	PermApprove Permission = "approve"
	PermSecrets Permission = "secrets"
)

type Role struct {
	Name        string
	Permissions map[Permission]struct{}
}

type Subject struct {
	ID    string
	Roles []Role
}

func (s Subject) Has(permission Permission) bool {
	for _, role := range s.Roles {
		if _, ok := role.Permissions[permission]; ok {
			return true
		}
	}
	return false
}

func NewRole(name string, perms ...Permission) Role {
	set := make(map[Permission]struct{}, len(perms))
	for _, perm := range perms {
		set[perm] = struct{}{}
	}
	return Role{Name: name, Permissions: set}
}
