package drain_test

import (
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/ginkgo"
	. "github.com/cloudfoundry/bosh-agent/internal/github.com/onsi/gomega"

	fakeaction "github.com/cloudfoundry/bosh-agent/agent/action/fakes"
	. "github.com/cloudfoundry/bosh-agent/agent/script/drain"
	"github.com/cloudfoundry/bosh-agent/agent/script/drain/fakes"
	fakesys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system/fakes"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
)

var _ = Describe("Testing with Ginkgo", func() {
	It("new drain script", func() {
		runner := fakesys.NewFakeCmdRunner()
		fs := fakesys.NewFakeFileSystem()
		params := &fakes.FakeScriptParams{}
		dirProvider := boshdir.NewProvider("/var/vcap")

		scriptProvider := NewConcreteScriptProvider(runner, fs, dirProvider, &fakeaction.FakeClock{})
		script := scriptProvider.NewScript("foo", params)
		Expect(script.Tag()).To(Equal("foo"))
		Expect(script.Path()).To(Equal("/var/vcap/jobs/foo/bin/drain"))
		Expect(script.(ConcreteScript).Params()).To(Equal(params))
	})
})