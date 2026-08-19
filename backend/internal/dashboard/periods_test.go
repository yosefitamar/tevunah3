package dashboard

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse(isoDate, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestBuildWindow_MesCalendario(t *testing.T) {
	// Mês inteiro recua para o mês inteiro anterior — não para "os 31 dias
	// anteriores", que atravessariam a virada e falseariam a comparação.
	w, err := BuildWindow("2026-03-01", "2026-03-31", day("2026-03-19"))
	if err != nil {
		t.Fatalf("BuildWindow: %v", err)
	}
	if w.Previous.From != "2026-02-01" || w.Previous.To != "2026-02-28" {
		t.Fatalf("anterior = %+v, quero fev/2026 inteiro", w.Previous)
	}
	if w.Series.From != "2025-04-01" || w.Series.To != "2026-03-31" {
		t.Fatalf("série = %+v, quero os 12 meses até mar/2026", w.Series)
	}
}

func TestBuildWindow_TrimestreCalendario(t *testing.T) {
	w, err := BuildWindow("2026-04-01", "2026-06-30", day("2026-06-15"))
	if err != nil {
		t.Fatalf("BuildWindow: %v", err)
	}
	if w.Previous.From != "2026-01-01" || w.Previous.To != "2026-03-31" {
		t.Fatalf("anterior = %+v, quero jan–mar/2026", w.Previous)
	}
}

func TestBuildWindow_RecorteQuebrado(t *testing.T) {
	// Recorte que não fecha meses recua pela mesma quantidade de dias.
	w, err := BuildWindow("2026-03-10", "2026-03-19", day("2026-03-19"))
	if err != nil {
		t.Fatalf("BuildWindow: %v", err)
	}
	if w.Previous.From != "2026-02-28" || w.Previous.To != "2026-03-09" {
		t.Fatalf("anterior = %+v, quero os 10 dias imediatamente anteriores", w.Previous)
	}
}

func TestBuildWindow_SemRecorte(t *testing.T) {
	// "Todo o período" não tem base de comparação: Previous fica vazio e a
	// série se ancora no mês corrente.
	w, err := BuildWindow("", "", day("2026-08-19"))
	if err != nil {
		t.Fatalf("BuildWindow: %v", err)
	}
	if w.Previous.From != "" || w.Previous.To != "" {
		t.Fatalf("anterior = %+v, quero vazio", w.Previous)
	}
	if w.Series.From != "2025-09-01" || w.Series.To != "2026-08-31" {
		t.Fatalf("série = %+v, quero os 12 meses até ago/2026", w.Series)
	}
}

func TestBuildWindow_DataInvalida(t *testing.T) {
	if _, err := BuildWindow("2026-13-01", "2026-13-31", day("2026-08-19")); err == nil {
		t.Fatal("quero erro para mês 13")
	}
	if _, err := BuildWindow("2026-03-31", "2026-03-01", day("2026-08-19")); err == nil {
		t.Fatal("quero erro para intervalo invertido")
	}
}

func TestMonthsBetween_SemFuros(t *testing.T) {
	got, err := monthsBetween("2025-11-01", "2026-02-28")
	if err != nil {
		t.Fatalf("monthsBetween: %v", err)
	}
	want := []string{"2025-11", "2025-12", "2026-01", "2026-02"}
	if len(got) != len(want) {
		t.Fatalf("meses = %v, quero %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("meses = %v, quero %v", got, want)
		}
	}
}
