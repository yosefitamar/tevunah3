package main

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipom"
)

// Fase 2 do envio ao SIPOM: procedimento e materiais (docs/sipom-materiais.md).

// ─── JSON ───

// sipomProcedimentoJSON é o procedimento como vai ao SIPOM, com o que o
// relatório trouxe em texto ao lado, para o analista conferir.
type sipomProcedimentoJSON struct {
	Procedimento *sipomRefJSON `json:"procedimento"`
	Numero       string        `json:"numero"`
	Ano          string        `json:"ano"`
	Delegacia    *sipomRefJSON `json:"delegacia"`
	Delegado     *sipomRefJSON `json:"delegado"`
	// DelegadoCandidates: delegados que casam com o nome abreviado quando
	// mais de um serve.
	DelegadoCandidates []sipomRefJSON `json:"delegado_candidates"`
	// Como o relatório escreveu.
	TipoTexto      string `json:"tipo_texto"`
	NumeroTexto    string `json:"numero_texto"`
	DelegaciaTexto string `json:"delegacia_texto"`
	DelegadoTexto  string `json:"delegado_texto"`
}

type sipomArmaJSON struct {
	Index   int           `json:"index"`
	Tipo    *sipomRefJSON `json:"tipo"`
	Marca   *sipomRefJSON `json:"marca"`
	Calibre *sipomRefJSON `json:"calibre"`
	Manual  bool          `json:"manual"`
	OK      bool          `json:"ok"`
}

type sipomDrogaJSON struct {
	Index      int           `json:"index"`
	Droga      *sipomRefJSON `json:"droga"`
	Unidade    string        `json:"unidade"`
	Quantidade *float64      `json:"quantidade"`
	Manual     bool          `json:"manual"`
	OK         bool          `json:"ok"`
}

type sipomVeiculoJSON struct {
	Index       int           `json:"index"`
	Tipo        *sipomRefJSON `json:"tipo"`
	Cor         *sipomRefJSON `json:"cor"`
	MarcaModelo *sipomRefJSON `json:"marca_modelo"`
	// MarcaModeloCandidates: linhas que casam em parte com marca e modelo.
	MarcaModeloCandidates []sipomRefJSON `json:"marca_modelo_candidates"`
	Situacao              int            `json:"situacao"` // 1 apreendido, 2 recuperado, 0 indefinida
	Manual                bool           `json:"manual"`
	OK                    bool           `json:"ok"`
}

type sipomMateriaisJSON struct {
	Armas    []sipomArmaJSON    `json:"armas"`
	Drogas   []sipomDrogaJSON   `json:"drogas"`
	Veiculos []sipomVeiculoJSON `json:"veiculos"`
}

func ref(id *int, name func(int) string) *sipomRefJSON {
	if id == nil {
		return nil
	}
	return &sipomRefJSON{ID: *id, Nome: name(*id)}
}

// sipomProcedimentoJSON monta o bloco do procedimento. Os candidatos a
// delegado são recalculados aqui (não ficam gravados).
func (a *app) sipomProcedimentoJSON(o *opsreport.Occurrence, terms *sipom.TermMap) *sipomProcedimentoJSON {
	f := o.Sipom
	manual := func(x string) bool { return hasStr(f.Manual, x) }
	proc := a.sipom.ResolveProcedure(sipom.ProcInput{
		Type: o.ProcedureType, Number: o.ProcedureNumber, Station: o.PoliceStation, Delegate: o.Delegate,
		Terms: terms, ProcedimentoID: f.ProcedimentoID, DelegaciaID: f.DelegaciaID, DelegadoID: f.DelegadoID,
		ManualProc: manual(opsreport.SipomFieldProcedimento), ManualDelegacia: manual(opsreport.SipomFieldDelegacia),
		ManualDelegado: manual(opsreport.SipomFieldDelegado),
	})
	out := &sipomProcedimentoJSON{
		Numero: proc.Numero, Ano: proc.Ano, DelegadoCandidates: []sipomRefJSON{},
		TipoTexto: o.ProcedureType, NumeroTexto: o.ProcedureNumber,
		DelegaciaTexto: o.PoliceStation, DelegadoTexto: o.Delegate,
	}
	out.Procedimento = ref(f.ProcedimentoID, func(id int) string { n, _ := a.sipom.Procedimento(id); return n })
	out.Delegacia = ref(f.DelegaciaID, func(id int) string { d, _ := a.sipom.Delegacia(id); return d.Nome })
	out.Delegado = ref(f.DelegadoID, func(id int) string { d, _ := a.sipom.Delegado(id); return d.Nome })
	for _, id := range proc.DelegadoCandidates {
		d, _ := a.sipom.Delegado(id)
		out.DelegadoCandidates = append(out.DelegadoCandidates, sipomRefJSON{ID: id, Nome: d.Nome})
	}
	return out
}

// sipomMateriaisJSON monta o bloco dos materiais a partir da tradução
// gravada nos itens.
func (a *app) sipomMateriaisJSON(o *opsreport.Occurrence, terms *sipom.TermMap) *sipomMateriaisJSON {
	out := &sipomMateriaisJSON{Armas: []sipomArmaJSON{}, Drogas: []sipomDrogaJSON{}, Veiculos: []sipomVeiculoJSON{}}
	for i, w := range o.Weapons {
		out.Armas = append(out.Armas, sipomArmaJSON{
			Index: i, Manual: w.SipomManual,
			Tipo:    ref(w.SipomTipoID, a.sipom.ArmaTipoNome),
			Marca:   ref(w.SipomMarcaID, a.sipom.ArmaMarcaNome),
			Calibre: ref(w.SipomCalibreID, a.sipom.ArmaCalibreNome),
			OK:      w.SipomTipoID != nil && w.SipomMarcaID != nil && w.SipomCalibreID != nil,
		})
	}
	for i, d := range o.Drugs {
		j := sipomDrogaJSON{Index: i, Manual: d.SipomManual, Quantidade: d.SipomQuantidade}
		if d.SipomDrogaID != nil {
			if dr, ok := a.sipom.Droga(*d.SipomDrogaID); ok {
				j.Droga = &sipomRefJSON{ID: dr.ID, Nome: dr.Nome}
				j.Unidade = dr.Unidade
			}
		}
		j.OK = j.Droga != nil && j.Quantidade != nil
		out.Drogas = append(out.Drogas, j)
	}
	for i, v := range o.Vehicles {
		j := sipomVeiculoJSON{
			Index: i, Manual: v.SipomManual, Situacao: v.SipomSituacao, MarcaModeloCandidates: []sipomRefJSON{},
			Tipo: ref(v.SipomTipoCodigo, a.sipom.VeiculoTipoNome),
			Cor:  ref(v.SipomCorCodigo, a.sipom.VeiculoCorNome),
		}
		if v.SipomMarcaModeloCodigo != nil {
			if m, ok := a.sipom.MarcaModelo(*v.SipomMarcaModeloCodigo); ok {
				j.MarcaModelo = &sipomRefJSON{ID: m.Codigo, Nome: m.Descricao}
			}
		}
		if j.MarcaModelo == nil && !v.SipomManual {
			r := a.sipom.ResolveVehicle(sipom.VehicleInput{Kind: v.Kind, Brand: v.Brand, Model: v.Model, Color: v.Color}, terms)
			for _, m := range r.MarcaModeloCandidates {
				j.MarcaModeloCandidates = append(j.MarcaModeloCandidates, sipomRefJSON{ID: m.Codigo, Nome: m.Descricao})
			}
		}
		j.OK = j.Tipo != nil && j.Cor != nil && j.MarcaModelo != nil && j.Situacao != 0
		out.Veiculos = append(out.Veiculos, j)
	}
	return out
}

// ─── GET /api/sipom/catalogo ───
//
// Listas dos selects de procedimento e materiais, para as correções do
// analista. Marca/modelo de veículo não vem aqui (40 mil linhas): tem busca
// própria.

func (a *app) handleSipomCatalogo(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	refs := func(in []sipom.Ref) []sipomRefJSON {
		out := make([]sipomRefJSON, 0, len(in))
		for _, x := range in {
			out = append(out, sipomRefJSON{ID: x.ID, Nome: x.Nome})
		}
		return out
	}
	delegacias := make([]sipomRefJSON, 0)
	for _, d := range a.sipom.Delegacias() {
		delegacias = append(delegacias, sipomRefJSON{ID: d.ID, Nome: d.Nome})
	}
	delegados := make([]sipomRefJSON, 0)
	for _, d := range a.sipom.Delegados() {
		delegados = append(delegados, sipomRefJSON{ID: d.ID, Nome: d.Nome})
	}
	drogas := make([]map[string]any, 0)
	for _, d := range a.sipom.Drogas() {
		drogas = append(drogas, map[string]any{"id": d.ID, "nome": d.Nome, "unidade": d.Unidade})
	}
	httpx.OK(w, map[string]any{
		"procedimentos":  refs(a.sipom.Procedimentos()),
		"delegacias":     delegacias,
		"delegados":      delegados,
		"arma_tipos":     refs(a.sipom.ArmaTipos()),
		"arma_marcas":    refs(a.sipom.ArmaMarcas()),
		"arma_calibres":  refs(a.sipom.ArmaCalibres()),
		"drogas":         drogas,
		"veiculo_tipos":  refs(a.sipom.VeiculoTipos()),
		"veiculo_cores":  refs(a.sipom.VeiculoCores()),
		"celular_marcas": refs(a.sipom.CelularMarcas()),
	})
}

// GET /api/sipom/marcas-modelos?q=
func (a *app) handleSipomMarcasModelos(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	items := []sipomRefJSON{}
	for _, m := range a.sipom.SearchMarcasModelos(r.URL.Query().Get("q"), 30) {
		items = append(items, sipomRefJSON{ID: m.Codigo, Nome: m.Descricao})
	}
	httpx.OK(w, map[string]any{"items": items})
}

// ─── PUT /api/ops-occurrences/{id}/sipom/procedimento ───
//
//	{"procedimento_id": 3, "delegacia_id": 72, "delegado_id": 401}  (cada um opcional)
//	{"reset": true}                                                 volta ao automático
//
// A delegacia e o delegado escolhidos viram termos aprendidos para o texto
// que o relatório trouxe ("DMC" → 201; "Ítalo Renno Alves" → 361): as
// próximas ocorrências com o mesmo texto já entram resolvidas.

func (a *app) handleSipomSetProcedimento(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	id := r.PathValue("id")
	var req struct {
		ProcedimentoID *int `json:"procedimento_id"`
		DelegaciaID    *int `json:"delegacia_id"`
		DelegadoID     *int `json:"delegado_id"`
		Reset          bool `json:"reset"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if req.ProcedimentoID != nil {
		if _, ok := a.sipom.Procedimento(*req.ProcedimentoID); !ok {
			httpx.Error(w, http.StatusBadRequest, "tipo de procedimento inexistente")
			return
		}
	}
	if req.DelegaciaID != nil {
		if _, ok := a.sipom.Delegacia(*req.DelegaciaID); !ok {
			httpx.Error(w, http.StatusBadRequest, "delegacia inexistente")
			return
		}
	}
	if req.DelegadoID != nil {
		if _, ok := a.sipom.Delegado(*req.DelegadoID); !ok {
			httpx.Error(w, http.StatusBadRequest, "delegado inexistente")
			return
		}
	}
	if !req.Reset && req.ProcedimentoID == nil && req.DelegaciaID == nil && req.DelegadoID == nil {
		httpx.Error(w, http.StatusBadRequest, "informe procedimento_id, delegacia_id ou delegado_id — ou reset")
		return
	}
	before, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	if err := a.opsReports.SetSipomProcedimento(r.Context(), id, req.ProcedimentoID, req.DelegaciaID, req.DelegadoID, req.Reset); err != nil {
		log.Printf("sipom set procedimento: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar")
		return
	}
	learned := map[string]any{}
	if !req.Reset {
		me := middleware.UserFrom(r.Context())
		if req.DelegaciaID != nil {
			a.learnTerm(r, learned, sipom.TermDelegacia, before.PoliceStation, *req.DelegaciaID, me.ID)
		}
		if req.DelegadoID != nil {
			a.learnTerm(r, learned, sipom.TermDelegado, before.Delegate, *req.DelegadoID, me.ID)
		}
	}
	a.finishSipomChange(w, r, id, "opsreport.sipom.procedimento", before, learned)
}

// ─── PUT /api/ops-occurrences/{id}/sipom/materiais/{kind}/{pos} ───
//
//	armas:    {"tipo_id": 2, "marca_id": 27, "calibre_id": 10}
//	drogas:   {"droga_id": 9, "quantidade": 582}
//	veiculos: {"tipo_codigo": 4, "cor_codigo": 11, "marca_modelo_codigo": 2892, "situacao": 1}
//	qualquer: {"reset": true}
//
// Os códigos escolhidos viram termos aprendidos para o texto do relatório
// ("REVOLVER" → tipo 2, "PRETO" → cor 11, "HONDA/CG 160" → 2892).

func (a *app) handleSipomSetMaterial(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	id, kind := r.PathValue("id"), r.PathValue("kind")
	pos, err := strconv.Atoi(r.PathValue("pos"))
	if err != nil || pos < 0 {
		httpx.Error(w, http.StatusBadRequest, "posição inválida")
		return
	}
	var req struct {
		TipoID            *int     `json:"tipo_id"`
		MarcaID           *int     `json:"marca_id"`
		CalibreID         *int     `json:"calibre_id"`
		DrogaID           *int     `json:"droga_id"`
		Quantidade        *float64 `json:"quantidade"`
		TipoCodigo        *int     `json:"tipo_codigo"`
		CorCodigo         *int     `json:"cor_codigo"`
		MarcaModeloCodigo *int     `json:"marca_modelo_codigo"`
		Situacao          int      `json:"situacao"`
		Reset             bool     `json:"reset"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	before, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	me := middleware.UserFrom(r.Context())
	learned := map[string]any{}
	switch kind {
	case "armas":
		if pos >= len(before.Weapons) {
			httpx.Error(w, http.StatusNotFound, "arma não encontrada")
			return
		}
		if !req.Reset && (req.TipoID == nil || req.MarcaID == nil || req.CalibreID == nil) {
			httpx.Error(w, http.StatusBadRequest, "informe tipo_id, marca_id e calibre_id")
			return
		}
		if !req.Reset && (a.sipom.ArmaTipoNome(*req.TipoID) == "" || a.sipom.ArmaMarcaNome(*req.MarcaID) == "" ||
			a.sipom.ArmaCalibreNome(*req.CalibreID) == "") {
			httpx.Error(w, http.StatusBadRequest, "tipo, marca ou calibre inexistente")
			return
		}
		err = a.opsReports.SetWeaponSipom(r.Context(), id, pos, req.TipoID, req.MarcaID, req.CalibreID, req.Reset)
		if err == nil && !req.Reset {
			wpn := before.Weapons[pos]
			a.learnTerm(r, learned, sipom.TermArmaTipo, wpn.Kind, *req.TipoID, me.ID)
			a.learnTerm(r, learned, sipom.TermArmaMarca, wpn.Brand, *req.MarcaID, me.ID)
			a.learnTerm(r, learned, sipom.TermArmaCalibre, wpn.Caliber, *req.CalibreID, me.ID)
		}
	case "drogas":
		if pos >= len(before.Drugs) {
			httpx.Error(w, http.StatusNotFound, "droga não encontrada")
			return
		}
		if !req.Reset && (req.DrogaID == nil || req.Quantidade == nil || *req.Quantidade <= 0) {
			httpx.Error(w, http.StatusBadRequest, "informe droga_id e quantidade")
			return
		}
		if !req.Reset {
			if _, ok := a.sipom.Droga(*req.DrogaID); !ok {
				httpx.Error(w, http.StatusBadRequest, "droga inexistente")
				return
			}
		}
		err = a.opsReports.SetDrugSipom(r.Context(), id, pos, req.DrogaID, req.Quantidade, req.Reset)
		if err == nil && !req.Reset {
			a.learnTerm(r, learned, sipom.TermDroga, before.Drugs[pos].Description, *req.DrogaID, me.ID)
		}
	case "veiculos":
		if pos >= len(before.Vehicles) {
			httpx.Error(w, http.StatusNotFound, "veículo não encontrado")
			return
		}
		if !req.Reset && (req.TipoCodigo == nil || req.CorCodigo == nil || req.MarcaModeloCodigo == nil ||
			(req.Situacao != sipom.VeiculoApreendido && req.Situacao != sipom.VeiculoRecuperado)) {
			httpx.Error(w, http.StatusBadRequest, "informe tipo_codigo, cor_codigo, marca_modelo_codigo e situacao (1 ou 2)")
			return
		}
		if !req.Reset {
			_, okMM := a.sipom.MarcaModelo(*req.MarcaModeloCodigo)
			if a.sipom.VeiculoTipoNome(*req.TipoCodigo) == "" || a.sipom.VeiculoCorNome(*req.CorCodigo) == "" || !okMM {
				httpx.Error(w, http.StatusBadRequest, "tipo, cor ou marca/modelo inexistente")
				return
			}
		}
		err = a.opsReports.SetVehicleSipom(r.Context(), id, pos, req.TipoCodigo, req.CorCodigo, req.MarcaModeloCodigo, req.Situacao, req.Reset)
		if err == nil && !req.Reset {
			v := before.Vehicles[pos]
			a.learnTerm(r, learned, sipom.TermVeiculoTipo, v.Kind, *req.TipoCodigo, me.ID)
			a.learnTerm(r, learned, sipom.TermVeiculoCor, v.Color, *req.CorCodigo, me.ID)
			a.learnTerm(r, learned, sipom.TermVeiculoMarcaModel, strings.TrimSpace(v.Brand)+"/"+strings.TrimSpace(v.Model), *req.MarcaModeloCodigo, me.ID)
		}
	default:
		httpx.Error(w, http.StatusNotFound, "tipo de material desconhecido")
		return
	}
	if err != nil {
		if errors.Is(err, opsreport.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "item não encontrado")
			return
		}
		log.Printf("sipom set material %s: %v", kind, err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar")
		return
	}
	a.finishSipomChange(w, r, id, "opsreport.sipom.materiais", before, learned)
}

// learnTerm grava o termo aprendido (campo + texto do relatório → id) e
// anota em learned o que mudou. Texto vazio não ensina nada.
func (a *app) learnTerm(r *http.Request, learned map[string]any, field, term string, id int, actor string) {
	if strings.TrimSpace(term) == "" {
		return
	}
	changed, err := a.sipomTerms.Learn(r.Context(), field, term, id, actor)
	if err != nil {
		if !errors.Is(err, sipom.ErrEmptyTerm) {
			log.Printf("sipom term %s: %v", field, err)
		}
		return
	}
	if changed {
		learned[field] = map[string]any{"term": strings.ToUpper(strings.TrimSpace(term)), "id": id}
	}
}

// finishSipomChange recalcula (o acervo inteiro quando algum termo foi
// aprendido — ele alcança outras ocorrências; só esta, senão), audita e
// responde com o bloco SIPOM atualizado.
func (a *app) finishSipomChange(w http.ResponseWriter, r *http.Request, id, action string,
	before *opsreport.StoredOccurrence, learned map[string]any) {
	if len(learned) > 0 {
		if _, _, err := a.opsReports.RecomputeSipom(r.Context(), a.sipom, ""); err != nil {
			log.Printf("sipom recompute after terms: %v", err)
		}
	} else {
		a.sipomRecomputeOne(r.Context(), id)
	}
	after, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler a ocorrência")
		return
	}
	afterSnap := sipomAuditSnapshot(after)
	if len(learned) > 0 {
		afterSnap["terms_learned"] = learned
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       action,
		ResourceType: audit.Ptr("ops_occurrence"),
		ResourceID:   &id,
		Before:       sipomAuditSnapshot(before),
		After:        afterSnap,
	})
	occ := a.toOpsOccurrenceJSON(&after.Occurrence)
	occ.Sipom = a.sipomJSON(&after.Occurrence, opsreport.SipomPeople(after))
	a.markAreaLearned(r.Context(), &after.Occurrence, occ.Sipom)
	httpx.OK(w, map[string]any{"sipom": occ.Sipom, "officers": occ.Officers, "terms_learned": learned})
}
