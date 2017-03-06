// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package apiserver

import (
	"reflect"

	"github.com/juju/errors"

	"github.com/juju/1.25-upgrade/juju2/apiserver/common"
	"github.com/juju/1.25-upgrade/juju2/apiserver/common/storagecommon"
	"github.com/juju/1.25-upgrade/juju2/apiserver/facade"
	"github.com/juju/1.25-upgrade/juju2/apiserver/params"
	"github.com/juju/1.25-upgrade/juju2/controller"
	"github.com/juju/1.25-upgrade/juju2/core/migration"
	"github.com/juju/1.25-upgrade/juju2/network"
	"github.com/juju/1.25-upgrade/juju2/state"
)

func init() {
	common.RegisterFacade(
		"AllWatcher", 1, NewAllWatcher,
		reflect.TypeOf((*SrvAllWatcher)(nil)),
	)
	// Note: AllModelWatcher uses the same infrastructure as AllWatcher
	// but they are get under separate names as it possible the may
	// diverge in the future (especially in terms of authorisation
	// checks).
	common.RegisterFacade(
		"AllModelWatcher", 2, NewAllWatcher,
		reflect.TypeOf((*SrvAllWatcher)(nil)),
	)
	common.RegisterFacade(
		"NotifyWatcher", 1, newNotifyWatcher,
		reflect.TypeOf((*srvNotifyWatcher)(nil)),
	)
	common.RegisterFacade(
		"StringsWatcher", 1, newStringsWatcher,
		reflect.TypeOf((*srvStringsWatcher)(nil)),
	)
	common.RegisterFacade(
		"RemoteApplicationWatcher", 1, newRemoteApplicationWatcher,
		reflect.TypeOf((*srvRemoteApplicationWatcher)(nil)),
	)
	common.RegisterFacade(
		"RemoteRelationsWatcher", 1, newRemoteRelationsWatcher,
		reflect.TypeOf((*srvRemoteRelationsWatcher)(nil)),
	)
	common.RegisterFacade(
		"RelationUnitsWatcher", 1, newRelationUnitsWatcher,
		reflect.TypeOf((*srvRelationUnitsWatcher)(nil)),
	)
	common.RegisterFacade(
		"VolumeAttachmentsWatcher", 2, newVolumeAttachmentsWatcher,
		reflect.TypeOf((*srvMachineStorageIdsWatcher)(nil)),
	)
	common.RegisterFacade(
		"FilesystemAttachmentsWatcher", 2, newFilesystemAttachmentsWatcher,
		reflect.TypeOf((*srvMachineStorageIdsWatcher)(nil)),
	)
	common.RegisterFacade(
		"EntityWatcher", 2, newEntitiesWatcher,
		reflect.TypeOf((*srvEntitiesWatcher)(nil)),
	)
	common.RegisterFacade(
		"MigrationStatusWatcher", 1, newMigrationStatusWatcher,
		reflect.TypeOf((*srvMigrationStatusWatcher)(nil)),
	)
}

// NewAllWatcher returns a new API server endpoint for interacting
// with a watcher created by the WatchAll and WatchAllModels API calls.
func NewAllWatcher(context facade.Context) (facade.Facade, error) {
	id := context.ID()
	auth := context.Auth()
	resources := context.Resources()

	if !auth.AuthClient() {
		// Note that we don't need to check specific permissions
		// here, as the AllWatcher can only do anything if the
		// watcher resource has already been created, so we can
		// rely on the permission check there to ensure that
		// this facade can't do anything it shouldn't be allowed
		// to.
		//
		// This is useful because the AllWatcher is reused for
		// both the WatchAll (requires model access rights) and
		// the WatchAllModels (requring controller superuser
		// rights) API calls.
		return nil, common.ErrPerm
	}
	watcher, ok := resources.Get(id).(*state.Multiwatcher)
	if !ok {
		return nil, common.ErrUnknownWatcher
	}
	return &SrvAllWatcher{
		watcherCommon: newWatcherCommon(context),
		watcher:       watcher,
	}, nil
}

type watcherCommon struct {
	id        string
	resources facade.Resources
	dispose   func()
}

func newWatcherCommon(context facade.Context) watcherCommon {
	return watcherCommon{
		context.ID(),
		context.Resources(),
		context.Dispose,
	}
}

// Stop stops the watcher.
func (w *watcherCommon) Stop() error {
	w.dispose()
	return w.resources.Stop(w.id)
}

// SrvAllWatcher defines the API methods on a state.Multiwatcher.
// which watches any changes to the state. Each client has its own
// current set of watchers, stored in resources. It is used by both
// the AllWatcher and AllModelWatcher facades.
type SrvAllWatcher struct {
	watcherCommon
	watcher *state.Multiwatcher
}

func (aw *SrvAllWatcher) Next() (params.AllWatcherNextResults, error) {
	deltas, err := aw.watcher.Next()
	return params.AllWatcherNextResults{
		Deltas: deltas,
	}, err
}

// srvNotifyWatcher defines the API access to methods on a state.NotifyWatcher.
// Each client has its own current set of watchers, stored in resources.
type srvNotifyWatcher struct {
	watcherCommon
	watcher state.NotifyWatcher
}

func isAgent(auth facade.Authorizer) bool {
	return auth.AuthMachineAgent() || auth.AuthUnitAgent()
}

func newNotifyWatcher(context facade.Context) (facade.Facade, error) {
	id := context.ID()
	auth := context.Auth()
	resources := context.Resources()

	if !isAgent(auth) {
		return nil, common.ErrPerm
	}
	watcher, ok := resources.Get(id).(state.NotifyWatcher)
	if !ok {
		return nil, common.ErrUnknownWatcher
	}
	return &srvNotifyWatcher{
		watcherCommon: newWatcherCommon(context),
		watcher:       watcher,
	}, nil
}

// Next returns when a change has occurred to the
// entity being watched since the most recent call to Next
// or the Watch call that created the NotifyWatcher.
func (w *srvNotifyWatcher) Next() error {
	if _, ok := <-w.watcher.Changes(); ok {
		return nil
	}
	err := w.watcher.Err()
	if err == nil {
		err = common.ErrStoppedWatcher
	}
	return err
}

// srvStringsWatcher defines the API for methods on a state.StringsWatcher.
// Each client has its own current set of watchers, stored in resources.
// srvStringsWatcher notifies about changes for all entities of a given kind,
// sending the changes as a list of strings.
type srvStringsWatcher struct {
	watcherCommon
	watcher state.StringsWatcher
}

func newStringsWatcher(context facade.Context) (facade.Facade, error) {
	id := context.ID()
	auth := context.Auth()
	resources := context.Resources()

	if !isAgent(auth) {
		return nil, common.ErrPerm
	}
	watcher, ok := resources.Get(id).(state.StringsWatcher)
	if !ok {
		return nil, common.ErrUnknownWatcher
	}
	return &srvStringsWatcher{
		watcherCommon: newWatcherCommon(context),
		watcher:       watcher,
	}, nil
}

// Next returns when a change has occured to an entity of the
// collection being watched since the most recent call to Next
// or the Watch call that created the srvStringsWatcher.
func (w *srvStringsWatcher) Next() (params.StringsWatchResult, error) {
	if changes, ok := <-w.watcher.Changes(); ok {
		return params.StringsWatchResult{
			Changes: changes,
		}, nil
	}
	err := w.watcher.Err()
	if err == nil {
		err = common.ErrStoppedWatcher
	}
	return params.StringsWatchResult{}, err
}

// srvRelationUnitsWatcher defines the API wrapping a state.RelationUnitsWatcher.
// It notifies about units entering and leaving the scope of a RelationUnit,
// and changes to the settings of those units known to have entered.
type srvRelationUnitsWatcher struct {
	watcherCommon
	watcher state.RelationUnitsWatcher
}

func newRelationUnitsWatcher(context facade.Context) (facade.Facade, error) {
	id := context.ID()
	auth := context.Auth()
	resources := context.Resources()

	if !isAgent(auth) {
		return nil, common.ErrPerm
	}
	watcher, ok := resources.Get(id).(state.RelationUnitsWatcher)
	if !ok {
		return nil, common.ErrUnknownWatcher
	}
	return &srvRelationUnitsWatcher{
		watcherCommon: newWatcherCommon(context),
		watcher:       watcher,
	}, nil
}

// Next returns when a change has occured to an entity of the
// collection being watched since the most recent call to Next
// or the Watch call that created the srvRelationUnitsWatcher.
func (w *srvRelationUnitsWatcher) Next() (params.RelationUnitsWatchResult, error) {
	if changes, ok := <-w.watcher.Changes(); ok {
		return params.RelationUnitsWatchResult{
			Changes: changes,
		}, nil
	}
	err := w.watcher.Err()
	if err == nil {
		err = common.ErrStoppedWatcher
	}
	return params.RelationUnitsWatchResult{}, err
}

// srvRemoteApplicationWatcher will sends changes to relations a service
// is involved in, including changes to the units involved in those
// relations, and their settings.
type srvRemoteApplicationWatcher struct {
	watcherCommon
	watcher state.RemoteApplicationWatcher
}

func newRemoteApplicationWatcher(context facade.Context) (facade.Facade, error) {
	id := context.ID()
	resources := context.Resources()
	auth := context.Auth()

	if !auth.AuthController() {
		return nil, common.ErrPerm
	}
	watcher, ok := resources.Get(id).(state.RemoteApplicationWatcher)
	if !ok {
		return nil, common.ErrUnknownWatcher
	}
	return &srvRemoteApplicationWatcher{
		watcherCommon: newWatcherCommon(context),
		watcher:       watcher,
	}, nil
}

// Next returns when a change has occured to an entity of the
// collection being watched since the most recent call to Next
// or the Watch call that created the srvRemoteApplicationWatcher.
func (w *srvRemoteApplicationWatcher) Next() (params.RemoteApplicationWatchResult, error) {
	if change, ok := <-w.watcher.Changes(); ok {
		return params.RemoteApplicationWatchResult{
			Change: &change,
		}, nil
	}
	err := w.watcher.Err()
	if err == nil {
		err = common.ErrStoppedWatcher
	}
	return params.RemoteApplicationWatchResult{}, err
}

// srvRemoteRelationsWatcher defines the API wrapping a RemoteRelationsWatcher.
// This watcher notifies about:
//  - addition and removal of relations of relations to remote applications
//  - lifecycle changes to relations of relations to remote applications
//  - settings of relation units changing
//  - units departing the relation (joining is implicit in seeing new settings)
type srvRemoteRelationsWatcher struct {
	watcherCommon
	watcher state.RemoteRelationsWatcher
}

// RemoteRelationsWatcher is a watcher that reports on changes to relations
// and relation units related to those relations for a specified service.
type RemoteRelationsWatcher interface {
	Changes() <-chan params.RemoteRelationsChange
	Err() error
	Stop() error
}

func newRemoteRelationsWatcher(context facade.Context) (facade.Facade, error) {
	id := context.ID()
	resources := context.Resources()
	auth := context.Auth()

	if !auth.AuthController() {
		return nil, common.ErrPerm
	}
	watcher, ok := resources.Get(id).(state.RemoteRelationsWatcher)
	if !ok {
		return nil, common.ErrUnknownWatcher
	}
	return &srvRemoteRelationsWatcher{
		watcherCommon: newWatcherCommon(context),
		watcher:       watcher,
	}, nil
}

// Next returns when a change has occured to an entity of the
// collection being watched since the most recent call to Next
// or the Watch call that created the srvRemoteRelationsWatcher.
func (w *srvRemoteRelationsWatcher) Next() (params.RemoteRelationsWatchResult, error) {
	if changes, ok := <-w.watcher.Changes(); ok {
		return params.RemoteRelationsWatchResult{
			Change: &changes,
		}, nil
	}
	err := w.watcher.Err()
	if err == nil {
		err = common.ErrStoppedWatcher
	}
	return params.RemoteRelationsWatchResult{}, err
}

// srvMachineStorageIdsWatcher defines the API wrapping a state.StringsWatcher
// watching machine/storage attachments. This watcher notifies about storage
// entities (volumes/filesystems) being attached to and detached from machines.
//
// TODO(axw) state needs a new watcher, this is a bt of a hack. State watchers
// could do with some deduplication of logic, and I don't want to add to that
// spaghetti right now.
type srvMachineStorageIdsWatcher struct {
	watcherCommon
	watcher state.StringsWatcher
	parser  func([]string) ([]params.MachineStorageId, error)
}

func newVolumeAttachmentsWatcher(context facade.Context) (facade.Facade, error) {
	return newMachineStorageIdsWatcher(
		context,
		storagecommon.ParseVolumeAttachmentIds,
	)
}

func newFilesystemAttachmentsWatcher(context facade.Context) (facade.Facade, error) {
	return newMachineStorageIdsWatcher(
		context,
		storagecommon.ParseFilesystemAttachmentIds,
	)
}

func newMachineStorageIdsWatcher(
	context facade.Context,
	parser func([]string) ([]params.MachineStorageId, error),
) (facade.Facade, error) {
	id := context.ID()
	auth := context.Auth()
	resources := context.Resources()
	if !isAgent(auth) {
		return nil, common.ErrPerm
	}
	watcher, ok := resources.Get(id).(state.StringsWatcher)
	if !ok {
		return nil, common.ErrUnknownWatcher
	}
	return &srvMachineStorageIdsWatcher{
		watcherCommon: newWatcherCommon(context),
		watcher:       watcher,
		parser:        parser,
	}, nil
}

// Next returns when a change has occured to an entity of the
// collection being watched since the most recent call to Next
// or the Watch call that created the srvMachineStorageIdsWatcher.
func (w *srvMachineStorageIdsWatcher) Next() (params.MachineStorageIdsWatchResult, error) {
	if stringChanges, ok := <-w.watcher.Changes(); ok {
		changes, err := w.parser(stringChanges)
		if err != nil {
			return params.MachineStorageIdsWatchResult{}, err
		}
		return params.MachineStorageIdsWatchResult{
			Changes: changes,
		}, nil
	}
	err := w.watcher.Err()
	if err == nil {
		err = common.ErrStoppedWatcher
	}
	return params.MachineStorageIdsWatchResult{}, err
}

// EntitiesWatcher defines an interface based on the StringsWatcher
// but also providing a method for the mapping of the received
// strings to the tags of the according entities.
type EntitiesWatcher interface {
	state.StringsWatcher

	// MapChanges maps the received strings to their according tag strings.
	// The EntityFinder interface representing state or a mock has to be
	// upcasted into the needed sub-interface of state for the real mapping.
	MapChanges(in []string) ([]string, error)
}

// srvEntitiesWatcher defines the API for methods on a state.StringsWatcher.
// Each client has its own current set of watchers, stored in resources.
// srvEntitiesWatcher notifies about changes for all entities of a given kind,
// sending the changes as a list of strings, which could be transformed
// from state entity ids to their corresponding entity tags.
type srvEntitiesWatcher struct {
	watcherCommon
	watcher EntitiesWatcher
}

func newEntitiesWatcher(context facade.Context) (facade.Facade, error) {
	id := context.ID()
	auth := context.Auth()
	resources := context.Resources()

	if !isAgent(auth) {
		return nil, common.ErrPerm
	}
	watcher, ok := resources.Get(id).(EntitiesWatcher)
	if !ok {
		return nil, common.ErrUnknownWatcher
	}
	return &srvEntitiesWatcher{
		watcherCommon: newWatcherCommon(context),
		watcher:       watcher,
	}, nil
}

// Next returns when a change has occured to an entity of the
// collection being watched since the most recent call to Next
// or the Watch call that created the srvEntitiesWatcher.
func (w *srvEntitiesWatcher) Next() (params.EntitiesWatchResult, error) {
	if changes, ok := <-w.watcher.Changes(); ok {
		mapped, err := w.watcher.MapChanges(changes)
		if err != nil {
			return params.EntitiesWatchResult{}, errors.Annotate(err, "cannot map changes")
		}
		return params.EntitiesWatchResult{
			Changes: mapped,
		}, nil
	}
	err := w.watcher.Err()
	if err == nil {
		err = common.ErrStoppedWatcher
	}
	return params.EntitiesWatchResult{}, err
}

var getMigrationBackend = func(st *state.State) migrationBackend {
	return st
}

// migrationBackend defines State functionality required by the
// migration watchers.
type migrationBackend interface {
	LatestMigration() (state.ModelMigration, error)
	APIHostPorts() ([][]network.HostPort, error)
	ControllerConfig() (controller.Config, error)
}

func newMigrationStatusWatcher(context facade.Context) (facade.Facade, error) {
	id := context.ID()
	auth := context.Auth()
	resources := context.Resources()
	st := context.State()

	if !isAgent(auth) {
		return nil, common.ErrPerm
	}
	w, ok := resources.Get(id).(state.NotifyWatcher)
	if !ok {
		return nil, common.ErrUnknownWatcher
	}
	return &srvMigrationStatusWatcher{
		watcherCommon: newWatcherCommon(context),
		watcher:       w,
		st:            getMigrationBackend(st),
	}, nil
}

type srvMigrationStatusWatcher struct {
	watcherCommon
	watcher state.NotifyWatcher
	st      migrationBackend
}

// Next returns when the status for a model migration for the
// associated model changes. The current details for the active
// migration are returned.
func (w *srvMigrationStatusWatcher) Next() (params.MigrationStatus, error) {
	empty := params.MigrationStatus{}

	if _, ok := <-w.watcher.Changes(); !ok {
		err := w.watcher.Err()
		if err == nil {
			err = common.ErrStoppedWatcher
		}
		return empty, err
	}

	mig, err := w.st.LatestMigration()
	if errors.IsNotFound(err) {
		return params.MigrationStatus{
			Phase: migration.NONE.String(),
		}, nil
	} else if err != nil {
		return empty, errors.Annotate(err, "migration lookup")
	}

	phase, err := mig.Phase()
	if err != nil {
		return empty, errors.Annotate(err, "retrieving migration phase")
	}

	sourceAddrs, err := w.getLocalHostPorts()
	if err != nil {
		return empty, errors.Annotate(err, "retrieving source addresses")
	}

	sourceCACert, err := getControllerCACert(w.st)
	if err != nil {
		return empty, errors.Annotate(err, "retrieving source CA cert")
	}

	target, err := mig.TargetInfo()
	if err != nil {
		return empty, errors.Annotate(err, "retrieving target info")
	}

	return params.MigrationStatus{
		MigrationId:    mig.Id(),
		Attempt:        mig.Attempt(),
		Phase:          phase.String(),
		SourceAPIAddrs: sourceAddrs,
		SourceCACert:   sourceCACert,
		TargetAPIAddrs: target.Addrs,
		TargetCACert:   target.CACert,
	}, nil
}

func (w *srvMigrationStatusWatcher) getLocalHostPorts() ([]string, error) {
	hostports, err := w.st.APIHostPorts()
	if err != nil {
		return nil, errors.Trace(err)
	}
	var out []string
	for _, section := range hostports {
		for _, hostport := range section {
			out = append(out, hostport.String())
		}
	}
	return out, nil
}

// This is a shim to avoid the need to use a working State into the
// unit tests. It is tested as part of the client side API tests.
var getControllerCACert = func(st migrationBackend) (string, error) {
	cfg, err := st.ControllerConfig()
	if err != nil {
		return "", errors.Trace(err)
	}

	cacert, ok := cfg.CACert()
	if !ok {
		return "", errors.New("missing CA cert for controller model")
	}
	return cacert, nil
}
