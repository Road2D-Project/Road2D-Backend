package enum

import "testing"

func TestGroupRolePermissions(t *testing.T) {
	if !GroupRoleOwner.IsOwner() || GroupRoleAdmin.IsOwner() {
		t.Fatal("owner check")
	}
	if !GroupRoleOwner.CanManageMembers() || !GroupRoleAdmin.CanManageMembers() || GroupRoleMember.CanManageMembers() {
		t.Fatal("manage members")
	}
	if !GroupRoleOwner.CanDeleteGroup() || GroupRoleAdmin.CanDeleteGroup() {
		t.Fatal("delete group is owner-only")
	}
	if !GroupRoleOwner.CanEditGroup() || !GroupRoleAdmin.CanEditGroup() || GroupRoleMember.CanEditGroup() {
		t.Fatal("edit group is owner/admin")
	}
}

func TestMembershipStatusRejoin(t *testing.T) {
	if !MembershipLeft.CanRejoin() || !MembershipRejected.CanRejoin() || !MembershipKicked.CanRejoin() {
		t.Fatal("left/rejected/kicked may rejoin")
	}
	if MembershipActive.CanRejoin() || MembershipPending.CanRejoin() || MembershipInvited.CanRejoin() {
		t.Fatal("active/pending/invited must not use CanRejoin")
	}
	if !MembershipActive.IsActive() || MembershipPending.IsActive() {
		t.Fatal("active check")
	}
}
