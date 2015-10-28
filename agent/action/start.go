package action

import (
	"errors"

	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	nimbus "github.com/cloudfoundry/bosh-agent/nimbus"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type StartAction struct {
	jobSupervisor boshjobsuper.JobSupervisor
	dualDCSupport nimbus.DualDCSupport
	platform      boshplatform.Platform
}

func NewStart(jobSupervisor boshjobsuper.JobSupervisor, dualDCSupport nimbus.DualDCSupport, platform boshplatform.Platform) (start StartAction) {
	start = StartAction{
		jobSupervisor: jobSupervisor,
		dualDCSupport: dualDCSupport,
		platform:      platform,
	}
	return
}

func (a StartAction) IsAsynchronous() bool {
	return false
}

func (a StartAction) IsPersistent() bool {
	return false
}

func (a StartAction) Run() (value string, err error) {

	// nimbus start - TODO: extract as a separate method
	// TODO: starting a passive job should return error - director should never call start action on the agent
	disk, found := a.dualDCSupport.PersistentDiskSettings()
	if found {
		if err = a.platform.MountPersistentDisk(disk, a.platform.GetDirProvider().StoreDir()); err != nil {
			err = bosherr.WrapErrorf(err, "Mounting persistent disk %s", disk)
			return
		}
	}
	if err = a.dualDCSupport.StartDNSUpdatesIfRequired(); err != nil {
		err = bosherr.WrapErrorf(err, "Startign DNS updates if required")
		return
	}
	// nimbus start

	err = a.jobSupervisor.Start()
	if err != nil {
		err = bosherr.WrapError(err, "Starting Monitored Services")
		return
	}

	value = "started"
	return
}

func (a StartAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a StartAction) Cancel() error {
	return errors.New("not supported")
}
