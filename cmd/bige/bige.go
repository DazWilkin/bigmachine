package main

import (
	"context"
	"encoding/gob"
	"flag"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/grailbio/base/log"
	"github.com/grailbio/bigmachine"
	"github.com/grailbio/bigmachine/driver"
	"golang.org/x/sync/errgroup"
)

var (
	nsamples = flag.Int("nsamples", 1e10, "number of samples")
	nmachine = flag.Int("nmachine", 2, "number of machines to provision for the task")
)

func init() {
	gob.Register(Euler{})
}

// Euler represents Euler's constant (2.71828...)
type Euler struct{}

// Sample uses a statistical method to approximate the value of e
// As n tends to infinity, the average number of random numbers (0..1) needed whose sum exceeds 1, tends to e
func (Euler) Sample(ctx context.Context, n uint64, m *uint64) error {
	log.Printf("[Euler:Sample] n=%d", n)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// Run n tests
	for i := uint64(0); i < n; i++ {
		// How many random numbers needed until their total(σ)>1?
		σ := 0.0
		j := uint64(0)
		for j = 0; σ <= 1; j++ {
			σ += r.Float64()
		}
		// Periodically log position and totals
		if i%1e4 == 0 {
			log.Printf("[Euler:Sample] %d/%d: σ=%f j=%d", i, n, σ, j)
		}
		*m += j
	}
	return nil
}

func main() {
	log.AddFlags()
	flag.Parse()
	b := driver.Start()
	defer b.Shutdown()

	go func() {
		log.Printf("http.ListenAndServer: %v", http.ListenAndServe(":3333", nil))
	}()

	ctx := context.Background()
	serviceName := "Euler"
	machines, err := b.Start(ctx, *nmachine, bigmachine.Services{
		serviceName: Euler{},
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Print("waiting for machines to come online")
	for _, m := range machines {
		<-m.Wait(bigmachine.Running)
		log.Printf("machine %s %s", m.Addr, m.State())
	}
	log.Print("all machines are ready")

	perMachine := uint64(*nsamples) / uint64(*nmachine)
	log.Printf("perMachine=%d", perMachine)

	var total uint64
	var cores int
	g, ctx := errgroup.WithContext(ctx)
	log.Printf("len(machines)=%d", len(machines))
	for _, m := range machines {
		m := m
		log.Printf("Maxprocs=%d (%s)", m.Maxprocs, m.Addr)
		for i := 0; i < m.Maxprocs; i++ {
			cores++
			g.Go(func() error {
				var count uint64
				err := m.Call(ctx, fmt.Sprintf("%s.%s", serviceName, "Sample"), perMachine/uint64(m.Maxprocs), &count)
				if err == nil {
					atomic.AddUint64(&total, count)
				}
				return err
			})
		}
	}
	log.Printf("distributing work among %d cores", cores)
	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
	log.Printf("total=%d; nsamples=%d", total, *nsamples)
	var (
		// Approximation for e is total/samples
		e    = big.NewRat(int64(total), int64(*nsamples))
		prec = int(math.Log(float64(*nsamples)) / math.Log(10))
	)
	fmt.Printf("e=%s\n", e.FloatString(prec))
}
