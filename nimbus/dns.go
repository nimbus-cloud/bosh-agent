package nimbus

import (
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
)

// dns_register_on_start support goes in here
// update DNS every 60s

func (r DualDCSupport) StartDNSUpdatesIfRequired() (err error) {

	spec, err := r.specService.Get()
	if err != nil {
		return bosherr.WrapError(err, "Fetching spec")
	}

	if spec.DNSRegisterOnStart != "" {
		r.runPeriodicUpdates(spec.DNSRegisterOnStart)
	}

	return
}

func (r DualDCSupport) StopDNSUpdatesIfRequired() (err error) {

	return nil
}

func (r DualDCSupport) runPeriodicUpdates(nameToRegister string) {
	// this method starts something on a new goroutine
}

func (r DualDCSupport) updateAllDNSServers(nameToRegister string) (err error) {
	thisHostIP, err := r.thisHostIP()
	if err != nil {
		r.logger.Error(nimbusLogTag, "error reading this host IP: %s", err)
		return
	}

	// TODO: this is wrong - need to take dnsservers from properties
	for _, network := range r.settingsService.GetSettings().Networks {
		for _, dnsServer := range network.DNS {
			r.updateDNSServer(dnsServer, nameToRegister, thisHostIP)
		}
	}

	return
}

func (r DualDCSupport) updateDNSServer(dnsServer, nameToRegister, ip string) {

	// zone - derived from nameToRegister

	// TODO: need to get hold of properties from apply spec - possibly extend v1_apply_spec with these props
	//	return unless properties["dns"]
	//	return unless properties["dns"]["ttl"]
	//	return unless properties["dns"]["key"]
	//	return unless properties["dns"]["dnsservers"]

	//	r.fs.TempFile()

}

//func updateDns() {
//	// iterate over all dns servers and
//	// nsupdate -t 4 -y #{properties["dns"]["key"]} -v #{tmpdir}/update-#{dns_server}.ns
//}
