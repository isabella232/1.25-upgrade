// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package applicationscaler

import (
	"github.com/juju/errors"

	"github.com/juju/1.25-upgrade/juju2/apiserver/common"
	"github.com/juju/1.25-upgrade/juju2/apiserver/facade"
	"github.com/juju/1.25-upgrade/juju2/state"
)

// This file contains untested shims to let us wrap state in a sensible
// interface and avoid writing tests that depend on mongodb. If you were
// to change any part of it so that it were no longer *obviously* and
// *trivially* correct, you would be Doing It Wrong.

func init() {
	common.RegisterStandardFacade("ApplicationScaler", 1, newFacade)
}

// newFacade wraps the supplied *state.State for the use of the Facade.
func newFacade(st *state.State, res facade.Resources, auth facade.Authorizer) (*Facade, error) {
	return NewFacade(backendShim{st}, res, auth)
}

// backendShim wraps a *State to implement Backend without pulling in direct
// mongodb dependencies. It would be awesome if we were to put this in state
// and test it properly there, where we have no choice but to test against
// mongodb anyway, but that's relatively low priority...
//
// ...so long as it stays simple, and the full functionality remains tested
// elsewhere.
type backendShim struct {
	st *state.State
}

// WatchScaledServices is part of the Backend interface.
func (shim backendShim) WatchScaledServices() state.StringsWatcher {
	return shim.st.WatchMinUnits()
}

// RescaleService is part of the Backend interface.
func (shim backendShim) RescaleService(name string) error {
	service, err := shim.st.Application(name)
	if err != nil {
		return errors.Trace(err)
	}
	return service.EnsureMinUnits()
}
