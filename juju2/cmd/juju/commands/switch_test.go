// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package commands

import (
	"errors"
	"os"

	"github.com/juju/cmd"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/1.25-upgrade/juju2/cmd/modelcmd"
	_ "github.com/juju/1.25-upgrade/juju2/juju"
	"github.com/juju/1.25-upgrade/juju2/jujuclient"
	"github.com/juju/1.25-upgrade/juju2/jujuclient/jujuclienttesting"
	coretesting "github.com/juju/1.25-upgrade/juju2/testing"
)

type SwitchSimpleSuite struct {
	coretesting.FakeJujuXDGDataHomeSuite
	testing.Stub
	store     *jujuclienttesting.MemStore
	stubStore *jujuclienttesting.StubStore
	onRefresh func()
}

var _ = gc.Suite(&SwitchSimpleSuite{})

func (s *SwitchSimpleSuite) SetUpTest(c *gc.C) {
	s.FakeJujuXDGDataHomeSuite.SetUpTest(c)
	s.Stub.ResetCalls()
	s.store = jujuclienttesting.NewMemStore()
	s.stubStore = jujuclienttesting.WrapClientStore(s.store)
	s.onRefresh = nil
}

func (s *SwitchSimpleSuite) refreshModels(store jujuclient.ClientStore, controllerName string) error {
	s.MethodCall(s, "RefreshModels", store, controllerName)
	if s.onRefresh != nil {
		s.onRefresh()
	}
	return s.NextErr()
}

func (s *SwitchSimpleSuite) run(c *gc.C, args ...string) (*cmd.Context, error) {
	cmd := &switchCommand{
		Store:         s.stubStore,
		RefreshModels: s.refreshModels,
	}
	return coretesting.RunCommand(c, modelcmd.WrapBase(cmd), args...)
}

func (s *SwitchSimpleSuite) TestNoArgs(c *gc.C) {
	_, err := s.run(c)
	c.Assert(err, gc.ErrorMatches, "no currently specified model")
}

func (s *SwitchSimpleSuite) TestNoArgsCurrentController(c *gc.C) {
	s.addController(c, "a-controller")
	s.store.CurrentControllerName = "a-controller"
	ctx, err := s.run(c)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stdout(ctx), gc.Equals, "a-controller\n")
}

func (s *SwitchSimpleSuite) TestUnknownControllerNameReturnsError(c *gc.C) {
	s.addController(c, "a-controller")
	s.store.CurrentControllerName = "a-controller"
	_, err := s.run(c, "another-controller:modela")
	c.Assert(err, gc.ErrorMatches, "controller another-controller not found")
}

func (s *SwitchSimpleSuite) TestNoArgsCurrentModel(c *gc.C) {
	s.addController(c, "a-controller")
	s.store.CurrentControllerName = "a-controller"
	s.store.Models["a-controller"] = &jujuclient.ControllerModels{
		Models:       map[string]jujuclient.ModelDetails{"admin/mymodel": {}},
		CurrentModel: "admin/mymodel",
	}
	ctx, err := s.run(c)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stdout(ctx), gc.Equals, "a-controller:admin/mymodel\n")
}

func (s *SwitchSimpleSuite) TestSwitchWritesCurrentController(c *gc.C) {
	s.addController(c, "a-controller")
	context, err := s.run(c, "a-controller")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Equals, " -> a-controller (controller)\n")
	s.stubStore.CheckCalls(c, []testing.StubCall{
		{"CurrentController", nil},
		{"ControllerByName", []interface{}{"a-controller"}},
		{"CurrentModel", []interface{}{"a-controller"}},
		{"SetCurrentController", []interface{}{"a-controller"}},
	})
}

func (s *SwitchSimpleSuite) TestSwitchWithCurrentController(c *gc.C) {
	s.store.CurrentControllerName = "old"
	s.addController(c, "new")
	context, err := s.run(c, "new")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Equals, "old (controller) -> new (controller)\n")
}

func (s *SwitchSimpleSuite) TestSwitchLocalControllerWithCurrent(c *gc.C) {
	s.store.CurrentControllerName = "old"
	s.addController(c, "new")
	context, err := s.run(c, "new")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Equals, "old (controller) -> new (controller)\n")
}

func (s *SwitchSimpleSuite) TestSwitchSameController(c *gc.C) {
	s.store.CurrentControllerName = "same"
	s.addController(c, "same")
	context, err := s.run(c, "same")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Equals, "same (controller) (no change)\n")
	s.stubStore.CheckCalls(c, []testing.StubCall{
		{"CurrentController", nil},
		{"CurrentModel", []interface{}{"same"}},
		{"ControllerByName", []interface{}{"same"}},
	})
}

func (s *SwitchSimpleSuite) TestSwitchControllerToModel(c *gc.C) {
	s.store.CurrentControllerName = "ctrl"
	s.addController(c, "ctrl")
	s.store.Models["ctrl"] = &jujuclient.ControllerModels{
		Models: map[string]jujuclient.ModelDetails{"admin/mymodel": {}},
	}
	context, err := s.run(c, "mymodel")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Equals, "ctrl (controller) -> ctrl:admin/mymodel\n")
	s.stubStore.CheckCalls(c, []testing.StubCall{
		{"CurrentController", nil},
		{"CurrentModel", []interface{}{"ctrl"}},
		{"ControllerByName", []interface{}{"mymodel"}},
		{"AccountDetails", []interface{}{"ctrl"}},
		{"SetCurrentModel", []interface{}{"ctrl", "admin/mymodel"}},
	})
	c.Assert(s.store.Models["ctrl"].CurrentModel, gc.Equals, "admin/mymodel")
}

func (s *SwitchSimpleSuite) TestSwitchControllerToModelDifferentController(c *gc.C) {
	s.store.CurrentControllerName = "old"
	s.addController(c, "new")
	s.store.Models["new"] = &jujuclient.ControllerModels{
		Models: map[string]jujuclient.ModelDetails{"admin/mymodel": {}},
	}
	context, err := s.run(c, "new:mymodel")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Equals, "old (controller) -> new:admin/mymodel\n")
	s.stubStore.CheckCalls(c, []testing.StubCall{
		{"CurrentController", nil},
		{"CurrentModel", []interface{}{"old"}},
		{"ControllerByName", []interface{}{"new:mymodel"}},
		{"ControllerByName", []interface{}{"new"}},
		{"AccountDetails", []interface{}{"new"}},
		{"SetCurrentModel", []interface{}{"new", "admin/mymodel"}},
		{"SetCurrentController", []interface{}{"new"}},
	})
	c.Assert(s.store.Models["new"].CurrentModel, gc.Equals, "admin/mymodel")
}

func (s *SwitchSimpleSuite) TestSwitchLocalControllerToModelDifferentController(c *gc.C) {
	s.store.CurrentControllerName = "old"
	s.addController(c, "new")
	s.store.Models["new"] = &jujuclient.ControllerModels{
		Models: map[string]jujuclient.ModelDetails{"admin/mymodel": {}},
	}
	context, err := s.run(c, "new:mymodel")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Equals, "old (controller) -> new:admin/mymodel\n")
	s.stubStore.CheckCalls(c, []testing.StubCall{
		{"CurrentController", nil},
		{"CurrentModel", []interface{}{"old"}},
		{"ControllerByName", []interface{}{"new:mymodel"}},
		{"ControllerByName", []interface{}{"new"}},
		{"AccountDetails", []interface{}{"new"}},
		{"SetCurrentModel", []interface{}{"new", "admin/mymodel"}},
		{"SetCurrentController", []interface{}{"new"}},
	})
	c.Assert(s.store.Models["new"].CurrentModel, gc.Equals, "admin/mymodel")
}

func (s *SwitchSimpleSuite) TestSwitchControllerToDifferentControllerCurrentModel(c *gc.C) {
	s.store.CurrentControllerName = "old"
	s.addController(c, "new")
	s.store.Models["new"] = &jujuclient.ControllerModels{
		Models:       map[string]jujuclient.ModelDetails{"admin/mymodel": {}},
		CurrentModel: "admin/mymodel",
	}
	context, err := s.run(c, "new:mymodel")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Equals, "old (controller) -> new:admin/mymodel\n")
	s.stubStore.CheckCalls(c, []testing.StubCall{
		{"CurrentController", nil},
		{"CurrentModel", []interface{}{"old"}},
		{"ControllerByName", []interface{}{"new:mymodel"}},
		{"ControllerByName", []interface{}{"new"}},
		{"AccountDetails", []interface{}{"new"}},
		{"SetCurrentModel", []interface{}{"new", "admin/mymodel"}},
		{"SetCurrentController", []interface{}{"new"}},
	})
}

func (s *SwitchSimpleSuite) TestSwitchToModelDifferentOwner(c *gc.C) {
	s.store.CurrentControllerName = "same"
	s.addController(c, "same")
	s.store.Models["same"] = &jujuclient.ControllerModels{
		Models: map[string]jujuclient.ModelDetails{
			"admin/mymodel":  {},
			"bianca/mymodel": {},
		},
		CurrentModel: "admin/mymodel",
	}
	context, err := s.run(c, "bianca/mymodel")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Equals, "same:admin/mymodel -> same:bianca/mymodel\n")
	c.Assert(s.store.Models["same"].CurrentModel, gc.Equals, "bianca/mymodel")
}

func (s *SwitchSimpleSuite) TestSwitchUnknownNoCurrentController(c *gc.C) {
	_, err := s.run(c, "unknown")
	c.Assert(err, gc.ErrorMatches, `"unknown" is not the name of a model or controller`)
	s.stubStore.CheckCalls(c, []testing.StubCall{
		{"CurrentController", nil},
		{"ControllerByName", []interface{}{"unknown"}},
	})
}

func (s *SwitchSimpleSuite) TestSwitchUnknownCurrentControllerRefreshModels(c *gc.C) {
	s.store.CurrentControllerName = "ctrl"
	s.addController(c, "ctrl")
	s.onRefresh = func() {
		s.store.Models["ctrl"] = &jujuclient.ControllerModels{
			Models: map[string]jujuclient.ModelDetails{"admin/unknown": {}},
		}
	}
	ctx, err := s.run(c, "unknown")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(ctx), gc.Equals, "ctrl (controller) -> ctrl:admin/unknown\n")
	s.CheckCallNames(c, "RefreshModels")
}

func (s *SwitchSimpleSuite) TestSwitchUnknownCurrentControllerRefreshModelsStillUnknown(c *gc.C) {
	s.store.CurrentControllerName = "ctrl"
	s.addController(c, "ctrl")
	_, err := s.run(c, "unknown")
	c.Assert(err, gc.ErrorMatches, `"unknown" is not the name of a model or controller`)
	s.CheckCallNames(c, "RefreshModels")
}

func (s *SwitchSimpleSuite) TestSwitchUnknownCurrentControllerRefreshModelsFails(c *gc.C) {
	s.store.CurrentControllerName = "ctrl"
	s.addController(c, "ctrl")
	s.SetErrors(errors.New("not very refreshing"))
	_, err := s.run(c, "unknown")
	c.Assert(err, gc.ErrorMatches, "refreshing models cache: not very refreshing")
	s.CheckCallNames(c, "RefreshModels")
}

func (s *SwitchSimpleSuite) TestSettingWhenEnvVarSet(c *gc.C) {
	os.Setenv("JUJU_MODEL", "using-model")
	_, err := s.run(c, "erewhemos-2")
	c.Assert(err, gc.ErrorMatches, `cannot switch when JUJU_MODEL is overriding the model \(set to "using-model"\)`)
}

func (s *SwitchSimpleSuite) TestTooManyParams(c *gc.C) {
	_, err := s.run(c, "foo", "bar")
	c.Assert(err, gc.ErrorMatches, `unrecognized args: ."bar".`)
}

func (s *SwitchSimpleSuite) addController(c *gc.C, name string) {
	s.store.Controllers[name] = jujuclient.ControllerDetails{}
	s.store.Accounts[name] = jujuclient.AccountDetails{
		User: "admin",
	}
}
