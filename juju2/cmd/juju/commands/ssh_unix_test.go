// Copyright 2014 Canonical Ltd.
// Copyright 2014 Cloudbase Solutions SRL
// Licensed under the AGPLv3, see LICENCE file for details.

// +build !windows

package commands

import (
	"fmt"
	"reflect"

	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/1.25-upgrade/juju2/apiserver"
	jujussh "github.com/juju/1.25-upgrade/juju2/network/ssh"
	coretesting "github.com/juju/1.25-upgrade/juju2/testing"
)

type SSHSuite struct {
	SSHCommonSuite
}

var _ = gc.Suite(&SSHSuite{})

var sshTests = []struct {
	about       string
	args        []string
	hostChecker jujussh.ReachableChecker
	forceAPIv1  bool
	expected    argsSpec
	expectedErr string
}{
	{
		about:       "connect to machine 0 (api v1)",
		args:        []string{"0"},
		hostChecker: validAddresses("0.private", "0.public"),
		forceAPIv1:  true,
		expected: argsSpec{
			hostKeyChecking: "yes",
			knownHosts:      "0",
			enablePty:       true,
			args:            "ubuntu@0.public",
		},
	},
	{
		about:       "connect to machine 0 (api v2)",
		args:        []string{"0"},
		hostChecker: validAddresses("0.private", "0.public", "0.1.2.3"), // set by setAddresses() and setLinkLayerDevicesAddresses()
		forceAPIv1:  false,
		expected: argsSpec{
			hostKeyChecking: "yes",
			knownHosts:      "0",
			enablePty:       true,
			argsMatch:       `ubuntu@0.(public|private|1\.2\.3)`, // can be any of the 3
		},
	},
	{
		about:       "connect to machine 0 and pass extra arguments",
		args:        []string{"0", "uname", "-a"},
		hostChecker: validAddresses("0.public"),
		expected: argsSpec{
			hostKeyChecking: "yes",
			knownHosts:      "0",
			enablePty:       true,
			args:            "ubuntu@0.public uname -a",
		},
	},
	{
		about:       "connect to machine 0 with no pseudo-tty",
		args:        []string{"--pty=false", "0"},
		hostChecker: validAddresses("0.public"),
		expected: argsSpec{
			hostKeyChecking: "yes",
			knownHosts:      "0",
			enablePty:       false,
			args:            "ubuntu@0.public",
		},
	},
	{
		about:       "connect to machine 1 which has no SSH host keys",
		args:        []string{"1"},
		hostChecker: validAddresses("1.public"),
		expectedErr: `retrieving SSH host keys for "1": keys not found`,
	},
	{
		about:       "connect to machine 1 which has no SSH host keys, no host key checks",
		args:        []string{"--no-host-key-checks", "1"},
		hostChecker: validAddresses("1.public"),
		expected: argsSpec{
			hostKeyChecking: "no",
			knownHosts:      "null",
			enablePty:       true,
			args:            "ubuntu@1.public",
		},
	},
	{
		about:       "connect to arbitrary (non-entity) hostname",
		args:        []string{"foo@some.host"},
		hostChecker: validAddresses("some.host"),
		expected: argsSpec{
			// In this case, use the user's own known_hosts and own
			// StrictHostKeyChecking config.
			hostKeyChecking: "",
			knownHosts:      "",
			enablePty:       true,
			args:            "foo@some.host",
		},
	},
	{
		about:       "connect to unit mysql/0",
		args:        []string{"mysql/0"},
		hostChecker: validAddresses("0.public"),
		expected: argsSpec{
			hostKeyChecking: "yes",
			knownHosts:      "0",
			enablePty:       true,
			args:            "ubuntu@0.public",
		},
	},
	{
		about:       "connect to unit mysql/0 as the mongo user",
		args:        []string{"mongo@mysql/0"},
		hostChecker: validAddresses("0.public"),
		expected: argsSpec{
			hostKeyChecking: "yes",
			knownHosts:      "0",
			enablePty:       true,
			args:            "mongo@0.public",
		},
	},
	{
		about:       "connect to unit mysql/0 and pass extra arguments",
		args:        []string{"mysql/0", "ls", "/"},
		hostChecker: validAddresses("0.public"),
		expected: argsSpec{
			hostKeyChecking: "yes",
			knownHosts:      "0",
			enablePty:       true,
			args:            "ubuntu@0.public ls /",
		},
	},
	{
		about:       "connect to unit mysql/0 with proxy (api v1)",
		args:        []string{"--proxy=true", "mysql/0"},
		hostChecker: validAddresses("0.private", "0.public"),
		forceAPIv1:  true,
		expected: argsSpec{
			hostKeyChecking: "yes",
			knownHosts:      "0",
			enablePty:       true,
			withProxy:       true,
			args:            "ubuntu@0.private",
		},
	},
	{
		about:       "connect to unit mysql/0 with proxy (api v2)",
		args:        []string{"--proxy=true", "mysql/0"},
		hostChecker: validAddresses("0.private", "0.public", "0.1.2.3"), // set by setAddresses() and setLinkLayerDevicesAddresses()
		forceAPIv1:  false,
		expected: argsSpec{
			hostKeyChecking: "yes",
			knownHosts:      "0",
			enablePty:       true,
			withProxy:       true,
			argsMatch:       `ubuntu@0.(public|private|1\.2\.3)`, // can be any of the 3
		},
	},
}

func (s *SSHSuite) TestSSHCommand(c *gc.C) {
	s.setupModel(c)

	for i, t := range sshTests {
		c.Logf("test %d: %s -> %s", i, t.about, t.args)

		s.setHostChecker(t.hostChecker)
		s.setForceAPIv1(t.forceAPIv1)

		ctx, err := coretesting.RunCommand(c, newSSHCommand(s.hostChecker), t.args...)
		if t.expectedErr != "" {
			c.Check(err, gc.ErrorMatches, t.expectedErr)
		} else {
			c.Check(err, jc.ErrorIsNil)
			c.Check(coretesting.Stderr(ctx), gc.Equals, "")
			stdout := coretesting.Stdout(ctx)
			t.expected.check(c, stdout)
		}
	}
}

func (s *SSHSuite) TestSSHCommandModelConfigProxySSH(c *gc.C) {
	s.setupModel(c)

	// Setting proxy-ssh=true in the environment overrides --proxy.
	err := s.State.UpdateModelConfig(map[string]interface{}{"proxy-ssh": true}, nil, nil)
	c.Assert(err, jc.ErrorIsNil)

	s.setForceAPIv1(true)
	s.setHostChecker(validAddresses("0.private", "0.public", "0.1.2.3"))

	ctx, err := coretesting.RunCommand(c, newSSHCommand(s.hostChecker), "0")
	c.Check(err, jc.ErrorIsNil)
	c.Check(coretesting.Stderr(ctx), gc.Equals, "")
	expectedArgs := argsSpec{
		hostKeyChecking: "yes",
		knownHosts:      "0",
		enablePty:       true,
		withProxy:       true,
		args:            "ubuntu@0.private",
	}
	expectedArgs.check(c, coretesting.Stdout(ctx))

	s.setForceAPIv1(false)
	ctx, err = coretesting.RunCommand(c, newSSHCommand(s.hostChecker), "0")
	c.Check(err, jc.ErrorIsNil)
	c.Check(coretesting.Stderr(ctx), gc.Equals, "")
	expectedArgs.argsMatch = `ubuntu@0.(public|private|1\.2\.3)` // can be any of the 3 with api v2.
	expectedArgs.check(c, coretesting.Stdout(ctx))

}

func (s *SSHSuite) TestSSHWillWorkInUpgrade(c *gc.C) {
	// Check the API client interface used by "juju ssh" against what
	// the API server will allow during upgrades. Ensure that the API
	// server will allow all required API calls to support SSH.
	type concrete struct {
		sshAPIClient
	}
	t := reflect.TypeOf(concrete{})
	for i := 0; i < t.NumMethod(); i++ {
		name := t.Method(i).Name

		// Close isn't an API method.
		if name == "Close" {
			continue
		}
		c.Logf("checking %q", name)
		c.Check(apiserver.IsMethodAllowedDuringUpgrade("SSHClient", name), jc.IsTrue)
	}
}

/// XXX(jam): 2017-01-25 do we need these functions anymore? We don't really
//support ssh'ing to V1 anymore
func (s *SSHSuite) TestSSHCommandHostAddressRetryAPIv1(c *gc.C) {
	// Start with nothing valid to connect to.
	s.setHostChecker(validAddresses())
	s.setForceAPIv1(true)

	s.testSSHCommandHostAddressRetry(c, false)
}

func (s *SSHSuite) TestSSHCommandHostAddressRetryAPIv2(c *gc.C) {
	s.setHostChecker(validAddresses())
	s.setForceAPIv1(false)

	s.testSSHCommandHostAddressRetry(c, false)
}

func (s *SSHSuite) TestSSHCommandHostAddressRetryProxyAPIv1(c *gc.C) {
	s.setHostChecker(validAddresses())
	s.setForceAPIv1(true)

	s.testSSHCommandHostAddressRetry(c, true)
}

func (s *SSHSuite) TestSSHCommandHostAddressRetryProxyAPIv2(c *gc.C) {
	s.setHostChecker(validAddresses())
	s.setForceAPIv1(false)

	s.testSSHCommandHostAddressRetry(c, true)
}

func (s *SSHSuite) testSSHCommandHostAddressRetry(c *gc.C, proxy bool) {
	m := s.Factory.MakeMachine(c, nil)
	s.setKeys(c, m)

	called := 0
	attemptStarter := &callbackAttemptStarter{next: func() bool {
		called++
		return called < 2
	}}
	restorer := testing.PatchValue(&sshHostFromTargetAttemptStrategy, attemptStarter)
	defer restorer.Restore()

	// Ensure that the ssh command waits for a public (private with proxy=true)
	// address, or the attempt strategy's Done method returns false.
	args := []string{"--proxy=" + fmt.Sprint(proxy), "0"}
	_, err := coretesting.RunCommand(c, newSSHCommand(s.hostChecker), args...)
	c.Assert(err, gc.ErrorMatches, `no .+ address\(es\)`)
	c.Assert(called, gc.Equals, 2)

	s.setHostChecker(validAddresses("0.private", "0.public"))

	called = 0
	attemptStarter.next = func() bool {
		called++
		if called > 1 {
			s.setAddresses(c, m)
		}
		return true
	}

	_, err = coretesting.RunCommand(c, newSSHCommand(s.hostChecker), args...)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(called, gc.Equals, 2)
}

type callbackAttemptStarter struct {
	next func() bool
}

func (s *callbackAttemptStarter) Start() attempt {
	return callbackAttempt{next: s.next}
}

type callbackAttempt struct {
	next func() bool
}

func (a callbackAttempt) Next() bool {
	return a.next()
}
