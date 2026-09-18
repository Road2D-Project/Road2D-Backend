package enum

// GroupRole is the per-group membership role, not a global account permission.
// JSON and Postgres store the lowercase name: "owner" | "admin" | "member".
//
// owner  — Unique creator of the group. May edit name/description, add/invite/
//          approve/kick members, appoint or demote admins, delete the group, or
//          transfer ownership. Cannot leave while still owner.
// admin  — May edit name/description and manage members (add, invite, approve,
//          reject, kick). Cannot delete the group or change the owner's role.
// member — May read the active roster, change their own nickname, leave, or
//          request to join. Cannot manage other members or group settings.
//
//go:generate go tool enumer -type=GroupRole -json -text -sql -trimprefix=GroupRole -transform=lower

type GroupRole int

const (
	GroupRoleOwner GroupRole = iota
	GroupRoleAdmin
	GroupRoleMember
)

func (role GroupRole) IsOwner() bool {
	return role == GroupRoleOwner
}

func (role GroupRole) IsAdmin() bool {
	return role == GroupRoleAdmin
}

func (role GroupRole) CanManageMembers() bool {
	return role == GroupRoleOwner || role == GroupRoleAdmin
}

func (role GroupRole) CanDeleteGroup() bool {
	return role == GroupRoleOwner
}

func (role GroupRole) CanChangeRoles() bool {
	return role == GroupRoleOwner
}

func (role GroupRole) CanEditGroup() bool {
	return role.CanManageMembers()
}
