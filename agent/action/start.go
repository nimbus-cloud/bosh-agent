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
	actionHook    nimbus.ActionHook
}

func NewStart(jobSupervisor boshjobsuper.JobSupervisor, dualDCSupport *nimbus.DualDCSupport, platform boshplatform.Platform) (start StartAction) {
	start = StartAction{
		jobSupervisor: jobSupervisor,
		actionHook:    nimbus.NewActionHook(platform, dualDCSupport),
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

	if err = a.actionHook.OnStartAction(); err != nil {
		err = bosherr.WrapError(err, "calling nimbus on start hook")
		return
	}

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
