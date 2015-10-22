package nimbus

// dns_register_on_start support goes in here
// update DNS every 60s

type DNSRegistrar struct {
}

func NewDNSRegistrar() DNSRegistrar {
	return DNSRegistrar{}
}

func (r DNSRegistrar) StartDNSUpdates() {

}

func (r DNSRegistrar) StopDNSUpdates() {

}

//func updateDns() {
//	// iterate over all dns servers and
//	// nsupdate -t 4 -y #{properties["dns"]["key"]} -v #{tmpdir}/update-#{dns_server}.ns
//}
