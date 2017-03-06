// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// The storage command provides a storage management interface,
// for manipulating and inspecting storage entities (volumes,
// filesystems, charm storage).
package storage

import (
	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"

	"github.com/juju/1.25-upgrade/juju2/api/storage"
	"github.com/juju/1.25-upgrade/juju2/apiserver/params"
	"github.com/juju/1.25-upgrade/juju2/cmd/juju/common"
	"github.com/juju/1.25-upgrade/juju2/cmd/modelcmd"
)

// StorageCommandBase is a helper base structure that has a method to get the
// storage managing client.
type StorageCommandBase struct {
	modelcmd.ModelCommandBase
}

// NewStorageAPI returns a storage api for the root api endpoint
// that the environment command returns.
func (c *StorageCommandBase) NewStorageAPI() (*storage.Client, error) {
	root, err := c.NewAPIRoot()
	if err != nil {
		return nil, err
	}
	return storage.NewClient(root), nil
}

// StorageInfo defines the serialization behaviour of the storage information.
type StorageInfo struct {
	Kind        string              `yaml:"kind" json:"kind"`
	Status      EntityStatus        `yaml:"status" json:"status"`
	Persistent  bool                `yaml:"persistent" json:"persistent"`
	Attachments *StorageAttachments `yaml:"attachments" json:"attachments"`
}

// StorageAttachments contains details about all attachments to a storage
// instance.
type StorageAttachments struct {
	// Units is a mapping from unit ID to unit storage attachment details.
	Units map[string]UnitStorageAttachment `yaml:"units" json:"units"`
}

// UnitStorageAttachment contains details of a unit storage attachment.
type UnitStorageAttachment struct {
	// MachineId is the ID of the machine that the unit is assigned to.
	//
	// This is omitempty to cater for legacy results, where the machine
	// information is not available.
	MachineId string `yaml:"machine,omitempty" json:"machine,omitempty"`

	// Location is the location of the storage attachment.
	Location string `yaml:"location,omitempty" json:"location,omitempty"`

	// TODO(axw) per-unit status when we have it in state.
}

// formatStorageDetails takes a set of StorageDetail and
// creates a mapping from storage ID to storage details.
func formatStorageDetails(storages []params.StorageDetails) (map[string]StorageInfo, error) {
	if len(storages) == 0 {
		return nil, nil
	}
	output := make(map[string]StorageInfo)
	for _, details := range storages {
		storageTag, storageInfo, err := createStorageInfo(details)
		if err != nil {
			return nil, errors.Trace(err)
		}
		output[storageTag.Id()] = storageInfo
	}
	return output, nil
}

func createStorageInfo(details params.StorageDetails) (names.StorageTag, StorageInfo, error) {
	storageTag, err := names.ParseStorageTag(details.StorageTag)
	if err != nil {
		return names.StorageTag{}, StorageInfo{}, errors.Trace(err)
	}

	info := StorageInfo{
		Kind: details.Kind.String(),
		Status: EntityStatus{
			details.Status.Status,
			details.Status.Info,
			// TODO(axw) we should support formatting as ISO time
			common.FormatTime(details.Status.Since, false),
		},
		Persistent: details.Persistent,
	}

	if len(details.Attachments) > 0 {
		unitStorageAttachments := make(map[string]UnitStorageAttachment)
		for unitTagString, attachmentDetails := range details.Attachments {
			unitTag, err := names.ParseUnitTag(unitTagString)
			if err != nil {
				return names.StorageTag{}, StorageInfo{}, errors.Trace(err)
			}
			var machineId string
			if attachmentDetails.MachineTag != "" {
				machineTag, err := names.ParseMachineTag(attachmentDetails.MachineTag)
				if err != nil {
					return names.StorageTag{}, StorageInfo{}, errors.Trace(err)
				}
				machineId = machineTag.Id()
			}
			unitStorageAttachments[unitTag.Id()] = UnitStorageAttachment{
				machineId,
				attachmentDetails.Location,
			}
		}
		info.Attachments = &StorageAttachments{unitStorageAttachments}
	}

	return storageTag, info, nil
}
