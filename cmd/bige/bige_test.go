package main

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"testing"
)

func TestSample(t *testing.T) {
	for i := 4; i <= 8; i++ {
		n := uint64(int(math.Pow10(i)))
		t.Run(fmt.Sprintf("Samples=%d", n), func(t *testing.T) {
			e := Euler{}
			m := uint64(0)
			e.Sample(context.TODO(), n, &m)

			got, _ := big.NewRat(int64(m), int64(n)).Float64()
			want := 2.71828
			diff := math.Abs((got - want) / got)
			if diff > .005 {
				t.Errorf("got=%f; want=%f (diff=%f)", got, want, diff)
			}
		})
	}
}
