package connpool_test

import (
	"fmt"
	"metron/connpool"

	. "github.com/apoydence/eachers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnManager", func() {
	var (
		mockConnCreator *mockConnCreator
		mockWriteCloser *mockWriteCloser

		connManager *connpool.ConnManager
	)

	BeforeEach(func() {
		mockConnCreator = newMockConnCreator()
		mockWriteCloser = newMockWriteCloser()

		connManager = connpool.NewConnManager(5, mockConnCreator)
	})

	JustBeforeEach(func() {
		close(mockWriteCloser.WriteOutput.N)
		close(mockWriteCloser.WriteOutput.Err)
	})

	Describe("Write()", func() {
		Context("Connection is dead", func() {
			Context("Creator has not created new connection yet", func() {
				It("returns a ConnNotAvailableErr", func() {
					_, err := connManager.Write([]byte("some-data"))
					Expect(err).To(MatchError(connpool.ConnNotAvailableErr))
				})
			})

			Context("Creator returns new connection", func() {
				BeforeEach(func() {
					mockConnCreator.CreateOutput.Ret0 <- mockWriteCloser
				})

				It("asks for a new Connection", func(done Done) {
					defer close(done)
					Eventually(mockConnCreator.CreateCalled).Should(HaveLen(1))

					_, err := connManager.Write([]byte("some-data"))
					Expect(err).ToNot(HaveOccurred())
					Expect(mockWriteCloser.WriteInput).To(BeCalled(With([]byte("some-data"))))
				})
			})
		})

		Context("connection is alive", func() {
			BeforeEach(func() {
				mockConnCreator.CreateOutput.Ret0 <- mockWriteCloser
				Eventually(mockConnCreator.CreateCalled).Should(HaveLen(1))
			})

			It("does not ask for a new connection", func() {
				Consistently(mockConnCreator.CreateCalled).Should(HaveLen(1))
			})

			Context("more writes than reset counter", func() {
				BeforeEach(func(done Done) {
					defer close(done)
					for i := 0; i < 5; i++ {
						mockWriteCloser.WriteOutput.N <- 9
						mockWriteCloser.WriteOutput.Err <- nil
						connManager.Write([]byte("some-data"))
					}
				})

				It("closes the connection after the reset count is reached", func() {
					Eventually(mockConnCreator.CreateCalled).Should(HaveLen(2))
				})
			})

			Context("writer returns an error", func() {
				BeforeEach(func() {
					mockWriteCloser.WriteOutput.Err <- fmt.Errorf("some-error")
				})

				It("returns the error", func() {
					_, err := connManager.Write([]byte("some-data"))

					Expect(err).To(MatchError(fmt.Errorf("some-error")))
				})

				It("kills the connection and gets a new one", func() {
					connManager.Write([]byte("some-data"))

					Eventually(mockConnCreator.CreateCalled).Should(HaveLen(2))
				})
			})
		})
	})
})
