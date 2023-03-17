package commands_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/internal/commands"
	"github.com/pivotal-cf/kiln/internal/commands/fakes"
	"golang.org/x/crypto/ssh/agent"
)

var _ = Describe("SSHAgentSetup", func() {
	Context("doesn't have keys", func() {
		It("needs to add keys", func() {
			fakeSSHAgentCreator := &fakes.SSHClientCreator{}
			fakeSSHClient := &fakes.SSHAgent{}
			fakeSSHClient.ListReturns([]*agent.Key{}, nil)
			fakeSSHClient.AddReturns(nil)
			fakeSSHAgentCreator.NewClientReturns(fakeSSHClient)

			subject, err := commands.NewSSHProvider(fakeSSHAgentCreator)
			Expect(err).NotTo(HaveOccurred())
			Expect(subject.NeedsKeys()).To(BeTrue())
			key, _ := subject.GetKeys()
			Expect(key).To(Not(BeNil()))
		})
		Context("the key is encrypted", func() {
			It("adds them successfully", func() {
				tmpfile, err := os.CreateTemp(GinkgoT().TempDir(), GinkgoT().Name())
				Expect(err).NotTo(HaveOccurred())
				_, err = tmpfile.Write(PEMEncryptedKey.PEMBytes)
				Expect(err).NotTo(HaveOccurred())
				_, err = tmpfile.Seek(0, 0)
				Expect(err).NotTo(HaveOccurred())

				fakeSSHAgentCreator := &fakes.SSHClientCreator{}
				fakeSSHClient := &fakes.SSHAgent{}
				fakeKey := commands.Key{KeyPath: tmpfile.Name(), Encrypted: true}
				fakeSSHClient.AddReturns(nil)
				fakeSSHAgentCreator.NewClientReturns(fakeSSHClient)

				subject, err := commands.NewSSHProvider(fakeSSHAgentCreator)
				Expect(err).NotTo(HaveOccurred())
				err = subject.AddKey(fakeKey, PEMEncryptedKey.EncryptionKey)
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeSSHClient.AddCallCount()).To(Equal(1))
			})
		})
		Context("the key isn't encrypted", func() {
			It("fails with a passphrase", func() {
				tmpfile, err := os.CreateTemp(GinkgoT().TempDir(), GinkgoT().Name())
				Expect(err).NotTo(HaveOccurred())
				_, err = tmpfile.Write(PEMUnencryptedKey.PEMBytes)
				Expect(err).NotTo(HaveOccurred())
				_, err = tmpfile.Seek(0, 0)
				Expect(err).NotTo(HaveOccurred())

				fakeSSHAgentCreator := &fakes.SSHClientCreator{}
				fakeSSHClient := &fakes.SSHAgent{}
				fakeKey := commands.Key{KeyPath: tmpfile.Name(), Encrypted: true}
				fakeSSHClient.AddReturns(nil)
				fakeSSHAgentCreator.NewClientReturns(fakeSSHClient)

				subject, err := commands.NewSSHProvider(fakeSSHAgentCreator)
				Expect(err).NotTo(HaveOccurred())
				key, err := subject.GetKeys(fakeKey.KeyPath)
				Expect(key.Encrypted).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())

				err = subject.AddKey(fakeKey, []byte("unnecessary-passphrase"))
				Expect(err).To(HaveOccurred())
				Expect(fakeSSHClient.AddCallCount()).To(Equal(0))
			})

			It("adds without a passphrase", func() {
				tmpfile, err := os.CreateTemp(GinkgoT().TempDir(), GinkgoT().Name())
				Expect(err).NotTo(HaveOccurred())
				_, err = tmpfile.Write(PEMUnencryptedKey.PEMBytes)
				Expect(err).NotTo(HaveOccurred())
				_, err = tmpfile.Seek(0, 0)
				Expect(err).NotTo(HaveOccurred())

				fakeSSHAgentCreator := &fakes.SSHClientCreator{}
				fakeSSHClient := &fakes.SSHAgent{}
				fakeKey := commands.Key{KeyPath: tmpfile.Name(), Encrypted: false}
				fakeSSHClient.AddReturns(nil)
				fakeSSHAgentCreator.NewClientReturns(fakeSSHClient)

				subject, err := commands.NewSSHProvider(fakeSSHAgentCreator)
				Expect(err).NotTo(HaveOccurred())
				key, err := subject.GetKeys(fakeKey.KeyPath)
				Expect(key.Encrypted).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())
				err = subject.AddKey(fakeKey, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeSSHClient.AddCallCount()).To(Equal(1))
			})
		})
	})
	Context("has keys", func() {
		It("doesn't need to add keys", func() {
			fakeSSHAgentCreator := &fakes.SSHClientCreator{}
			fakeSSHClient := &fakes.SSHAgent{}
			agentList := []*agent.Key{{Format: "rsa", Blob: []byte("something")}}
			fakeSSHClient.AddReturns(nil)
			fakeSSHClient.ListReturns(agentList, nil)
			fakeSSHAgentCreator.NewClientReturns(fakeSSHClient)

			subject, err := commands.NewSSHProvider(fakeSSHAgentCreator)
			Expect(err).NotTo(HaveOccurred())
			Expect(subject.NeedsKeys()).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

// from go standard library
// https://github.com/golang/crypto/blob/5d542ad81a58c89581d596f49d0ba5d435481bcf/ssh/testdata/keys.go
var PEMEncryptedKey = struct {
	Name              string
	EncryptionKey     []byte
	IncludesPublicKey bool
	PEMBytes          []byte
}{
	Name:          "rsa-encrypted",
	EncryptionKey: []byte("r54-G0pher_t3st$"),
	PEMBytes: []byte(`-----BEGIN RSA PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: AES-128-CBC,3E1714DE130BC5E81327F36564B05462
MqW88sud4fnWk/Jk3fkjh7ydu51ZkHLN5qlQgA4SkAXORPPMj2XvqZOv1v2LOgUV
dUevUn8PZK7a9zbZg4QShUSzwE5k6wdB7XKPyBgI39mJ79GBd2U4W3h6KT6jIdWA
goQpluxkrzr2/X602IaxLEre97FT9mpKC6zxKCLvyFWVIP9n3OSFS47cTTXyFr+l
7PdRhe60nn6jSBgUNk/Q1lAvEQ9fufdPwDYY93F1wyJ6lOr0F1+mzRrMbH67NyKs
rG8J1Fa7cIIre7ueKIAXTIne7OAWqpU9UDgQatDtZTbvA7ciqGsSFgiwwW13N+Rr
hN8MkODKs9cjtONxSKi05s206A3NDU6STtZ3KuPDjFE1gMJODotOuqSM+cxKfyFq
wxpk/CHYCDdMAVBSwxb/vraOHamylL4uCHpJdBHypzf2HABt+lS8Su23uAmL87DR
yvyCS/lmpuNTndef6qHPRkoW2EV3xqD3ovosGf7kgwGJUk2ZpCLVteqmYehKlZDK
r/Jy+J26ooI2jIg9bjvD1PZq+Mv+2dQ1RlDrPG3PB+rEixw6vBaL9x3jatCd4ej7
XG7lb3qO9xFpLsx89tkEcvpGR+broSpUJ6Mu5LBCVmrvqHjvnDhrZVz1brMiQtU9
iMZbgXqDLXHd6ERWygk7OTU03u+l1gs+KGMfmS0h0ZYw6KGVLgMnsoxqd6cFSKNB
8Ohk9ZTZGCiovlXBUepyu8wKat1k8YlHSfIHoRUJRhhcd7DrmojC+bcbMIZBU22T
Pl2ftVRGtcQY23lYd0NNKfebF7ncjuLWQGy+vZW+7cgfI6wPIbfYfP6g7QAutk6W
KQx0AoX5woZ6cNxtpIrymaVjSMRRBkKQrJKmRp3pC/lul5E5P2cueMs1fj4OHTbJ
lAUv88ywr+R+mRgYQlFW/XQ653f6DT4t6+njfO9oBcPrQDASZel3LjXLpjjYG/N5
+BWnVexuJX9ika8HJiFl55oqaKb+WknfNhk5cPY+x7SDV9ywQeMiDZpr0ffeYAEP
LlwwiWRDYpO+uwXHSFF3+JjWwjhs8m8g99iFb7U93yKgBB12dCEPPa2ZeH9wUHMJ
sreYhNuq6f4iWWSXpzN45inQqtTi8jrJhuNLTT543ErW7DtntBO2rWMhff3aiXbn
Uy3qzZM1nPbuCGuBmP9L2dJ3Z5ifDWB4JmOyWY4swTZGt9AVmUxMIKdZpRONx8vz
I9u9nbVPGZBcou50Pa0qTLbkWsSL94MNXrARBxzhHC9Zs6XNEtwN7mOuii7uMkVc
adrxgknBH1J1N+NX/eTKzUwJuPvDtA+Z5ILWNN9wpZT/7ed8zEnKHPNUexyeT5g3
uw9z9jH7ffGxFYlx87oiVPHGOrCXYZYW5uoZE31SCBkbtNuffNRJRKIFeipmpJ3P
7bpAG+kGHMelQH6b+5K1Qgsv4tpuSyKeTKpPFH9Av5nN4P1ZBm9N80tzbNWqjSJm
S7rYdHnuNEVnUGnRmEUMmVuYZnNBEVN/fP2m2SEwXcP3Uh7TiYlcWw10ygaGmOr7
MvMLGkYgQ4Utwnd98mtqa0jr0hK2TcOSFir3AqVvXN3XJj4cVULkrXe4Im1laWgp
-----END RSA PRIVATE KEY-----
`),
}

var PEMUnencryptedKey = struct {
	Name     string
	PEMBytes []byte
}{
	Name: "rsa-unencrypted",
	PEMBytes: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQC8A6FGHDiWCSREAXCq6yBfNVr0xCVG2CzvktFNRpue+RXrGs/2
a6ySEJQb3IYquw7HlJgu6fg3WIWhOmHCjfpG0PrL4CRwbqQ2LaPPXhJErWYejcD8
Di00cF3677+G10KMZk9RXbmHtuBFZT98wxg8j+ZsBMqGM1+7yrWUvynswQIDAQAB
AoGAJMCk5vqfSRzyXOTXLGIYCuR4Kj6pdsbNSeuuRGfYBeR1F2c/XdFAg7D/8s5R
38p/Ih52/Ty5S8BfJtwtvgVY9ecf/JlU/rl/QzhG8/8KC0NG7KsyXklbQ7gJT8UT
Ojmw5QpMk+rKv17ipDVkQQmPaj+gJXYNAHqImke5mm/K/h0CQQDciPmviQ+DOhOq
2ZBqUfH8oXHgFmp7/6pXw80DpMIxgV3CwkxxIVx6a8lVH9bT/AFySJ6vXq4zTuV9
6QmZcZzDAkEA2j/UXJPIs1fQ8z/6sONOkU/BjtoePFIWJlRxdN35cZjXnBraX5UR
fFHkePv4YwqmXNqrBOvSu+w2WdSDci+IKwJAcsPRc/jWmsrJW1q3Ha0hSf/WG/Bu
X7MPuXaKpP/DkzGoUmb8ks7yqj6XWnYkPNLjCc8izU5vRwIiyWBRf4mxMwJBAILa
NDvRS0rjwt6lJGv7zPZoqDc65VfrK2aNyHx2PgFyzwrEOtuF57bu7pnvEIxpLTeM
z26i6XVMeYXAWZMTloMCQBbpGgEERQpeUknLBqUHhg/wXF6+lFA+vEGnkY+Dwab2
KCXFGd+SQ5GdUcEMe9isUH6DYj/6/yCDoFrXXmpQb+M=
-----END RSA PRIVATE KEY-----
`),
}
