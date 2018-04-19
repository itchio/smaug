//+build windows

package fuji

import (
	"fmt"
	"time"

	"github.com/itchio/butler/comm"
	"github.com/itchio/ox/winox"
	"github.com/itchio/wharf/state"
	"github.com/pkg/errors"
)

// Setup ensures that fuji can run properly, ie.: that the
// registry contains credentials for the sandbox user, and
// that we can log in using those credentials.
//
// If any of those checks fail, it will need Administrator
// privileges to continue.
//
// It'll create a new user account, with a randomly generated
// username and pasword and store the credentials in the registry,
// in a location specified by settings.
func (i *instance) Setup(consumer *state.Consumer) error {
	startTime := time.Now()

	nullConsumer := &state.Consumer{}

	err := i.Check(nullConsumer)
	if err == nil {
		consumer.Statf("Already set up properly!")
		return nil
	}

	var username string
	var password string

	existingCreds, err := i.getCredentials()
	if err != nil {
		return errors.WithStack(err)
	}
	username = existingCreds.Username
	if username != "" {
		comm.Opf("Trying to salvage existing account (%s)....", username)
		password = generatePassword()
		err = winox.ForceSetPassword(username, password)
		if err != nil {
			consumer.Warnf("Could not force password: %+v", err)
			username = ""
		} else {
			comm.Statf("Forced password successfully")
		}
	}

	if username == "" {
		username = fmt.Sprintf("itch-player-%x", time.Now().Unix())
		comm.Opf("Generated username (%s)", username)

		password = generatePassword()
		comm.Opf("Generated password (%s)", password)

		comment := "itch.io sandbox user"

		comm.Opf("Adding user...")

		err = winox.AddUser(username, password, comment)
		if err != nil {
			return errors.WithStack(err)
		}
	}

	comm.Opf("Removing from Users group (so it doesn't show up as a login option)...")

	err = winox.RemoveUserFromUsersGroup(username)
	if err != nil {
		return errors.WithStack(err)
	}

	comm.Opf("Loading profile for the first time (to create some directories)...")

	err = winox.LoadProfileOnce(username, ".", password)
	if err != nil {
		return errors.WithStack(err)
	}

	comm.Opf("Saving to credentials registry...")

	creds := &Credentials{
		Username: username,
		Password: password,
	}
	err = i.saveCredentials(creds)
	if err != nil {
		return errors.WithStack(err)
	}

	comm.Statf("All done! (in %s)", time.Since(startTime))

	return nil
}
