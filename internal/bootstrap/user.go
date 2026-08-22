package bootstrap

import (
	"errors"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// singleUsername is the fixed username of the one user tally ever creates.
// Authentication never goes through ezbookkeeping's username/password login
// path (tally uses a static bearer token instead), so this value is never
// exposed to or entered by the end user.
const singleUsername = "tally"
const singleUserEmail = "tally@localhost"
const singleUserNickname = "Tally"

// EnsureSingleUser returns the uid of tally's single user, creating it with a
// random, never-validated password if it doesn't exist yet. It is safe to
// call on every startup: an existing user is left untouched.
func EnsureSingleUser(defaultCurrency string) (int64, error) {
	ctx := core.NewNullContext()

	existingUser, err := services.Users.GetUserByUsername(ctx, singleUsername)
	if err == nil {
		return existingUser.Uid, nil
	} else if !errors.Is(err, errs.ErrUserNotFound) {
		return 0, err
	}

	randomPassword, err := utils.GetRandomString(32)
	if err != nil {
		return 0, err
	}

	user := &models.User{
		Username:        singleUsername,
		Email:           singleUserEmail,
		Nickname:        singleUserNickname,
		Password:        randomPassword,
		DefaultCurrency: defaultCurrency,
	}

	if err := services.Users.CreateUser(ctx, user, false); err != nil {
		return 0, err
	}

	return user.Uid, nil
}
