package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	fakeas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec/fakes"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor/fakes"
	nimbus "github.com/cloudfoundry/bosh-agent/nimbus"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

func init() {
	Describe("Stop", func() {
		var (
			jobSupervisor   *fakejobsuper.FakeJobSupervisor
			platform        *fakeplatform.FakePlatform
			settingsService *fakesettings.FakeSettingsService
			logger          boshlog.Logger
			specService     *fakeas.FakeV1Service
			dualDCSupport   *nimbus.DualDCSupport
			action          StopAction
		)

		BeforeEach(func() {
			jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
			platform = fakeplatform.NewFakePlatform()
			logger = boshlog.NewLogger(boshlog.LevelNone)
			specService = fakeas.NewFakeV1Service()
			settingsService = &fakesettings.FakeSettingsService{}
			dualDCSupport = nimbus.NewDualDCSupport(
				platform.GetRunner(),
				platform.GetFs(),
				platform.GetDirProvider(),
				specService,
				settingsService,
				logger,
			)
			action = NewStop(jobSupervisor, dualDCSupport, platform)
		})

		It("is asynchronous", func() {
			Expect(action.IsAsynchronous()).To(BeTrue())
		})

		It("is not persistent", func() {
			Expect(action.IsPersistent()).To(BeFalse())
		})

		It("returns stopped", func() {
			stopped, err := action.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(stopped).To(Equal("stopped"))
		})

		It("stops job supervisor services", func() {
			_, err := action.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(jobSupervisor.Stopped).To(BeTrue())
		})
	})
}
