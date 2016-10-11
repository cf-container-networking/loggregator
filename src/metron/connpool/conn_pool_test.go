package connpool_test

import (
	"metron/connpool"

	"github.com/apoydence/eachers/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ConnPool", func() {
	var (
		mockConnCreator *mockConnCreator
		mockWriteCloser *mockWriteCloser

		connPool *connpool.ConnPool
	)

	BeforeEach(func() {
		mockConnCreator = newMockConnCreator()
		mockWriteCloser = newMockWriteCloser()

		connPool = connpool.New(5, 10, mockConnCreator)
	})

	JustBeforeEach(func() {
		close(mockConnCreator.CreateOutput.Ret0)
	})

	Describe("Write()", func() {
		Context("some of the connections are nil", func() {
			var (
				data []byte
			)

			BeforeEach(func() {
				data = []byte("some-data")
				testhelpers.AlwaysReturn(mockWriteCloser.WriteOutput.N, len(data))
				close(mockWriteCloser.WriteOutput.Err)
				mockConnCreator.CreateOutput.Ret0 <- mockWriteCloser
			})

			It("keeps trying until it finds a good connection", func() {
				for i := 0; i < 10; i++ {
					n, err := connPool.Write(data)
					Expect(err).ToNot(HaveOccurred())
					Expect(n).To(Equal(len(data)))
				}
			})
		})

		Context("all of the connections are nil", func() {
			It("tries for a while then returns an error", func(done Done) {
				errs := make(chan error, 1)
				go func() {
					defer close(done)
					_, err := connPool.Write([]byte("some-data"))
					errs <- err
				}()

				Eventually(errs, 5).Should(HaveLen(1))
			})
		})
	})

	Describe("initialization", func() {
		It("should create the corredt number of connection managers", func() {
			f := func() int {
				return len(mockConnCreator.CreateCalled)
			}

			Eventually(f).Should(BeNumerically(">=", 5))
		})
	})
})
