package dashboard

import (
	"fmt"
	"time"
)

// seriesMonths é o tamanho da série histórica do painel: 12 meses dão o ciclo
// anual inteiro, então a sazonalidade aparece sem confundir com tendência.
const seriesMonths = 12

const isoDate = "2006-01-02"

// BuildWindow monta os três recortes do painel a partir do período pedido.
//
// O período anterior é o que dá sentido ao número do período corrente, e a
// forma de recuar depende do recorte: quando ele é um bloco de meses-calendário
// inteiros (o caso comum — "mês atual", "últimos 3 meses"), o anterior é o
// bloco de mesmo tamanho em meses, para que fevereiro seja comparado com
// janeiro inteiro e não com "os 28 dias anteriores". Fora disso, recua-se pela
// mesma quantidade de dias.
//
// Recorte aberto de um lado ou dos dois ("todo o período") não tem anterior
// comparável: Previous volta vazio e o painel omite a variação em vez de
// inventar uma base.
//
// today ancora a série quando o recorte não tem data final.
func BuildWindow(from, to string, today time.Time) (Window, error) {
	var fromT, toT time.Time
	var err error
	if from != "" {
		if fromT, err = time.Parse(isoDate, from); err != nil {
			return Window{}, fmt.Errorf("date_from inválida: %w", err)
		}
	}
	if to != "" {
		if toT, err = time.Parse(isoDate, to); err != nil {
			return Window{}, fmt.Errorf("date_to inválida: %w", err)
		}
	}
	if from != "" && to != "" && toT.Before(fromT) {
		return Window{}, fmt.Errorf("date_to anterior a date_from")
	}

	w := Window{Current: Period{From: from, To: to}}

	// Âncora da série: o mês final do recorte, ou o mês corrente se o recorte
	// é aberto à direita.
	anchor := today
	if to != "" {
		anchor = toT
	}
	seriesEnd := endOfMonth(anchor)
	seriesStart := startOfMonth(seriesEnd).AddDate(0, -(seriesMonths - 1), 0)
	w.Series = Period{
		From: seriesStart.Format(isoDate),
		To:   seriesEnd.Format(isoDate),
	}

	if from == "" || to == "" {
		return w, nil
	}

	if n, ok := wholeMonths(fromT, toT); ok {
		prevTo := startOfMonth(fromT).AddDate(0, 0, -1)
		prevFrom := startOfMonth(fromT).AddDate(0, -n, 0)
		w.Previous = Period{From: prevFrom.Format(isoDate), To: prevTo.Format(isoDate)}
		return w, nil
	}

	days := int(toT.Sub(fromT).Hours()/24) + 1
	w.Previous = Period{
		From: fromT.AddDate(0, 0, -days).Format(isoDate),
		To:   fromT.AddDate(0, 0, -1).Format(isoDate),
	}
	return w, nil
}

// CurrentMonth devolve o mês corrente no fuso dado, usado como recorte padrão
// quando o cliente não manda período.
func CurrentMonth(tz string, now time.Time) Period {
	if loc, err := time.LoadLocation(tz); err == nil {
		now = now.In(loc)
	}
	return Period{
		From: startOfMonth(now).Format(isoDate),
		To:   endOfMonth(now).Format(isoDate),
	}
}

// wholeMonths reconhece o recorte que cobre meses-calendário inteiros e
// devolve quantos são.
func wholeMonths(from, to time.Time) (int, bool) {
	if from.Day() != 1 || !to.Equal(endOfMonth(to)) {
		return 0, false
	}
	n := (to.Year()-from.Year())*12 + int(to.Month()) - int(from.Month()) + 1
	if n <= 0 {
		return 0, false
	}
	return n, true
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func endOfMonth(t time.Time) time.Time {
	return startOfMonth(t).AddDate(0, 1, -1)
}

// monthsBetween lista os meses (YYYY-MM) cobertos pelo recorte, em ordem
// cronológica e sem furos.
func monthsBetween(from, to string) ([]string, error) {
	f, err := time.Parse(isoDate, from)
	if err != nil {
		return nil, err
	}
	t, err := time.Parse(isoDate, to)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, seriesMonths)
	for cur := startOfMonth(f); !cur.After(startOfMonth(t)); cur = cur.AddDate(0, 1, 0) {
		out = append(out, cur.Format("2006-01"))
	}
	return out, nil
}
