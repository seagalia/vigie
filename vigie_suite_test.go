package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestVigie(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Vigie Suite")
}
