package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/libbuildpack/cutlass"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CF NodeJS Buildpack", func() {
	var (
		app              *cutlass.App
		serviceBrokerApp *cutlass.App
		serviceNameOne   string
		files            []string
		expected         = strings.ReplaceAll("./node_modules/.bin/slnodejs run  --useinitialcolor true --token token1 --buildsessionid bs1  ./dist/server.js", " ", "")
	)

	BeforeEach(func() {
		serviceNameOne = "sealights-" + cutlass.RandStringRunes(20)
	})

	AfterEach(func() {
		app = DestroyApp(app)

		_ = RunCF("delete-service", "-f", serviceNameOne)

		serviceBrokerApp = DestroyApp(serviceBrokerApp)

		for _, file := range files {
			err := os.Remove(file)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("deploying a NodeJS app with sealights", func() {
		app = cutlass.New(Fixtures("with_sealights"))
		app.Name = "nodejs-sealights-" + cutlass.RandStringRunes(10)
		app.Memory = "256M"
		app.Disk = "512M"

		app.SetEnv("SL_BUILD_SESSION_ID", "bs1")

		By("Pushing an app with a user provided service", func() {
			Expect(RunCF("create-user-provided-service", serviceNameOne, "-p", `{
				"token": "token1"
			}`)).To(Succeed())

			Expect(app.PushNoStart()).To(Succeed())
			Expect(RunCF("bind-service", app.Name, serviceNameOne)).To(Succeed())
			Expect(app.Restart()).To(Succeed())

			tmpDropletFile := filepath.Join(os.TempDir(), fmt.Sprintf("droplet-%s.tgz", cutlass.RandStringRunes(10)))
			files = append(files, tmpDropletFile)
			Expect(app.DownloadDroplet(tmpDropletFile)).To(Succeed())
			file, err := os.Open(tmpDropletFile)
			Expect(err).ToNot(HaveOccurred())
			defer file.Close()
			gz, err := gzip.NewReader(file)
			Expect(err).ToNot(HaveOccurred())
			defer gz.Close()
			tr := tar.NewReader(gz)

			for {
				hdr, err := tr.Next()
				if err == io.EOF {
					break
				}
				if hdr.Name != "./app/package.json" {
					continue
				}
				b, err := ioutil.ReadAll(tr)
				p := map[string]interface{}{}
				json.Unmarshal(b, &p)

				actual := strings.ReplaceAll(p["scripts"].(map[string]interface{})["start"].(string), " ", "")
				Expect(actual).To(Equal(expected))
			}
		})

		By("Unbinding and deleting the CUPS seeker service", func() {
			Expect(RunCF("unbind-service", app.Name, serviceNameOne)).To(Succeed())
			Expect(RunCF("delete-service", "-f", serviceNameOne)).To(Succeed())
		})
	})
})
