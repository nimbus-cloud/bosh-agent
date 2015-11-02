package action

import (
	"errors"

	boshappl "github.com/cloudfoundry/bosh-agent/agent/applier"
	boshas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
	nimbus "github.com/cloudfoundry/bosh-agent/nimbus"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

type ApplyAction struct {
	applier         boshappl.Applier
	specService     boshas.V1Service
	settingsService boshsettings.Service
	actionHook      nimbus.ActionHook
}

func NewApply(
	applier boshappl.Applier,
	specService boshas.V1Service,
	settingsService boshsettings.Service,
	dualDCSupport nimbus.DualDCSupport,
	platform boshplatform.Platform,
) (action ApplyAction) {
	action.applier = applier
	action.specService = specService
	action.settingsService = settingsService
	action.actionHook = nimbus.NewActionHook(platform, dualDCSupport)
	return
}

func (a ApplyAction) IsAsynchronous() bool {
	return true
}

func (a ApplyAction) IsPersistent() bool {
	return false
}

func (a ApplyAction) Run(desiredSpec boshas.V1ApplySpec) (string, error) {
	settings := a.settingsService.GetSettings()

	resolvedDesiredSpec, err := a.specService.PopulateDHCPNetworks(desiredSpec, settings)
	if err != nil {
		return "", bosherr.WrapError(err, "Resolving dynamic networks")
	}

	if desiredSpec.ConfigurationHash != "" {
		currentSpec, err := a.specService.Get()
		if err != nil {
			return "", bosherr.WrapError(err, "Getting current spec")
		}

		err = a.applier.Apply(currentSpec, resolvedDesiredSpec)
		if err != nil {
			return "", bosherr.WrapError(err, "Applying")
		}
	}

	err = a.specService.Set(resolvedDesiredSpec)
	if err != nil {
		return "", bosherr.WrapError(err, "Persisting apply spec")
	}

	if err = a.actionHook.OnApplyAction(); err != nil {
		return "", bosherr.WrapError(err, "Calling nimbus OnApplyAction hook")
	}

	return "applied", nil
}

func (a ApplyAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a ApplyAction) Cancel() error {
	return errors.New("not supported")
}
