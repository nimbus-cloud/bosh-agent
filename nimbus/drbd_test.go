package nimbus

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	//	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	//	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("Nimbus", describeDrbd)

func describeDrbd() {

	BeforeEach(func() {

	})

	Describe("Drbd", func() {
		It("renders config file", func() {

			expectedOutput := `
resource r0 {
  net {
    protocol A;
    shared-secret OIUncfjJsbhInuic1243d;
    verify-alg sha1;
  }
  disk {
    resync-rate 24M;
  }
  handlers {
    before-resync-target "/lib/drbd/snapshot-resync-target-lvm.sh";
    after-resync-target "/lib/drbd/unsnapshot-resync-target-lvm.sh";
  }
  startup {
    wfc-timeout 3;
    degr-wfc-timeout 3;
    outdated-wfc-timeout 2;
  }
  on dff85535-580a-4bfb-bf49-5efbc017b5bb {
    device    drbd1;
    disk      /dev/mapper/vgStoreData-StoreData;
    address   10.76.245.71:7789;
    meta-disk internal;
  }
  on host2 {
    device    drbd1;
    disk      /dev/mapper/vgStoreData-StoreData;
    address   10.92.245.71:7789;
    meta-disk internal;
  }
}
`

			// TODO: fix the test - sort out the fakes
			//			out := drbdConfig("A", "OIUncfjJsbhInuic1243d", "dff85535-580a-4bfb-bf49-5efbc017b5bb", "10.76.245.71", "10.92.245.71")

			Expect("").NotTo(Equal(expectedOutput))
		})

	})

}
