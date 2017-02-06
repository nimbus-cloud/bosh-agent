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
	actionHook    nimbus.ActionHook
}

func NewStop(jobSupervisor boshjobsuper.JobSupervisor, dualDCSupport *nimbus.DualDCSupport, platform boshplatform.Platform) (stop StopAction) {
	stop = StopAction{
		jobSupervisor: jobSupervisor,
		actionHook:    nimbus.NewActionHook(platform, dualDCSupport),
	}
	return
}

func (a StopAction) IsAsynchronous() bool {
	return true
}

func (a StopAction) IsPersistent() bool {
	return false
}

func (a StopAction) IsLoggable() bool {
	return true
}

func (a StopAction) Run(protocolVersion ProtocolVersion) (value string, err error) {
	if protocolVersion > 2 {
		err = a.jobSupervisor.StopAndWait()
	} else {
		err = a.jobSupervisor.Stop()
	}

	if err != nil {
		err = bosherr.WrapError(err, "Stopping Monitored Services")
		return
	}

	if err = a.actionHook.OnStopAction(); err != nil {
		err = bosherr.WrapError(err, "calling nimbus on stop hook")
		return
	}

	value = "stopped"
	return
}

func (a StopAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a StopAction) Cancel() error {
	return errors.New("not supported")
}
