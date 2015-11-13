package action_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	. "github.com/cloudfoundry/bosh-agent/agent/action"
	fakeas "github.com/cloudfoundry/bosh-agent/agent/applier/applyspec/fakes"
	fakeappl "github.com/cloudfoundry/bosh-agent/agent/applier/fakes"
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
			applier         *fakeappl.FakeApplier
			specService     *fakeas.FakeV1Service
			platform        *fakeplatform.FakePlatform
			settingsService *fakesettings.FakeSettingsService
			logger          boshlog.Logger
			dualDCSupport   *nimbus.DualDCSupport
			action          StartAction
		)

		BeforeEach(func() {
			jobSupervisor = fakejobsuper.NewFakeJobSupervisor()
			applier = fakeappl.NewFakeApplier()
			specService = fakeas.NewFakeV1Service()
			action = NewStart(jobSupervisor, applier, specService, dualDCSupport, platform)
			platform = fakeplatform.NewFakePlatform()
			logger = boshlog.NewLogger(boshlog.LevelNone)
			settingsService = &fakesettings.FakeSettingsService{}
			dualDCSupport = nimbus.NewDualDCSupport(
				platform.GetRunner(),
				platform.GetFs(),
				platform.GetDirProvider(),
				specService,
				settingsService,
				logger,
			)
			action = NewStart(jobSupervisor, applier, specService, dualDCSupport, platform)
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

		It("configures jobs", func() {
			_, err := action.Run()
			Expect(err).ToNot(HaveOccurred())
			Expect(applier.Configured).To(BeTrue())
		})

		It("apply errs if a job fails configuring", func() {
			applier.ConfiguredError = errors.New("fake error")
			_, err := action.Run()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Configuring jobs"))
		})
	})
}
