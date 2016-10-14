package staticfinder_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestStaticfinder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Staticfinder Suite")
}
