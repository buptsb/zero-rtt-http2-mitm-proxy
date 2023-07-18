package prefetch

import (
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPrefetch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Prefetch Suite")
}

var mockCtrl *gomock.Controller

var _ = BeforeEach(func() {
	mockCtrl = gomock.NewController(GinkgoT())
})

var _ = BeforeSuite(func() {
})

var _ = AfterEach(func() {
	mockCtrl.Finish()
})
