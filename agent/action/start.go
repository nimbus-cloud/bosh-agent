package action

import (
	"errors"

	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	nimbus "github.com/cloudfoundry/bosh-agent/nimbus"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type StartAction struct {
	jobSupervisor boshjobsuper.JobSupervisor
}

func NewStart(jobSupervisor boshjobsuper.JobSupervisor) (start StartAction) {
	start = StartAction{
		jobSupervisor: jobSupervisor,
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

	// TODO: call to start DNS updates for dns_register_on_start property
	// TODO: this should be injected
	dnsRegistrar := nimbus.NewDNSRegistrar()
	dnsRegistrar.StartDNSUpdatesIfRequired()

	value = "started"
	return
}

func (a StartAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a StartAction) Cancel() error {
	return errors.New("not supported")
}
