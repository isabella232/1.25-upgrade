// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package metricworker

import (
	"github.com/juju/errors"

	"github.com/juju/1.25-upgrade/juju2/api/metricsmanager"
	"github.com/juju/1.25-upgrade/juju2/worker"
)

// NewMetricsManager creates a runner that will run the metricsmanagement workers.
func newMetricsManager(client metricsmanager.MetricsManagerClient, notify chan string) (worker.Runner, error) {
	// TODO(fwereade): break this out into separate manifolds (with their own facades).

	// Periodic workers automatically retry so none should return an error. If they do
	// it's ok to restart them individually.
	isFatal := func(error) bool {
		return false
	}
	// All errors are equal
	moreImportant := func(error, error) bool {
		return false
	}

	runner := worker.NewRunner(isFatal, moreImportant, worker.RestartDelay)
	err := runner.StartWorker("sender", func() (worker.Worker, error) {
		return newSender(client, notify), nil
	})

	if err != nil {
		return nil, errors.Trace(err)
	}

	err = runner.StartWorker("cleanup", func() (worker.Worker, error) {
		return newCleanup(client, notify), nil
	})
	if err != nil {
		return nil, errors.Trace(err)
	}
	return runner, nil
}
