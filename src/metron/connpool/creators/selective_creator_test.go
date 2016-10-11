package creators_test

import (
	"metron/connpool/creators"

	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SelectiveCreator", func() {
	var (
		mockCreatorA    *mockCreator
		mockCreatorB    *mockCreator
		mockCreatorC    *mockCreator
		mockWriteCloser *mockWriteCloser

		creator *creators.SelectiveCreator
	)

	BeforeEach(func() {
		mockCreatorA = newMockCreator()
		mockCreatorB = newMockCreator()
		mockCreatorC = newMockCreator()
		mockWriteCloser = newMockWriteCloser()

		creator = creators.NewSelectiveCreator(mockCreatorA, mockCreatorB, mockCreatorC)
	})

	Context("preferred creator returns writers", func() {
		BeforeEach(func() {
			testhelpers.AlwaysReturn(mockCreatorA.CreateOutput.Ret0, mockWriteCloser)
			testhelpers.AlwaysReturn(mockCreatorB.CreateOutput.Ret0, mockWriteCloser)
			testhelpers.AlwaysReturn(mockCreatorC.CreateOutput.Ret0, mockWriteCloser)
		})

		It("only uses preferred creator", func() {
			for i := 0; i < 10; i++ {
				Expect(creator.Create()).To(Equal(mockWriteCloser))
			}

			Expect(mockCreatorA.CreateCalled).To(HaveLen(10))
			Expect(mockCreatorB.CreateCalled).To(BeEmpty())
			Expect(mockCreatorC.CreateCalled).To(BeEmpty())
		})
	})

	Context("preferred creator does not return writers", func() {
		BeforeEach(func() {
			close(mockCreatorA.CreateOutput.Ret0)
			close(mockCreatorB.CreateOutput.Ret0)
			testhelpers.AlwaysReturn(mockCreatorC.CreateOutput.Ret0, mockWriteCloser)
		})

		It("tries to use the most preferred creators each time", func() {
			for i := 0; i < 10; i++ {
				Expect(creator.Create()).To(Equal(mockWriteCloser))
			}

			Expect(mockCreatorA.CreateCalled).To(HaveLen(10))
			Expect(mockCreatorB.CreateCalled).To(HaveLen(10))
		})
	})

	Context("none of the creators return a writer", func() {
		BeforeEach(func() {
			close(mockCreatorA.CreateOutput.Ret0)
			close(mockCreatorB.CreateOutput.Ret0)
			close(mockCreatorC.CreateOutput.Ret0)
		})

		It("returns nil", func() {
			Expect(creator.Create()).To(BeNil())
		})
	})
})
