package main

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"github.com/Aliizi83/vohu/internal/platform/user"
)

// Dev-only default credentials, same pattern as sample-golang-project's
// constants.DefaultUserUsername/.../DefaultUserPassword. Change these (or
// move to config) before this is ever exposed beyond localhost.
const (
	defaultAdminUsername = "admin"
	defaultAdminEmail    = "admin@example.com"
	defaultAdminPassword = "change-me-now"

	defaultAdminRoleName = "admin"
)

// knownPermissions is every permission key the platform currently defines.
// New modules add their keys here as they're built, and the admin role
// always gets all of them.
var knownPermissions = []string{
	"user:create",
	"user:read",
	"rbac:manage",
}

func seedDefaultAdmin(userService user.Service, rbacService rbac.Service) error {
	ctx := context.Background()

	role, err := rbacService.EnsureRole(ctx, defaultAdminRoleName)
	if err != nil {
		return err
	}

	for _, key := range knownPermissions {
		permission, err := rbacService.EnsurePermission(ctx, key)
		if err != nil {
			return err
		}
		if err := rbacService.GrantPermissionToRole(ctx, role.ID, permission.ID); err != nil {
			return err
		}
	}

	admin, err := userService.GetByUsername(ctx, defaultAdminUsername)
	if err != nil {
		if !errors.Is(err, user.ErrNotFound) {
			return err
		}

		res, err := userService.Register(ctx, user.CreateUserRequest{
			Username: defaultAdminUsername,
			Email:    defaultAdminEmail,
			Password: defaultAdminPassword,
		})
		if err != nil {
			return err
		}

		admin, err = userService.GetByID(ctx, res.ID)
		if err != nil {
			return err
		}
	}

	return rbacService.AssignRoleToUser(ctx, admin.ID, role.ID)
}
