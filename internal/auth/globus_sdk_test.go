package auth

import (
	"testing"

	sdkgroups "github.com/scttfrdmn/globus-go-sdk/v4/pkg/services/groups"
)

// SDKDirectory must satisfy Directory — this is the whole point of the seam.
// A compile-time assertion so the contract can't silently drift.
var _ Directory = (*SDKDirectory)(nil)

func TestCallerRole(t *testing.T) {
	tests := []struct {
		name string
		in   sdkgroups.Group
		want string
	}{
		{
			name: "manager from my_memberships",
			in:   sdkgroups.Group{MyMemberships: []sdkgroups.Member{{Role: "manager"}}, IsMember: true},
			want: GroupRoleManager,
		},
		{
			name: "member from my_memberships",
			in:   sdkgroups.Group{MyMemberships: []sdkgroups.Member{{Role: "member"}}, IsMember: true},
			want: GroupRoleMember,
		},
		{
			name: "admin from my_memberships",
			in:   sdkgroups.Group{MyMemberships: []sdkgroups.Member{{Role: "admin"}}, IsGroupAdmin: true},
			want: GroupRoleAdmin,
		},
		{
			// The fidelity that #50 restored: without my_memberships the bools
			// cannot express manager, so a manager would look like a member.
			// The fallback is deliberately never worse than the old stand-in.
			name: "falls back to admin bool when my_memberships absent",
			in:   sdkgroups.Group{IsGroupAdmin: true},
			want: GroupRoleAdmin,
		},
		{
			name: "falls back to member when nothing set",
			in:   sdkgroups.Group{},
			want: GroupRoleMember,
		},
		{
			name: "skips an empty-role membership entry",
			in:   sdkgroups.Group{MyMemberships: []sdkgroups.Member{{Role: ""}, {Role: "manager"}}},
			want: GroupRoleManager,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := callerRole(tt.in); got != tt.want {
				t.Errorf("callerRole() = %q, want %q", got, tt.want)
			}
		})
	}
}
