package manifest_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/kiln/pkg/planitest"
)

type Job struct {
	InstanceGroup string
	Name          string
}

func TestManifestGeneration(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Manifest Generation Suite")
}

var (
	product       *planitest.ProductService
	productConfig planitest.ProductConfig
	metadataFile  string
	productName   string
	configFile    *os.File
)

var _ = SynchronizedBeforeSuite(func() []byte {
	os.Setenv("RENDERER", "ops-manifest")
	productToBuild := os.Getenv("PRODUCT")
	if productToBuild == "" {
		productToBuild = "srt"
	}

	getwd, _ := os.Getwd()
	fmt.Println(getwd)
	fmt.Printf("Testing product: %s", productToBuild)

	cmd := exec.Command("../../bin/build")
	//cmd := exec.Command("docker", "echo \"hello\"")
	cmd.Env = append(os.Environ(),
		"METADATA_ONLY=true",
		"STUB_RELEASES=true",
		fmt.Sprintf("PRODUCT=%s", productToBuild),
	)

	output, err := cmd.Output()

	//Expect(string(output)).To(Equal("hi"))

	if err != nil {
		Expect(err).To(BeNil())
		//msg := fmt.Sprintf("error running bin/build: %s", err.(*exec.ExitError).Stderr)
		//Expect(err).NotTo(HaveOccurred(), msg)
	}

	return []byte(output)
}, func(metadataContents []byte) {
	metadataFile = string(metadataContents)
	productName = os.Getenv("PRODUCT")
	if productName == "" {
		productName = "srt"
	}

	var err error

	configFile, err = os.Open("config.json")
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	configFile.Close()
})

var _ = BeforeEach(func() {
	var err error
	productConfig = planitest.ProductConfig{
		ConfigFile: configFile,
		TileFile:   strings.NewReader(metadataFile),
	}
	product, err = planitest.NewProductService(productConfig)
	Expect(err).NotTo(HaveOccurred())
})

func fetchProductVersion() (string, error) {
	contents, err := ioutil.ReadFile(filepath.Join("..", "..", "version"))
	if err != nil {
		return "", err
	}

	matches := regexp.MustCompile(`(\d\.\d{1,2}\.\d{1,3})\-build\.\d{1,3}`).FindStringSubmatch(string(contents))

	if len(matches) != 2 {
		return "", fmt.Errorf("could not find version in %q", contents)
	}

	return matches[1], nil
}

type tlsKeypair struct {
	Certificate string
	PrivateKey  string
}

func generateTLSKeypair(hostname string) tlsKeypair {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	Expect(err).NotTo(HaveOccurred())

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		DNSNames:              []string{hostname},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	Expect(err).NotTo(HaveOccurred())

	certContents := bytes.Buffer{}
	err = pem.Encode(&certContents, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	Expect(err).NotTo(HaveOccurred())

	privateKeyContents := bytes.Buffer{}
	err = pem.Encode(&privateKeyContents, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	Expect(err).NotTo(HaveOccurred())

	return tlsKeypair{
		Certificate: certContents.String(),
		PrivateKey:  privateKeyContents.String(),
	}
}
