package experimental

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"code.cloudfoundry.org/cli/integration/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("v3-create-package with .cfignore", func() {
	var (
		orgName   string
		spaceName string
		appName   string
	)

	BeforeEach(func() {
		orgName = helpers.NewOrgName()
		spaceName = helpers.NewSpaceName()
		appName = helpers.PrefixedRandomName("app")
		setupCF(orgName, spaceName)

	})

	AfterEach(func() {
		helpers.QuickDeleteOrg(orgName)
	})

	Context("when .cfignore file exists", func() {
		Context("when the .cfignore file doesn't exclude any files", func() {
			It("pushes all the files except .cfignore", func() {
				helpers.WithHelloWorldApp(func(appDir string) {
					file1 := filepath.Join(appDir, "file1")
					err := ioutil.WriteFile(file1, nil, 0666)
					Expect(err).ToNot(HaveOccurred())

					file2 := filepath.Join(appDir, "file2")
					err = ioutil.WriteFile(file2, nil, 0666)
					Expect(err).ToNot(HaveOccurred())

					cfIgnoreFilePath := filepath.Join(appDir, ".cfignore")
					err = ioutil.WriteFile(cfIgnoreFilePath, nil, 0666)
					Expect(err).ToNot(HaveOccurred())

					Eventually(helpers.CustomCF(helpers.CFEnv{WorkingDirectory: appDir}, "v3-create-app", appName)).Should(Exit(0))
					session := helpers.CustomCF(helpers.CFEnv{WorkingDirectory: appDir}, "v3-create-package", appName)

					Eventually(session).Should(Exit(0))
					helpers.VerifyAppPackageContents(appName, "file1", "file2", "Staticfile", "index.html")
				})
			})
		})

		Context("when the .cfignore file excludes some files", func() {
			Context("when pushing from the current directory", func() {
				It("does not push those files", func() {
					helpers.WithHelloWorldApp(func(appDir string) {
						file1 := filepath.Join(appDir, "file1")
						err := ioutil.WriteFile(file1, nil, 0666)
						Expect(err).ToNot(HaveOccurred())

						file2 := filepath.Join(appDir, "file2")
						err = ioutil.WriteFile(file2, nil, 0666)
						Expect(err).ToNot(HaveOccurred())

						cfIgnoreFilePath := filepath.Join(appDir, ".cfignore")
						err = ioutil.WriteFile(cfIgnoreFilePath, []byte("file*"), 0666)
						Expect(err).ToNot(HaveOccurred())

						Eventually(helpers.CustomCF(helpers.CFEnv{WorkingDirectory: appDir}, "v3-create-app", appName)).Should(Exit(0))
						session := helpers.CustomCF(helpers.CFEnv{WorkingDirectory: appDir}, "v3-create-package", appName)

						Eventually(session).Should(Exit(0))
						helpers.VerifyAppPackageContents(appName, "Staticfile", "index.html")
					})
				})
			})
		})

		Context("when the CF_TRACE file is in the app source directory", func() {
			var previousEnv string

			AfterEach(func() {
				err := os.Setenv("CF_TRACE", previousEnv)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not push it", func() {
				helpers.WithHelloWorldApp(func(appDir string) {
					traceFilePath := filepath.Join(appDir, "i-am-trace.txt")
					err := ioutil.WriteFile(traceFilePath, nil, 0666)
					Expect(err).ToNot(HaveOccurred())

					previousEnv = os.Getenv("CF_TRACE")
					err = os.Setenv("CF_TRACE", traceFilePath)
					Expect(err).ToNot(HaveOccurred())

					Eventually(helpers.CustomCF(helpers.CFEnv{WorkingDirectory: appDir}, "v3-create-app", appName)).Should(Exit(0))
					session := helpers.CustomCF(helpers.CFEnv{WorkingDirectory: appDir}, "v3-create-package", appName)
					Eventually(session).Should(Exit(0))
					helpers.VerifyAppPackageContents(appName, "Staticfile", "index.html")
				})
			})
		})
	})
})
