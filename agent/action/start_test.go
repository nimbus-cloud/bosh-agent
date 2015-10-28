package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	fakejobsuper "github.com/cloudfoundry/bosh-agent/jobsupervisor/fakes"
	nimbus "github.com/cloudfoundry/bosh-agent/nimbus"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

func init() {
	Describe("Start", func() {
		var (
			jobSupervisor   *fakejobsuper.FakeJobSupervisor
			platform        *fakeplatform.FakePlatform
			settingsService *fakesettings.FakeSettingsService
			logger          boshlog.Logger
			dualDCSupport   nimbus.DualDCSupport
			action          StartAction
		)

		BeforeEach(func() {
			jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
			platform = fakeplatform.NewFakePlatform()
			logger = boshlog.NewLogger(boshlog.LevelNone)
			settingsService = &fakesettings.FakeSettingsService{}
			dualDCSupport = nimbus.NewDualDCSupport(
				platform.GetRunner(),
				platform.GetFs(),
				platform.GetDirProvider(),
				settingsService,
				logger,
			)
			action = NewStart(jobSupervisor, dualDCSupport, platform)
		})

		It("is synchronous", func() {
			Expect(action.IsAsynchronous()).To(BeFalse())
		})

		It("is not persistent", func() {
			Expect(action.IsPersistent()).To(BeFalse())
		})

		It("returns started", func() {
			started, err := action.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(started).To(Equal("started"))
		})

		It("starts monitor services", func() {
			_, err := action.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(jobSupervisor.Started).To(BeTrue())
		})
	})
}
