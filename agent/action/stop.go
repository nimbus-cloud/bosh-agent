package action

import (
	"errors"

	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	nimbus "github.com/cloudfoundry/bosh-agent/nimbus"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type StopAction struct {
	jobSupervisor boshjobsuper.JobSupervisor
	dualDCSupport nimbus.DualDCSupport
	platform      boshplatform.Platform
}

func NewStop(jobSupervisor boshjobsuper.JobSupervisor, dualDCSupport nimbus.DualDCSupport, platform boshplatform.Platform) (stop StopAction) {
	stop = StopAction{
		jobSupervisor: jobSupervisor,
		dualDCSupport: dualDCSupport,
		platform:      platform,
	}
	return
}

func (a StopAction) IsAsynchronous() bool {
	return true
}

func (a StopAction) IsPersistent() bool {
	return false
}

func (a StopAction) Run() (value string, err error) {
	err = a.jobSupervisor.Stop()
	if err != nil {
		err = bosherr.WrapError(err, "Stopping Monitored Services")
		return
	}

	// nimbus - start TODO: consider extracting as a method
	disk, found := a.dualDCSupport.PersistentDiskSettings()
	if found {
		if _, err = a.platform.UnmountPersistentDisk(disk); err != nil {
			err = bosherr.WrapErrorf(err, "Unmounting persistent disk %s", disk)
			return
		}
	}
	if err = a.dualDCSupport.StopDNSUpdatesIfRequired(); err != nil {
		err = bosherr.WrapError(err, "Stopping DNS updates if required")
		return
	}
	// nimbus - end

	value = "stopped"
	return
}

func (a StopAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a StopAction) Cancel() error {
	return errors.New("not supported")
}
