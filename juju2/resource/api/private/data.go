// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package private

// TODO(ericsnow) Eliminate the apiserver dependencies, if possible.

import (
	"github.com/juju/1.25-upgrade/juju2/apiserver/params"
	"github.com/juju/1.25-upgrade/juju2/resource/api"
)

// ListResourcesArgs holds the arguments for an API request to list
// resources for an application. The application is implicit to the uniter-
// specific HTTP connection.
type ListResourcesArgs struct {
	// ResourceNames holds the names of the application's resources for
	// which information should be provided.
	ResourceNames []string `json:"resource-names"`
}

// ResourcesResult holds the resource info for a list of requested
// resources.
type ResourcesResult struct {
	params.ErrorResult

	// Resources is the list of results for the requested resources,
	// in the same order as requested.
	Resources []ResourceResult `json:"resources"`
}

// ResourceResult is the result for a single requested resource.
type ResourceResult struct {
	params.ErrorResult

	// Resource is the info for the requested resource.
	Resource api.Resource `json:"resource"`
}
