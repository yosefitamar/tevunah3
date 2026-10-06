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
		HasGeo:         o.Geo.Located(),
	}
}

// SipomRefs é o que a tradução consulta além do catálogo.
type SipomRefs struct {
	// Natures é o de-para de naturezas (nil = sem de-para: a natureza fica
	// pendente).
	Natures *sipom.NatureMap
	// Areas é a referência de área aprendida das escolhas dos analistas
	// (cidade + bairro → área).
	Areas *sipom.AreaRules
	// GeoRequired: o geocodificador está ligado — ocorrência sem coordenada
	// fica pendente.
	GeoRequired bool
	// Terms são os termos aprendidos das escolhas dos analistas (delegacia,
	// delegado, tipos de arma, drogas, cores…).
	Terms *sipom.TermMap
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
// Sipom.Manual). A área sai da referência aprendida das escolhas dos
// analistas (refs.Areas: cidade + bairro → área) e, sem ela, do catálogo. Sem
// catálogo não faz nada: a ocorrência fica sem tradução e o recálculo resolve
// depois.
func TranslateSipom(cat *sipom.Catalog, refs SipomRefs, o *Occurrence) {
	if cat == nil {
		return
	}
	nm, ar := refs.Natures, refs.Areas
	in := SipomInput(o)
	in.GeoRequired = refs.GeoRequired
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
	// Referência aprendida: um analista já disse qual é a área deste lugar.
	// Vale sobre o catálogo (inclusive onde ele resolve sozinho — se alguém
	// corrigiu, o catálogo estava errado ali), desde que a companhia siga
	// ativa no SIPOM.
	if id, ok := ar.Lookup(in.City, in.Neighborhood); ok && cat.IsActiveCompany(id) {
		r.AreaID, r.AreaOptions = &id, nil
		r.Pending = without(r.Pending, sipom.PendArea, sipom.PendAreaAmbig)
	}
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
	// Procedimento (fase 2): tipo, delegacia e delegado.
	proc := cat.ResolveProcedure(sipom.ProcInput{
		Type: o.ProcedureType, Number: o.ProcedureNumber,
		Station: o.PoliceStation, Delegate: o.Delegate, Terms: refs.Terms,
		ProcedimentoID: stored.ProcedimentoID, DelegaciaID: stored.DelegaciaID, DelegadoID: stored.DelegadoID,
		ManualProc: manual(SipomFieldProcedimento), ManualDelegacia: manual(SipomFieldDelegacia),
		ManualDelegado: manual(SipomFieldDelegado),
	})
	r.Pending = append(r.Pending, proc.Pending...)

	// Materiais (fase 2): armas, drogas e veículos nas listas do SIPOM.
	r.Pending = append(r.Pending, translateMaterials(cat, refs.Terms, o)...)

	o.Sipom = SipomFields{
		NaturezaID: in.NaturezaID,
		Logradouro: r.Logradouro, Numeral: r.Numeral,
		CidadeID: r.CidadeID, BairroID: r.BairroID,
		AreaID: r.AreaID, OPMID: r.OPMID,
		ProcedimentoID: proc.ProcedimentoID, DelegaciaID: proc.DelegaciaID, DelegadoID: proc.DelegadoID,
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

// translateMaterials grava nos itens a tradução para o SIPOM e devolve as
// pendências: uma por tipo de material com algum item não resolvido.
func translateMaterials(cat *sipom.Catalog, terms *sipom.TermMap, o *Occurrence) []string {
	var pending []string
	okAll := true
	for i := range o.Weapons {
		w := &o.Weapons[i]
		r := cat.ResolveWeapon(sipom.WeaponInput{
			Kind: w.Kind, Brand: w.Brand, Model: w.Model, Caliber: w.Caliber,
			TipoID: w.SipomTipoID, MarcaID: w.SipomMarcaID, CalibreID: w.SipomCalibreID, Manual: w.SipomManual,
		}, terms)
		w.SipomTipoID, w.SipomMarcaID, w.SipomCalibreID = r.TipoID, r.MarcaID, r.CalibreID
		okAll = okAll && r.OK
	}
	if !okAll {
		pending = append(pending, sipom.PendArma)
	}
	okAll = true
	for i := range o.Drugs {
		d := &o.Drugs[i]
		r := cat.ResolveDrug(sipom.DrugInput{
			Description: d.Description, Grams: d.Grams,
			DrogaID: d.SipomDrogaID, Quantidade: d.SipomQuantidade, Manual: d.SipomManual,
		}, terms)
		d.SipomDrogaID, d.SipomQuantidade = r.DrogaID, r.Quantidade
		okAll = okAll && r.OK
	}
	if !okAll {
		pending = append(pending, sipom.PendDroga)
	}
	okAll = true
	for i := range o.Vehicles {
		v := &o.Vehicles[i]
		r := cat.ResolveVehicle(sipom.VehicleInput{
			Kind: v.Kind, Brand: v.Brand, Model: v.Model, Color: v.Color, Recovered: o.RecoveredVehicle(),
			TipoCodigo: v.SipomTipoCodigo, CorCodigo: v.SipomCorCodigo, MarcaModeloCodigo: v.SipomMarcaModeloCodigo,
			Situacao: v.SipomSituacao, Manual: v.SipomManual,
		}, terms)
		v.SipomTipoCodigo, v.SipomCorCodigo, v.SipomMarcaModeloCodigo, v.SipomSituacao =
			r.TipoCodigo, r.CorCodigo, r.MarcaModeloCodigo, r.Situacao
		okAll = okAll && r.OK
	}
	if !okAll {
		pending = append(pending, sipom.PendVeiculo)
	}
	return pending
}

// RecoveredVehicle diz se a ocorrência é de recuperação de veículo (pela
// natureza): o veículo vai ao SIPOM como recuperado, não apreendido.
func (o *Occurrence) RecoveredVehicle() bool {
	for _, n := range o.Natures {
		if strings.Contains(strings.ToLower(n), "recupera") {
			return true
		}
	}
	return false
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
	ar, err := sipom.NewAreaRuleRepo(r.db).Load(ctx)
	if err != nil {
		return 0, 0, err
	}
	terms, err := sipom.NewTermRepo(r.db).Load(ctx)
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
		TranslateSipom(cat, SipomRefs{Natures: nm, Areas: ar, GeoRequired: r.GeoRequired, Terms: terms}, o)
		if err := r.SaveSipomAll(ctx, srcs[i].ID, o.Sipom, o.Officers, o.Weapons, o.Drugs, o.Vehicles); err != nil {
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
