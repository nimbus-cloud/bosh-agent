package action

import (
	"errors"

	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	nimbus "github.com/cloudfoundry/bosh-agent/nimbus"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type StartAction struct {
	jobSupervisor boshjobsuper.JobSupervisor
	dualDCSupport nimbus.DualDCSupport
}

func NewStart(jobSupervisor boshjobsuper.JobSupervisor, dualDCSupport nimbus.DualDCSupport) (start StartAction) {
	start = StartAction{
		jobSupervisor: jobSupervisor,
		dualDCSupport: dualDCSupport,
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
	err = a.jobSupervisor.Start()
	if err != nil {
		err = bosherr.WrapError(err, "Starting Monitored Services")
		return
	}

	// TODO: starting a passive job should return error - director should never call start action on the agent
	// TODO: if drbd enabled should disks be mounted ?
	// TODO: should the value be "passive" ???
	a.dualDCSupport.StartDNSUpdatesIfRequired()

	value = "started"
	return
}

func (a StartAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a StartAction) Cancel() error {
	return errors.New("not supported")
}
