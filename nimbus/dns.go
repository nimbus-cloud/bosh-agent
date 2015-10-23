package nimbus

// dns_register_on_start support goes in here
// update DNS every 60s

func (r DualDCSupport) StartDNSUpdatesIfRequired() error {

	return nil
}

func (r DualDCSupport) StopDNSUpdatesIfRequired() error {

	return nil
}

//func updateDns() {
//	// iterate over all dns servers and
//	// nsupdate -t 4 -y #{properties["dns"]["key"]} -v #{tmpdir}/update-#{dns_server}.ns
//}
