// Copyright 2014 Canonical Ltd. All rights reserved.
// Licensed under the AGPLv3, see LICENCE file for details.

package authentication

import (
	"github.com/juju/errors"
	"github.com/juju/names"
	"gopkg.in/macaroon-bakery.v0/bakery"

	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/state"
)

// UserAuthenticator performs authentication for users.
type UserAuthenticator struct {
	AgentAuthenticator
}

var _ EntityAuthenticator = (*UserAuthenticator)(nil)

// TODO: MacaroonAuthenticator
// TODO: Issue a macaroon or return pre-generated macaroon -> return ErrDischareReq
//       - where should macaroons be stored? they shouldn't, except in mem (default bakery).
//       - when should they be created?
//         - root key generated on server startup. not reused among replica servers.
//         - macaroon issued on demand, reuse same root key
//       - how do we choose user tag coming in?
//         - special username? placeholder? empty username. need to return with
//           resolved entity in state so some refactoring of authenticators reqd?
// TODO: Verify macaroons -> logged in

// Authenticate authenticates the provided entity and returns an error on authentication failure.
/*
//TODO Probably don't need this type anymore
func (u *UserAuthenticator) Authenticate(req params.LoginRequest) (*state.Entity, error) {
	return u.AgentAuthenticator.Authenticate(req)
}
*/

// MacaroonAuthenticator performs authentication for users using macaroons.
type MacaroonAuthenticator struct {
	Service *bakery.Service
}

var _ EntityAuthenticator = (*MacaroonAuthenticator)(nil)

// Authenticate authenticates the provided entity and returns an error on authentication failure.
func (m *MacaroonAuthenticator) Authenticate(entityFinder EntityFinder, tag names.Tag, req params.LoginRequest) (state.Entity, error) {
	if len(req.Macaroons) == 0 {
		return nil, errors.Errorf("discharge required")
	}
	return nil, nil
}

