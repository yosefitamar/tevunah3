package opsreport

import (
	"context"
	"regexp"
	"strconv"
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
		People:         len(o.People),
		// Na importação o sexo ainda não existe (o vínculo ao dossiê vem
		// depois): todo identificado conta como sem sexo.
		PeopleWithoutSex:   withoutSex(o, false),
		PeopleSexSuggested: withoutSex(o, true),
		HasGeo:             o.Geo.Located(),
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

// withoutSex conta os identificados sem sexo: os que têm sugestão à espera
// de confirmação (suggested) ou os que não têm nem ela.
func withoutSex(o *Occurrence, suggested bool) int {
	n := 0
	for i, p := range o.People {
		if sipom.Unidentified(p.Name) || (i < len(o.peopleSexed) && o.peopleSexed[i]) {
			continue
		}
		has := i < len(o.peopleSexSug) && o.peopleSexSug[i] != 0
		if has == suggested {
			n++
		}
	}
	return n
}

// suggestSexo refaz a sugestão de sexo dos identificados sem sexo e devolve
// as que mudaram (índices em o.People).
func suggestSexo(o *Occurrence, names *sipom.NameSex, terms *sipom.TermMap) []int {
	identified := 0
	for _, p := range o.People {
		if !sipom.Unidentified(p.Name) {
			identified++
		}
	}
	var changed []int
	for i, p := range o.People {
		if i >= len(o.peopleSexSug) || (i < len(o.peopleSexed) && o.peopleSexed[i]) {
			continue
		}
		s, fonte := sipom.SuggestSexo(sipom.SexoInput{
			Name: p.Name, Role: p.Role, Narrative: o.Narrative, Single: identified == 1,
		}, names, terms)
		if s != o.peopleSexSug[i] || fonte != o.peopleSexSugFont[i] {
			o.peopleSexSug[i], o.peopleSexSugFont[i] = s, fonte
			changed = append(changed, i)
		}
	}
	return changed
}

// SipomPeople monta os envolvidos da ocorrência gravada no formato do
// SIPOM, com a qualificação do dossiê quando vinculado.
func SipomPeople(so *StoredOccurrence) []sipom.Person {
	out := make([]sipom.Person, 0, len(so.People))
	for _, sp := range so.People {
		p := sipom.TranslatePerson(personInput(so, sp))
		p.ID = sp.ID
		out = append(out, p)
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

// LearnedArea acha a referência de área para o lugar: pela grafia do
// relatório (cidade + bairro, ou a cidade inteira onde o catálogo não
// distingue bairro) e, sem ela, pelo nome do bairro no catálogo — como as
// referências inseridas pela tela ficam gravadas.
func LearnedArea(cat *sipom.Catalog, ar *sipom.AreaRules, in sipom.Input, cidadeID, bairroID *int) (int, bool) {
	if id, ok := ar.Find(in.City, in.Neighborhood, cat.CityWideAreas(cidadeID, bairroID)); ok {
		return id, true
	}
	if bairroID != nil {
		return ar.Lookup(in.City, cat.BairroNome(*bairroID))
	}
	return 0, false
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
			// A apreensão que a própria ficha traz corrobora a natureza: a
			// regra condicionada à evidência aplica sem pedir confirmação.
			ev := sipom.Evidence{Drug: len(o.Drugs) > 0}
			for _, w := range o.Weapons {
				ev.Firearm = ev.Firearm || sipom.IsFirearm(w.Kind)
			}
			ch := nm.Choose(o.Natures, ev)
			in.NaturezaID, in.NaturezaSuggested = ch.NaturezaID, ch.Suggested
		}
	}
	r := cat.Resolve(in)
	// Referência aprendida: um analista já disse qual é a área deste lugar.
	// Vale sobre o catálogo (inclusive onde ele resolve sozinho — se alguém
	// corrigiu, o catálogo estava errado ali), desde que a companhia siga
	// ativa no SIPOM.
	if id, ok := LearnedArea(cat, ar, in, r.CidadeID, r.BairroID); ok && cat.IsActiveCompany(id) {
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
	// Bairro escolhido no catálogo pelo analista — vale enquanto for da
	// cidade (trocou a cidade da ficha, volta ao automático).
	if manual(SipomFieldBairro) && stored.BairroID != nil && r.CidadeID != nil &&
		cat.BairroDaCidade(*stored.BairroID, *r.CidadeID) {
		r.BairroID = stored.BairroID
		r.Pending = without(r.Pending, sipom.PendBairroCatalogo)
		if !manual(SipomFieldArea) {
			if opts := cat.Areas(r.CidadeID, r.BairroID); len(opts) == 1 {
				r.AreaID, r.AreaOptions = &opts[0], nil
				r.Pending = without(r.Pending, sipom.PendArea, sipom.PendAreaAmbig)
			}
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

	// O SIPOM recusou o último envio e a ocorrência ainda não foi: fica
	// fora das prontas até o analista reenviar pela ficha.
	if stored.SentAt == nil && stored.Send.RecusadaEm != nil {
		r.Pending = append(r.Pending, sipom.PendEnvioRecusado)
	}

	o.Sipom = SipomFields{
		NaturezaID: in.NaturezaID,
		Logradouro: r.Logradouro, Numeral: r.Numeral,
		CidadeID: r.CidadeID, BairroID: r.BairroID,
		AreaID: r.AreaID, OPMID: r.OPMID,
		ProcedimentoID: proc.ProcedimentoID, DelegaciaID: proc.DelegaciaID, DelegadoID: proc.DelegadoID,
		Pending: r.Pending, Manual: o.Sipom.Manual,
		SentAt: stored.SentAt, Send: stored.Send,
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
		// Ciclomotor sem placa vai como MOTONETA. Qualquer outro sem placa
		// (moto, carro) fica com o tipo do relatório — e pendente.
		if plate(v.Plate) == "" && !v.SipomManual && sipom.Moped(v.Kind, v.Brand, v.Model) {
			moto := sipom.VeiculoTipoMotoneta
			v.SipomTipoCodigo = &moto
		}
		if p := VehiclePending(*v); p != "" {
			pending = appendOnce(pending, p)
		}
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
	names, err := r.NameSex(ctx)
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
		for _, k := range suggestSexo(o, names, terms) {
			if err := r.savePersonSexoSugerido(ctx, o.peopleIDs[k], o.peopleSexSug[k], o.peopleSexSugFont[k]); err != nil {
				return ready, pending, err
			}
		}
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

// VehiclePending diz o que falta para o veículo ir ao SIPOM ("" = nada).
// Com placa, bastam ela e a situação: placa cadastrada lá é só vinculada.
// Com placa, cor e marca/modelo (o SIPOM recusa a placa nova sem eles).
// Sem placa, vão situação, tipo, marca/modelo e cor; e, se não for
// ciclomotor, o analista antes confirma que a placa não pôde ser
// identificada (o relatório costuma omiti-la quando ela existe). Quando o
// SIPOM recusou o veículo (placa nova, sem cadastro), valem tipo, cor,
// marca/modelo e o ano de fabricação, que o relatório não traz.
func VehiclePending(v Vehicle) string {
	complete := VehicleComplete(v)
	switch {
	case v.SipomSituacao == 0:
		return sipom.PendVeiculo
	case v.SipomCadastroExigido:
		if !complete || v.SipomAnoFabricacao == nil {
			return sipom.PendVeiculo
		}
	case plate(v.Plate) == "":
		if !complete {
			return sipom.PendVeiculo
		}
		if !sipom.PlatelessType(*v.SipomTipoCodigo) && !v.SipomSemPlacaConfirmada {
			return sipom.PendVeiculoPlaca
		}
	case v.SipomCorCodigo == nil || v.SipomMarcaModeloCodigo == nil:
		return sipom.PendVeiculo
	}
	return ""
}

// VehicleComplete diz se o veículo tem tipo, cor e marca/modelo do SIPOM.
func VehicleComplete(v Vehicle) bool {
	return v.SipomTipoCodigo != nil && v.SipomCorCodigo != nil && v.SipomMarcaModeloCodigo != nil
}

// RejectedVehicles devolve os veículos a que o 422 do SIPOM se refere: os
// erros vêm por campo ("materiais.2.codigo_tipo"), indexados pelos
// materiais do corpo enviado.
func RejectedVehicles(p *sipom.Payload, erros map[string]string) []string {
	var ids []string
	for campo := range erros {
		m := materialFieldRE.FindStringSubmatch(campo)
		if m == nil {
			continue
		}
		i, err := strconv.Atoi(m[1])
		if err != nil || i >= len(p.Materiais) || p.Materiais[i].TipoID != sipom.MaterialVeiculo {
			continue
		}
		ids = appendOnce(ids, p.Materiais[i].ID)
	}
	return ids
}

var materialFieldRE = regexp.MustCompile(`^materiais\.(\d+)(\.|$)`)
