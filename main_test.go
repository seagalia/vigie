package main

import (
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CLI", func() {
	// run parses args with the real grammar and runs the command, without exiting the test process
	run := func(args ...string) error {
		c := &CLI{}
		parser, err := newParser(c, kong.Exit(func(int) {}))
		Expect(err).ToNot(HaveOccurred())
		ctx, err := parser.Parse(args)
		Expect(err).ToNot(HaveOccurred())
		return ctx.Run()
	}

	It("accepts a valid configuration", func() {
		path := filepath.Join(GinkgoT().TempDir(), "config.yml")
		Expect(os.WriteFile(path, []byte("services:\n- name: web\n  file: {path: web.txt}\n"), 0600)).To(Succeed())

		Expect(run("check-config", "-c", path)).To(Succeed())
	})

	It("refuses unknown keys in check-config", func() {
		path := filepath.Join(GinkgoT().TempDir(), "config.yml")
		Expect(os.WriteFile(path, []byte("services:\n- name: web\n  file: {path: web.txt}\nlisten: ':80'\n"), 0600)).To(Succeed())

		Expect(run("check-config", "-c", path)).To(MatchError(ContainSubstring("invalid config")))
	})

	It("names vigie in the version line", func() {
		Expect(versionLine()).To(HavePrefix("vigie 0.0.1-dev"))
	})
})
