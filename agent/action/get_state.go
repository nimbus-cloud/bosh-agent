package action

import (
	"errors"

	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	boshjobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshntp "github.com/cloudfoundry/bosh-agent/platform/ntp"
	boshvitals "github.com/cloudfoundry/bosh-agent/platform/vitals"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type GetStateAction struct {
	settingsService boshsettings.Service
	specService     boshas.V1Service
	jobSupervisor   boshjobsuper.JobSupervisor
	vitalsService   boshvitals.Service
	ntpService      boshntp.Service
	platform        boshplatform.Platform
}

func NewGetState(
	settingsService boshsettings.Service,
	specService boshas.V1Service,
	jobSupervisor boshjobsuper.JobSupervisor,
	vitalsService boshvitals.Service,
	ntpService boshntp.Service,
	platform boshplatform.Platform,
) (action GetStateAction) {
	action.settingsService = settingsService
	action.specService = specService
	action.jobSupervisor = jobSupervisor
	action.vitalsService = vitalsService
	action.ntpService = ntpService
	action.platform = platform
	return
}

func (a GetStateAction) IsAsynchronous() bool {
	return false
}

func (a GetStateAction) IsPersistent() bool {
	return false
}

type GetStateV1ApplySpec struct {
	boshas.V1ApplySpec

	AgentID      string                 `json:"agent_id"`
	BoshProtocol string                 `json:"bosh_protocol"`
	JobState     string                 `json:"job_state"`
	Vitals       *boshvitals.Vitals     `json:"vitals,omitempty"`
	Processes    []boshjobsuper.Process `json:"processes,omitempty"`
	VM           boshsettings.VM        `json:"vm"`
	Ntp          boshntp.Info           `json:"ntp"`
	Drbd         Drbd                   `json:"drbd"`
}

type Drbd struct {
	ConnectionState string `json:"connection_state"`
	Role            string `json:"role"`
	DiskState       string `json:"disk_state"`
	SyncState       string `json:"sync_state"`
}

func (a GetStateAction) Run(filters ...string) (GetStateV1ApplySpec, error) {
	spec, err := a.specService.Get()
	if err != nil {
		return GetStateV1ApplySpec{}, bosherr.WrapError(err, "Getting current spec")
	}

	var vitals boshvitals.Vitals
	var vitalsReference *boshvitals.Vitals

	if len(filters) > 0 && filters[0] == "full" {
		vitals, err = a.vitalsService.Get()
		if err != nil {
			return GetStateV1ApplySpec{}, bosherr.WrapError(err, "Building full vitals")
		}
		vitalsReference = &vitals
	}

	processes, err := a.jobSupervisor.Processes()
	if err != nil {
		return GetStateV1ApplySpec{}, bosherr.WrapError(err, "Getting processes status")
	}

	settings := a.settingsService.GetSettings()

	jobStatus := ""
	if spec.IsPassiveSide() {
		jobStatus = "passive"
	} else {
		jobStatus = a.jobSupervisor.Status()
	}

	value := GetStateV1ApplySpec{
		spec,
		settings.AgentID,
		"1",
		jobStatus,
		vitalsReference,
		processes,
		settings.VM,
		a.ntpService.GetInfo(),
		a.DrbdInfo(),
	}

	if value.NetworkSpecs == nil {
		value.NetworkSpecs = map[string]boshas.NetworkSpec{}
	}
	if value.ResourcePoolSpecs == nil {
		value.ResourcePoolSpecs = map[string]interface{}{}
	}
	if value.PackageSpecs == nil {
		value.PackageSpecs = map[string]boshas.PackageSpec{}
	}

	return value, nil
}

func (a GetStateAction) DrbdInfo() (drbd Drbd) {
	if a.platform.GetFs().FileExists("/proc/drbd") {
		stdout, _, _, _ := a.platform.GetRunner().RunCommand("sh", "-c", "drbdadm cstate r0 2>&1")
		drbd.ConnectionState = stdout

		stdout, _, _, _ = a.platform.GetRunner().RunCommand("sh", "-c", "drbdadm role r0 2>&1")
		drbd.Role = stdout

		stdout, _, _, _ = a.platform.GetRunner().RunCommand("sh", "-c", "drbdadm dstate r0 2>&1")
		drbd.DiskState = stdout
	} else {
		drbd.ConnectionState = "not running"
	}

	return
}

func (a GetStateAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a GetStateAction) Cancel() error {
	return errors.New("not supported")
}
