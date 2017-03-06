// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package common_test

import (
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/juju/names.v2"

	"github.com/juju/1.25-upgrade/juju2/apiserver/common"
	"github.com/juju/1.25-upgrade/juju2/apiserver/params"
	apiservertesting "github.com/juju/1.25-upgrade/juju2/apiserver/testing"
	"github.com/juju/1.25-upgrade/juju2/state"
	"github.com/juju/1.25-upgrade/juju2/testing"
)

type modelMachinesWatcherSuite struct {
	testing.BaseSuite
}

var _ = gc.Suite(&modelMachinesWatcherSuite{})

type fakeModelMachinesWatcher struct {
	state.ModelMachinesWatcher
	initial []string
}

func (f *fakeModelMachinesWatcher) WatchModelMachines() state.StringsWatcher {
	changes := make(chan []string, 1)
	// Simulate initial event.
	changes <- f.initial
	return &fakeStringsWatcher{changes}
}

func (s *modelMachinesWatcherSuite) TestWatchModelMachines(c *gc.C) {
	authorizer := apiservertesting.FakeAuthorizer{
		Tag:        names.NewMachineTag("0"),
		Controller: true,
	}
	resources := common.NewResources()
	s.AddCleanup(func(_ *gc.C) { resources.StopAll() })
	e := common.NewModelMachinesWatcher(
		&fakeModelMachinesWatcher{initial: []string{"foo"}},
		resources,
		authorizer,
	)
	result, err := e.WatchModelMachines()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(result, jc.DeepEquals, params.StringsWatchResult{"1", []string{"foo"}, nil})
	c.Assert(resources.Count(), gc.Equals, 1)
}

func (s *modelMachinesWatcherSuite) TestWatchAuthError(c *gc.C) {
	authorizer := apiservertesting.FakeAuthorizer{
		Tag:        names.NewMachineTag("1"),
		Controller: false,
	}
	resources := common.NewResources()
	s.AddCleanup(func(_ *gc.C) { resources.StopAll() })
	e := common.NewModelMachinesWatcher(
		&fakeModelMachinesWatcher{},
		resources,
		authorizer,
	)
	_, err := e.WatchModelMachines()
	c.Assert(err, gc.ErrorMatches, "permission denied")
	c.Assert(resources.Count(), gc.Equals, 0)
}
