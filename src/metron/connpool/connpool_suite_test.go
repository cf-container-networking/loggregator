//go:generate hel
package connpool_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestConnpool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Connpool Suite")
}
