package core

import (
	"context"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"gitlab.com/udevs/dbdiagram/backend/config"
	"gitlab.com/udevs/dbdiagram/backend/internal/apperr"
	"gitlab.com/udevs/dbdiagram/backend/internal/model"
)

type UserService struct {
	deps Deps
}

func (s *UserService) Get(ctx context.Context, guid string) (*model.User, error) {
	u, err := s.deps.Storage.User().Get(ctx, guid)
	if err != nil && isNotFound(err) {
		// A token for an account that no longer exists is simply not valid.
		return nil, apperr.Unauthenticated("account not found")
	}

	return u, err
}

// ByID is the gateway other modules use to show a user (an owner row). Unlike
// Get it keeps the plain NotFound: nobody is being authenticated here.
func (s *UserService) ByID(ctx context.Context, guid string) (*model.User, error) {
	return s.deps.Storage.User().Get(ctx, guid)
}

// ByEmail is the gateway other modules use to turn a typed email into a user
// (sharing). A missing account is a plain NotFound the caller words itself.
func (s *UserService) ByEmail(ctx context.Context, email string) (*model.User, error) {
	email, err := cleanEmail(email)
	if err != nil {
		return nil, err
	}

	return s.deps.Storage.User().GetByEmail(ctx, email)
}

func (s *UserService) UpdateName(ctx context.Context, guid, name string) (*model.User, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > config.MaxNameLength {
		return nil, apperr.InvalidArgument("name must be 1 to %d characters", config.MaxNameLength)
	}

	return s.deps.Storage.User().UpdateName(ctx, guid, name)
}

// ChangePassword needs the current password even though the caller is logged
// in: a stolen access token alone must not be enough to take over the account.
// Every other login is ended; the one making the change stays.
func (s *UserService) ChangePassword(ctx context.Context, guid, currentPassword, newPassword, keepFamily string) error {
	if err := checkPassword(newPassword); err != nil {
		return err
	}

	hash, err := s.deps.Storage.User().GetPasswordHash(ctx, guid)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(currentPassword)) != nil {
		return apperr.PermissionDenied("current password is wrong")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.deps.Cfg.BcryptCost)
	if err != nil {
		return apperr.Internal(err, "hash password")
	}
	if err := s.deps.Storage.User().UpdatePassword(ctx, guid, string(newHash)); err != nil {
		return err
	}

	return s.deps.Storage.Session().RevokeAllForUser(ctx, guid, keepFamily)
}
