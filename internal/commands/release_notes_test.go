package commands

import (
	Ω "github.com/onsi/gomega"
	"github.com/pivotal-cf/jhanda"
	"testing"
)

var _ jhanda.Command = ReleaseNotes{}

func TestReleaseNotes_Usage(t *testing.T) {
	please := Ω.NewWithT(t)

	rn := ReleaseNotes{}

	please.Expect(rn.Usage().Description).NotTo(Ω.BeEmpty())
	please.Expect(rn.Usage().ShortDescription).NotTo(Ω.BeEmpty())
	please.Expect(rn.Usage().Flags).NotTo(Ω.BeNil())
}
