// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package lifeflag

import (
	"gopkg.in/juju/names.v2"

	"github.com/juju/1.25-upgrade/juju2/apiserver/common"
	"github.com/juju/1.25-upgrade/juju2/apiserver/facade"
	"github.com/juju/1.25-upgrade/juju2/state"
)

type Backend interface {
	ModelUUID() string
	state.EntityFinder
}

func NewFacade(backend Backend, resources facade.Resources, authorizer facade.Authorizer) (*Facade, error) {
	if !authorizer.AuthController() {
		return nil, common.ErrPerm
	}
	expect := names.NewModelTag(backend.ModelUUID())
	getCanAccess := func() (common.AuthFunc, error) {
		return func(tag names.Tag) bool {
			return tag == expect
		}, nil
	}
	life := common.NewLifeGetter(backend, getCanAccess)
	watch := common.NewAgentEntityWatcher(backend, resources, getCanAccess)
	return &Facade{
		LifeGetter:         life,
		AgentEntityWatcher: watch,
	}, nil
}

type Facade struct {
	*common.LifeGetter
	*common.AgentEntityWatcher
}
