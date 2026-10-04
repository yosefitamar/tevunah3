package opsreport

import (
	"context"
	"strings"

	"github.com/belia/tevunah/backend/internal/sipom"
)

// Tradução das ocorrências para os códigos do SIPOM, destino do envio. A
// importação grava a tradução; RecomputeSipom a refaz para o acervo (o
// catálogo ou a regra mudaram), sem tocar no que o analista fixou à mão.

// SipomInput monta a entrada do resolvedor. O local do fato é o da
// ocorrência; sem ele (relatório só com abordagem), vale o da abordagem.
func SipomInput(o *Occurrence) sipom.Input {
	addr, nb, city := o.PlaceAddress, o.PlaceNeighborhood, o.PlaceCity
	if strings.TrimSpace(addr+nb+city) == "" {
		addr, nb, city = o.ApproachAddress, o.ApproachNeighborhood, o.ApproachCity
	}
	return sipom.Input{
		NaturezaID:     o.Sipom.NaturezaID,
		HasTime:        o.StartTime != "",
		Ficha:          o.CIOPS,
		Address:        addr,
		Neighborhood:   nb,
		City:           city,
		CIA:            o.CIA,
		BPM:            o.Unit(),
		Teams:          o.Teams,
		Officers:       len(o.Officers),
		UnlinkedPeople: unlinkedPeople(o.People, o.peopleLinked),
	}
}

// unlinkedPeople conta os envolvidos identificados sem dossiê. linked[i]
// diz se People[i] tem dossiê (nil = nenhum tem, caso da prévia).
func unlinkedPeople(people []Person, linked []bool) int {
	n := 0
	for i, p := range people {
		if sipom.Unidentified(p.Name) {
			continue
		}
		if i >= len(linked) || !linked[i] {
			n++
		}
	}
	return n
}

// SipomPeople monta os envolvidos da ocorrência gravada no formato do
// SIPOM, com a qualificação do dossiê quando vinculado.
func SipomPeople(so *StoredOccurrence) []sipom.Person {
	out := make([]sipom.Person, 0, len(so.People))
	for _, sp := range so.People {
		out = append(out, sipom.TranslatePerson(personInput(so, sp)))
	}
	return out
}

// SipomPeopleParsed é a versão da prévia: só o que o relatório traz.
func SipomPeopleParsed(o *Occurrence) []sipom.Person {
	out := make([]sipom.Person, 0, len(o.People))
	for _, p := range o.People {
		out = append(out, sipom.TranslatePerson(sipom.PersonInput{
			Role: p.Role, Name: p.Name, MotherName: p.MotherName, Note: p.Note, OccurredOn: o.OccurredOn,
		}))
	}
	return out
}

// TranslateSipom preenche o.Sipom e a composição de o.Officers. A natureza
// sai do de-para (nm), salvo se o analista a fixou ("natureza" em
// Sipom.Manual). Sem catálogo não faz nada: a ocorrência fica sem tradução e
// o recálculo resolve depois.
func TranslateSipom(cat *sipom.Catalog, nm *sipom.NatureMap, o *Occurrence) {
	if cat == nil {
		return
	}
	in := SipomInput(o)
	stored := o.Sipom
	manual := func(f string) bool { return hasString(stored.Manual, f) }
	if manual(SipomFieldComposicao) {
		in.Officers = 0 // composição definida pelo analista: não recalcula
	}
	if !manual(SipomFieldNatureza) {
		in.NaturezaID = nil
		if nm != nil {
			ch := nm.Choose(o.Natures)
			in.NaturezaID, in.NaturezaSuggested = ch.NaturezaID, ch.Suggested
		}
	}
	r := cat.Resolve(in)
	// Campos fixados pelo analista valem sobre o automático, e as pendências
	// deles passam a refletir o valor fixado.
	if manual(SipomFieldEndereco) {
		r.Logradouro, r.Numeral = stored.Logradouro, stored.Numeral
		r.Pending = without(r.Pending, sipom.PendLogradouro)
		if strings.TrimSpace(r.Logradouro) == "" {
			r.Pending = append(r.Pending, sipom.PendLogradouro)
		}
	}
	if manual(SipomFieldArea) {
		r.AreaID, r.AreaOptions = stored.AreaID, nil
		r.Pending = without(r.Pending, sipom.PendArea, sipom.PendAreaAmbig)
		if r.AreaID == nil {
			r.Pending = append(r.Pending, sipom.PendArea)
		}
	}
	if manual(SipomFieldOPM) {
		r.OPMID = stored.OPMID
		r.Pending = without(r.Pending, sipom.PendOPM)
		if r.OPMID == nil {
			r.Pending = append(r.Pending, sipom.PendOPM)
		}
	}
	o.Sipom = SipomFields{
		NaturezaID: in.NaturezaID,
		Logradouro: r.Logradouro, Numeral: r.Numeral,
		CidadeID: r.CidadeID, BairroID: r.BairroID,
		AreaID: r.AreaID, OPMID: r.OPMID,
		Pending: r.Pending, Manual: o.Sipom.Manual,
	}
	for i := range o.Officers {
		if i < len(r.Officers) {
			o.Officers[i].SipomEquipe = r.Officers[i].Equipe
			o.Officers[i].SipomPoliciamentoTipoID = r.Officers[i].PoliciamentoTipoID
			o.Officers[i].SipomFuncaoID = r.Officers[i].FuncaoID
		}
	}
}

func without(ss []string, drop ...string) []string {
	out := ss[:0:0]
	for _, s := range ss {
		if !hasString(drop, s) {
			out = append(out, s)
		}
	}
	return out
}

func hasString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// SipomBlocked diz se alguma pendência impede o envio.
func SipomBlocked(pending []string) bool {
	for _, p := range pending {
		if sipom.Blocking(p) {
			return true
		}
	}
	return false
}

// RecomputeSipom refaz a tradução das ocorrências gravadas (todas, ou só
// onlyID) e devolve quantas ficaram prontas para envio e quantas pendentes.
func (r *Repo) RecomputeSipom(ctx context.Context, cat *sipom.Catalog, onlyID string) (ready, pending int, err error) {
	nm, err := sipom.NewMapRepo(r.db).Load(ctx)
	if err != nil {
		return 0, 0, err
	}
	srcs, err := r.SipomSources(ctx, onlyID)
	if err != nil {
		return 0, 0, err
	}
	for i := range srcs {
		o := &srcs[i].Occurrence
		o.Sipom.Manual = srcs[i].Manual
		TranslateSipom(cat, nm, o)
		if err := r.SaveSipom(ctx, srcs[i].ID, o.Sipom, o.Officers); err != nil {
			return ready, pending, err
		}
		if SipomBlocked(o.Sipom.Pending) {
			pending++
		} else {
			ready++
		}
	}
	return ready, pending, nil
}
