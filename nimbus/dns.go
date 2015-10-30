package nimbus

import (
	"errors"
	"fmt"
	"strings"
	"time"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/system"
)

const dnsUpdateInterval = 60 * time.Second

func (r *DualDCSupport) StartDNSUpdatesIfRequired() (err error) {
	r.logger.Debug(nimbusLogTag, "StartDNSUpdatesIfRequired - begin")
	var enabled bool
	if enabled, err = r.dnsUpdatesEnabled(); err != nil {
		return
	}

	if enabled {
		if r.cancelChan != nil {
			close(r.cancelChan)
			r.cancelChan = nil
		}
		r.cancelChan = make(chan struct{})
		go r.runPeriodicUpdates(r.cancelChan)
	}

	return
}

func (r *DualDCSupport) StopDNSUpdatesIfRequired() (err error) {
	r.logger.Debug(nimbusLogTag, "StopDNSUpdatesIfRequired - begin")
	var enabled bool
	if enabled, err = r.dnsUpdatesEnabled(); err != nil {
		return
	}

	if enabled && r.cancelChan != nil {
		close(r.cancelChan)
		r.cancelChan = nil
	}

	return
}

func (r DualDCSupport) dnsUpdatesEnabled() (enabled bool, err error) {
	spec, err := r.specService.Get()
	if err != nil {
		return false, bosherr.WrapError(err, "Fetching spec")
	}

	return spec.DNSRegisterOnStart != "", nil
}

func (r DualDCSupport) runPeriodicUpdates(cancelChan chan struct{}) {
	tickChan := time.Tick(dnsUpdateInterval)

	r.updateAllDNSServers()

	for {
		select {
		case <-tickChan:
			err := r.updateAllDNSServers()
			if err != nil {
				r.logger.Error(nimbusLogTag, "Error updating DNS: %s, will try again in: %d s", err, dnsUpdateInterval)
			}
		case <-cancelChan:
			return
		}
	}
}

func (r DualDCSupport) updateAllDNSServers() (err error) {

	spec, err := r.specService.Get()
	if err != nil {
		return bosherr.WrapError(err, "Fetching spec")
	}

	dnsSpec := spec.PropertiesSpec.DNSSpec
	if len(dnsSpec.DNSServers) == 0 || dnsSpec.Key == "" || dnsSpec.TTL == 0 {
		r.logger.Error(nimbusLogTag, "dnsSpec.DNSServers or dnsSpec.Key or dnsSpec.TTL empty")
		return errors.New("dnsSpec.DNSServers or dnsSpec.Key or dnsSpec.TTL empty")
	}

	thisHostIP, err := r.thisHostIP()
	if err != nil {
		return
	}

	for _, dnsServer := range dnsSpec.DNSServers {
		err = r.updateDNSServer(spec.DNSRegisterOnStart, thisHostIP, dnsServer, dnsSpec.Key, dnsSpec.TTL)
		if err != nil {
			r.logger.Error(nimbusLogTag, "error updating dns server: %s, name: %s, error: %s", dnsServer, spec.DNSRegisterOnStart, err)
			return
		}
	}

	return
}

func (r DualDCSupport) updateDNSServer(nameToRegister, ip, dnsServer, dnsKey string, ttl int) (err error) {

	var tmpFile system.File
	if tmpFile, err = r.fs.TempFile("dnsRegisterOnStart-"); err != nil {
		return
	}
	defer r.fs.RemoveAll(tmpFile.Name())

	idx := strings.Index(nameToRegister, ".")
	zone := nameToRegister[idx+1:]

	configBody := fmt.Sprintf(
		dnsConfigTemplate,
		dnsServer,
		zone,
		nameToRegister,
		nameToRegister,
		ttl,
		ip,
	)

	if err = r.fs.WriteFileString(tmpFile.Name(), configBody); err != nil {
		return
	}

	_, _, _, err = r.cmdRunner.RunCommand("nsupdate", "-t", "4", "-y", dnsKey, "-v", tmpFile.Name())

	return
}

const dnsConfigTemplate = `
server %s
zone %s
update delete %s A
update add %s %d A %s
send

`
