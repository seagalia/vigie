package services

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	eds "github.com/seagalia/vigie/sdk/vigie/eds/v1"
	"google.golang.org/protobuf/testing/protocmp"
)

// countingSource answers its ips, or its error, and counts how often it is asked
type countingSource struct {
	calls atomic.Int32
	ips   []*eds.Endpoint
	err   error
	delay time.Duration
}

func (s *countingSource) Resolve() ([]*eds.Endpoint, error) {
	s.calls.Add(1)
	time.Sleep(s.delay)
	return s.ips, s.err
}

var _ = Describe("Cache", func() {
	var (
		source *countingSource
		now    time.Time
		cache  *Cache
	)

	BeforeEach(func() {
		source = &countingSource{ips: []*eds.Endpoint{{Ip: "10.0.0.1", Port: 80}}}
		now = time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
		cache = NewCache(source, 5*time.Second)
		cache.now = func() time.Time { return now }
	})

	It("answers from the cache until the ttl is over", func() {
		_, _ = cache.Resolve()
		now = now.Add(4 * time.Second)

		ips, err := cache.Resolve()

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(BeComparableTo([]*eds.Endpoint{{Ip: "10.0.0.1", Port: 80}}, protocmp.Transform()))
		Expect(source.calls.Load()).To(Equal(int32(1)))
	})

	It("asks the source again once the ttl is over", func() {
		_, _ = cache.Resolve()
		now = now.Add(5 * time.Second)

		_, _ = cache.Resolve()

		Expect(source.calls.Load()).To(Equal(int32(2)))
	})

	It("never keeps an error", func() {
		source.err = errors.New("registry down")
		_, err := cache.Resolve()
		Expect(err).To(MatchError("registry down"))

		source.err = nil
		ips, err := cache.Resolve()

		Expect(err).ToNot(HaveOccurred())
		Expect(ips).To(HaveLen(1))
		Expect(source.calls.Load()).To(Equal(int32(2)))
	})

	It("asks the source once for requests arriving together", func() {
		source.delay = 50 * time.Millisecond
		wg := sync.WaitGroup{}
		for range 10 {
			wg.Go(func() {
				defer GinkgoRecover()
				ips, err := cache.Resolve()
				Expect(err).ToNot(HaveOccurred())
				Expect(ips).To(HaveLen(1))
			})
		}
		wg.Wait()

		Expect(source.calls.Load()).To(Equal(int32(1)))
	})
})
