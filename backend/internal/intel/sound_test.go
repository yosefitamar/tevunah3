package intel

import "testing"

func TestNameMatch(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"Maria de Sousa Lima", "MARIA DE SOUSA LIMA", NameExact},
		{"José da Silva", "JOSE SILVA", NameExact}, // conectivo e acento não contam
		{"Maria de Sousa Lima", "MARIA DE SOUZA LIMA", NameSound},
		{"Thiago Resende Costa", "TIAGO REZENDE COSTA", NameSound},
		{"Wagner Mello", "Vagner Melo", NameSound},
		{"Francisco Rodrigres Alves", "FRANCISCO RODRIGUES ALVES", NameNear},
		{"Pricila Karine Lopes", "PRISCILA KARINE LOPES", NameNear},
		{"Maria Lima", "Maria Lira", NameNone},              // palavra curta: outro nome
		{"Maria de Sousa", "Maria de Sousa Lima", NameNone}, // palavra a menos
		{"Jose Carlos Pereira", "Carlos Jose Pereira", NameNone},
		{"Marcos Antonio", "Marcio Antunes", NameNone},
		{"", "Maria", NameNone},
	} {
		if got := NameMatch(c.a, c.b); got != c.want {
			t.Errorf("NameMatch(%q, %q) = %d, quer %d", c.a, c.b, got, c.want)
		}
	}
}
