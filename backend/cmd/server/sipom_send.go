package main

// Envio pela API do SIPOM (internal/sipom/client.go) e as correções que o
// contrato dela pede além das da fase 2: sexo do envolvido, ano do veículo
// e bairro do catálogo.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipom"
)

// sipomEnvioJSON é o resultado do último envio pela API.
type sipomEnvioJSON struct {
	Protocolo     string        `json:"protocolo"`
	Identificador string        `json:"identificador"`
	Situacao      string        `json:"situacao"`
	Avisos        []sipom.Aviso `json:"avisos"`
	Erro          string        `json:"erro"`
	TentativaEm   *time.Time    `json:"tentativa_em"`
	ExcluidaEm    *time.Time    `json:"excluida_em"`
}

// sipomEnvio devolve nil quando nunca houve tentativa pela API.
func sipomEnvio(s opsreport.SipomSend) *sipomEnvioJSON {
	if s.TentativaEm == nil {
		return nil
	}
	avisos := s.Avisos
	if avisos == nil {
		avisos = []sipom.Aviso{}
	}
	return &sipomEnvioJSON{
		Protocolo: s.Protocolo, Identificador: s.Identificador, Situacao: s.Situacao,
		Avisos: avisos, Erro: s.Erro, TentativaEm: s.TentativaEm, ExcluidaEm: s.ExcluidaEm,
	}
}

// Tempo máximo de um envio: corpo de até 20 MB e as tentativas extras do
// cliente (429/500). Fica abaixo dos 100 s do tunnel da Cloudflare.
const sipomSendTimeout = 90 * time.Second

// POST /api/ops-occurrences/{id}/sipom/enviar
//
// Monta o payload com as fotos e envia ao SIPOM. Reenviar é seguro: o SIPOM
// identifica a ocorrência por origem.id ("sem_alteracao" quando nada mudou,
// "atualizado" quando a ficha mudou).
func (a *app) handleSipomEnviar(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	if a.sipomClient == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "envio ao SIPOM não configurado (sem credenciais da API)")
		return
	}
	id := r.PathValue("id")
	// Recalcula antes: o dossiê vinculado pode ter mudado (sexo, CPF…).
	a.sipomRecomputeOne(r.Context(), id)
	so, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	if so.Sipom.Send.ExcluidaEm != nil {
		httpx.Error(w, http.StatusConflict, "a ocorrência foi excluída no SIPOM — o SIPOM não a recria")
		return
	}
	me := middleware.UserFrom(r.Context())
	payload, err := opsreport.BuildSipomPayload(a.sipom, so, a.sipomPayloadSources(r, me.ClearanceLevel, true))
	var nr *opsreport.NotReadyError
	switch {
	case errors.As(err, &nr):
		pend := make([]sipomPendingJSON, 0, len(nr.Pending))
		for _, p := range nr.Pending {
			pend = append(pend, sipomPendingJSON{Code: p, Label: sipom.PendingLabel[p], Blocking: true})
		}
		httpx.WriteJSON(w, http.StatusConflict, httpx.Envelope{
			Message: "a ocorrência ainda tem pendência que impede o envio", Errors: pend,
		})
		return
	case err != nil:
		httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	body, err := marshalSipom(payload)
	if err != nil {
		log.Printf("sipom marshal: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao montar o envio")
		return
	}
	sum := sha256.Sum256(body)
	bodySHA := hex.EncodeToString(sum[:])

	// O envio pode passar do WriteTimeout padrão do servidor.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(sipomSendTimeout + 10*time.Second))
	ctx, cancel := context.WithTimeout(r.Context(), sipomSendTimeout)
	defer cancel()
	res, sendErr := a.sipomClient.Receber(ctx, body)

	aid, sid, ip, ua := a.actorInfo(r)
	meta := map[string]any{"bytes": len(body), "body_sha256": bodySHA, "versao": sipom.PayloadVersion}
	if sendErr != nil {
		var ae *sipom.APIError
		excluded := errors.As(sendErr, &ae) && ae.Status == http.StatusConflict
		if err := a.opsReports.SaveSipomSendError(r.Context(), id, sendErr.Error(), excluded); err != nil {
			log.Printf("sipom send error save: %v", err)
		}
		meta["erro"] = sendErr.Error()
		_ = a.audit.Log(r.Context(), audit.Entry{
			ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
			Action: "opsreport.sipom.send.fail", ResourceType: audit.Ptr("ops_occurrence"), ResourceID: &id,
			After: meta,
		})
		var errs any
		if ae != nil && len(ae.Erros) > 0 {
			errs = ae.Erros
		}
		httpx.WriteJSON(w, http.StatusBadGateway, httpx.Envelope{Message: sipomSendMessage(sendErr), Errors: errs})
		return
	}
	if err := a.opsReports.SaveSipomSend(r.Context(), id, me.ID, res, bodySHA); err != nil {
		// O SIPOM já gravou: o próximo envio do mesmo origem.id volta
		// "sem_alteracao" e grava aqui.
		log.Printf("sipom send save: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "o SIPOM recebeu, mas houve erro ao gravar o resultado — envie de novo")
		return
	}
	meta["protocolo"], meta["situacao"], meta["identificador"], meta["avisos"] =
		res.Protocolo, res.Situacao, res.Identificador, res.Avisos
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action: "opsreport.sipom.send", ResourceType: audit.Ptr("ops_occurrence"), ResourceID: &id,
		Before: map[string]any{"sipom_sent_at": so.Sipom.SentAt, "sipom_identificador": so.Sipom.Send.Identificador},
		After:  meta,
	})
	after, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler a ocorrência")
		return
	}
	httpx.OK(w, map[string]any{"sipom": a.sipomJSON(&after.Occurrence, opsreport.SipomPeople(after))})
}

// marshalSipom serializa sem escapar <, > e & (texto do histórico vai como
// está). É este corpo, byte a byte, que vai assinado.
func marshalSipom(p *sipom.Payload) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// sipomSendMessage traduz a falha do envio para o analista.
func sipomSendMessage(err error) string {
	var ae *sipom.APIError
	if !errors.As(err, &ae) {
		return "erro no envio ao SIPOM: " + err.Error()
	}
	switch {
	case ae.Status == 0:
		return "o SIPOM não respondeu — tente de novo"
	case ae.Codigo == sipom.ErrValidacao:
		return "o SIPOM recusou campos da ocorrência (nada foi gravado): " + ae.Error()
	case ae.Codigo == sipom.ErrSemPermissao:
		return "o SIPOM negou o envio (cliente desativado, IP não liberado ou OPM fora do escopo)"
	case ae.Codigo == sipom.ErrAssinaturaInvalida:
		return "o SIPOM recusou a assinatura (chave, segredo ou relógio do servidor)"
	case ae.Codigo == sipom.ErrConflito:
		return "a ocorrência foi excluída no SIPOM — o SIPOM não a recria"
	case ae.Codigo == sipom.ErrLimiteExcedido:
		return "limite de envios por minuto do SIPOM — aguarde um pouco e envie de novo"
	case ae.Status >= 500:
		return "falha no SIPOM (nada foi gravado) — tente de novo"
	}
	return ae.Error()
}

// GET /api/admin/sipom/ping — testa as credenciais e a assinatura sem
// gravar nada no SIPOM.
func (a *app) handleSipomPing(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "sipom.mapping.manage") {
		return
	}
	if a.sipomClient == nil {
		httpx.OK(w, map[string]any{"configurado": false})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	start := time.Now()
	cliente, err := a.sipomClient.Ping(ctx)
	out := map[string]any{"configurado": true, "url": a.sipomClient.BaseURL, "ms": time.Since(start).Milliseconds()}
	if err != nil {
		out["ok"], out["erro"] = false, sipomSendMessage(err)
	} else {
		out["ok"], out["cliente"] = true, cliente
	}
	httpx.OK(w, out)
}

// GET /api/sipom/bairros?cidade_id=919 — bairros do catálogo da cidade,
// para quando o do relatório não casa com nenhum.
func (a *app) handleSipomBairros(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	cid, err := strconv.Atoi(r.URL.Query().Get("cidade_id"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "cidade_id obrigatório")
		return
	}
	items := []sipomRefJSON{}
	for _, b := range a.sipom.Bairros(cid) {
		items = append(items, sipomRefJSON{ID: b.ID, Nome: b.Nome})
	}
	httpx.OK(w, map[string]any{"items": items})
}

// PUT /api/ops-occurrences/{id}/sipom/pessoas/{pid}/sexo {"sexo": 1 | 2 | 0}
//
// Sexo do envolvido para o SIPOM, quando o dossiê não diz. 0 limpa.
func (a *app) handleSipomSetSexo(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	id, pid := r.PathValue("id"), r.PathValue("pid")
	var req struct {
		Sexo int `json:"sexo"`
	}
	if err := httpx.Decode(r, &req); err != nil || (req.Sexo != 0 && req.Sexo != sipom.SexoMasculino && req.Sexo != sipom.SexoFeminino) {
		httpx.Error(w, http.StatusBadRequest, "sexo deve ser 1 (masculino), 2 (feminino) ou 0 (limpar)")
		return
	}
	before, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	if err := a.opsReports.SetPersonSipomSexo(r.Context(), id, pid, req.Sexo); err != nil {
		if errors.Is(err, opsreport.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "envolvido não encontrado")
			return
		}
		log.Printf("sipom set sexo: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar")
		return
	}
	a.finishSipomChange(w, r, id, "opsreport.sipom.pessoa.sexo", before, nil)
}

// PUT /api/ops-occurrences/{id}/sipom/veiculos/{pos}/ano
//
//	{"ano_fabricacao": 2019, "ano_modelo": 2020 | null}
//
// O relatório não traz o ano; o SIPOM exige o de fabricação para placa nova.
func (a *app) handleSipomSetVehicleYears(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	id := r.PathValue("id")
	pos, err := strconv.Atoi(r.PathValue("pos"))
	if err != nil || pos < 0 {
		httpx.Error(w, http.StatusBadRequest, "posição inválida")
		return
	}
	var req struct {
		AnoFabricacao *int `json:"ano_fabricacao"`
		AnoModelo     *int `json:"ano_modelo"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	maxYear := time.Now().Year() + 1
	for _, y := range []*int{req.AnoFabricacao, req.AnoModelo} {
		if y != nil && (*y < 1900 || *y > maxYear) {
			httpx.Error(w, http.StatusBadRequest, "ano entre 1900 e "+strconv.Itoa(maxYear))
			return
		}
	}
	if req.AnoFabricacao != nil && req.AnoModelo != nil && (*req.AnoModelo < *req.AnoFabricacao || *req.AnoModelo > *req.AnoFabricacao+1) {
		httpx.Error(w, http.StatusBadRequest, "ano do modelo deve ser o de fabricação ou o seguinte")
		return
	}
	before, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	if pos >= len(before.Vehicles) {
		httpx.Error(w, http.StatusNotFound, "veículo não encontrado")
		return
	}
	if err := a.opsReports.SetVehicleYears(r.Context(), id, pos, req.AnoFabricacao, req.AnoModelo); err != nil {
		log.Printf("sipom set vehicle years: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar")
		return
	}
	a.finishSipomChange(w, r, id, "opsreport.sipom.veiculo.ano", before, nil)
}
