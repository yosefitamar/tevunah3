package sipom

import "testing"

func TestWholeQuantity(t *testing.T) {
	for in, want := range map[float64]float64{0.03: 1, 0.5: 1, 2.6: 3, 29.3: 29, 23: 23, 116.5: 117} {
		if got := WholeQuantity(in); got == nil || *got != want {
			t.Errorf("WholeQuantity(%v) = %v, quer %v", in, got, want)
		}
	}
	for _, in := range []float64{0, -1} {
		if got := WholeQuantity(in); got != nil {
			t.Errorf("WholeQuantity(%v) = %v, quer nil", in, *got)
		}
	}
}
