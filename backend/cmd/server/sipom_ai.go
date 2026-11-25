package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/belia/tevunah/backend/internal/ai"
	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipomai"
)

// POST /api/ops-occurrences/{id}/sipom/ia
//
// Pede à IA (chave e modelo do próprio usuário) sugestões para as
// pendências da ocorrência que travam o envio ao SIPOM. Nada é gravado: o
// analista confirma cada sugestão, que então passa pelo endpoint de
// correção de sempre.
func (a *app) handleSipomAISuggest(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	if a.sipom == nil {
		httpx.Error(w, http.StatusServiceUnavailable, "catálogo do SIPOM não carregado")
		return
	}
	id := r.PathValue("id")
	so, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	terms, err := a.sipomTerms.Load(r.Context())
	if err != nil {
		log.Printf("sipom ia terms: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler os termos aprendidos")
		return
	}
	me := middleware.UserFrom(r.Context())
	active, err := a.ai.ActiveGuidance(r.Context(), sipomai.Feature, me.ID)
	if err != nil {
		log.Printf("sipom ia orientações: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler as orientações")
		return
	}
	guidance := make([]sipomai.Guidance, 0, len(active))
	for _, g := range active {
		guidance = append(guidance, sipomai.Guidance{
			ID: g.ID, Topic: g.Topic, CidadeID: g.CidadeID, Body: g.Body, Author: g.AuthorName,
			Date: g.CreatedAt.In(a.agencyLocation()).Format("02/01/2006"), Pending: g.Status == ai.GuidancePending,
		})
	}
	req, ok := sipomai.Build(a.sipom, terms, so, guidance)
	// Releitura do PDF original: campos que a importação pode ter lido errado.
	rr := a.sipomReread(r.Context(), so)
	// Delegacia que a tradução não identificou: a IA escolhe no catálogo.
	dg := sipomai.BuildDelegacia(a.sipom, so)
	if !ok && rr == nil && dg == nil {
		httpx.OK(w, map[string]any{"suggestions": []sipomai.Suggestion{}, "sections": []string{}})
		return
	}
	sections := []string{}
	if ok {
		sections = append(sections, req.Sections...)
	}
	if rr != nil {
		sections = append(sections, "releitura")
	}
	if dg != nil {
		sections = append(sections, "delegacia")
	}
	// Nova verificação: a análise anterior (e o que o analista fez com ela)
	// entra em cada pedido.
	prev, err := a.ai.LatestAnalysis(r.Context(), sipomai.Feature, id)
	if err != nil {
		log.Printf("sipom ia análise anterior: %v", err)
		prev = nil
	}
	prevText := a.sipomPrevious(prev).Prompt()

	// Até cinco chamadas em paralelo (juntas, o schema passa do limite do
	// structured outputs): a principal (pendências), a das apreensões que
	// faltam e a da recusa do SIPOM. A das apreensões é complemento — se
	// falhar, as outras valem sozinhas.
	type call struct {
		name   string
		prompt string
		schema map[string]any
		max    int
		parse  func([]byte) ([]sipomai.Suggestion, error)
		// filled by the goroutine
		list []sipomai.Suggestion
		res  *ai.StructuredResult
		err  error
	}
	var calls []*call
	if ok {
		calls = append(calls,
			&call{name: "principal", prompt: req.Prompt, schema: sipomai.Schema, max: 6000, parse: req.Parse},
			&call{name: "apreensões que faltam", prompt: req.AddPrompt, schema: sipomai.AddSchema, max: 3000, parse: req.ParseAdd},
			&call{name: "recusa", prompt: req.RefusalPrompt, schema: sipomai.RefusalSchema, max: 3000, parse: req.ParseRefusal},
		)
	}
	if rr != nil {
		calls = append(calls, &call{name: "releitura", prompt: rr.Prompt, schema: sipomai.RereadSchema, max: 3000, parse: rr.Parse})
	}
	if dg != nil {
		calls = append(calls, &call{name: "delegacia", prompt: dg.Prompt, schema: sipomai.DelegaciaSchema, max: 1500, parse: dg.Parse})
	}
	var wg sync.WaitGroup
	for _, c := range calls {
		if c.prompt == "" {
			continue
		}
		wg.Add(1)
		go func(c *call) {
			defer wg.Done()
			c.res, c.err = a.ai.Structured(r.Context(), me.ID, ai.StructuredRequest{
				Feature: sipomai.Feature, System: sipomai.System, Prompt: c.prompt + prevText,
				Schema: c.schema, MaxTokens: c.max, Effort: "low",
			})
			if c.err == nil {
				if c.list, c.err = c.parse(c.res.JSON); c.err != nil {
					log.Printf("sipom ia parse (%s): %v", c.name, c.err)
					c.err = ai.ErrBadOutput
				}
			} else {
				log.Printf("sipom ia (%s): %v", c.name, c.err)
			}
		}(c)
	}
	wg.Wait()
	// Erro que vale para a resposta: o da principal ou, sem ela, o da recusa.
	// Os de chave, limite e recusa da IA valem de qualquer uma.
	err = nil
	// Lista, nunca null: a tela itera sobre ela.
	suggestions := []sipomai.Suggestion{}
	var res *ai.StructuredResult
	ran := 0
	for _, c := range calls {
		if c.prompt == "" {
			continue
		}
		ran++
		if res == nil && c.res != nil {
			res = c.res
		}
		if c.err == nil {
			suggestions = append(suggestions, c.list...)
		} else if c.name != "apreensões que faltam" && err == nil {
			err = c.err
		}
	}
	// Uma das chamadas deu certo: mostra o que veio, salvo erro de chave/limite.
	if err != nil && len(suggestions) > 0 && !errors.Is(err, ai.ErrNoKey) && !errors.Is(err, ai.ErrKeyRejected) &&
		!errors.Is(err, ai.ErrKeyForbidden) {
		err = nil
	}
	if ran == 0 {
		httpx.OK(w, map[string]any{"suggestions": []sipomai.Suggestion{}, "sections": []string{}})
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	entry := audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "opsreport.sipom.ia",
		ResourceType: audit.Ptr("ops_occurrence"),
		ResourceID:   &id,
	}
	if res != nil {
		in, outTok := 0, 0
		for _, c := range calls {
			if c.res != nil {
				in += c.res.Usage.InputTokens
				outTok += c.res.Usage.OutputTokens
			}
		}
		after := map[string]any{
			"model": res.Model, "sections": sections, "recusa": ok && req.RefusalPrompt != "", "releitura": rr != nil,
			"input_tokens": in, "output_tokens": outTok,
		}
		if err != nil {
			after["error"] = err.Error()
		} else {
			after["suggestions"] = len(suggestions)
		}
		entry.After = after
		_ = a.audit.Log(r.Context(), entry)
	}

	if err != nil {
		switch {
		case errors.Is(err, ai.ErrNoKey):
			httpx.Error(w, http.StatusPreconditionFailed, "cadastre sua chave da API Anthropic em SISTEMA → ASSISTENTE IA")
		case errors.Is(err, ai.ErrKeyRejected), errors.Is(err, ai.ErrKeyForbidden):
			httpx.Error(w, http.StatusPreconditionFailed, err.Error()+" (SISTEMA → ASSISTENTE IA)")
		case errors.Is(err, ai.ErrRateLimited):
			httpx.Error(w, http.StatusTooManyRequests, err.Error())
		case errors.Is(err, ai.ErrRefused), errors.Is(err, ai.ErrTruncated), errors.Is(err, ai.ErrBadOutput),
			errors.Is(err, ai.ErrUpstream):
			httpx.Error(w, http.StatusBadGateway, err.Error())
		default:
			log.Printf("sipom ia: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao consultar a IA")
		}
		return
	}

	if used := sipomai.UsedGuidance(suggestions); len(used) > 0 {
		if err := a.ai.MarkGuidanceUsed(r.Context(), used); err != nil {
			log.Printf("sipom ia uso das orientações: %v", err)
		}
	}
	cost := 0.0
	for _, c := range calls {
		x := c.res
		if x == nil {
			continue
		}
		if m := ai.FindModel(x.Model); m != nil {
			u := x.Usage
			cost += m.Cost(u.InputTokens, u.OutputTokens, u.CacheWriteTokens, u.CacheReadTokens)
		}
	}
	in, outTok := 0, 0
	for _, c := range calls {
		if c.res != nil {
			in += c.res.Usage.InputTokens
			outTok += c.res.Usage.OutputTokens
		}
	}
	// Guardada: a tela mostra esta análise sem consultar de novo.
	saved := &ai.Analysis{Feature: sipomai.Feature, OccurrenceID: id, CreatedBy: me.ID, Model: res.Model,
		InputTokens: in, OutputTokens: outTok, CostUSD: cost, Sections: sections, Fingerprint: sipomFingerprint(so)}
	if prev != nil {
		saved.PreviousID = &prev.ID
	}
	if aid, at, err := a.ai.SaveAnalysis(r.Context(), ai.NewAnalysis{
		Feature: sipomai.Feature, OccurrenceID: id, UserID: me.ID, PreviousID: saved.PreviousID,
		Model: res.Model, InputTokens: in, OutputTokens: outTok, CostUSD: cost,
		Sections: sections, Suggestions: suggestions, Fingerprint: saved.Fingerprint,
	}); err != nil {
		log.Printf("sipom ia (gravar análise): %v", err)
		saved = nil
	} else {
		saved.ID, saved.CreatedAt, saved.AuthorName = aid, at, me.DisplayName
		if saved.AuthorName == "" {
			saved.AuthorName = me.Email
		}
	}
	httpx.OK(w, map[string]any{
		"suggestions": suggestions,
		"sections":    sections,
		"model":       res.Model,
		"cost_usd":    cost,
		"cidade":      a.sipomCidade(so),
		"analysis":    a.analysisJSON(saved, so, prev != nil),
	})
}

// GET /api/ops-occurrences/{id}/sipom/ia
//
// A última análise guardada da ocorrência, sem consultar a IA. analysis
// null = nunca foi analisada.
func (a *app) handleSipomAIGet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	id := r.PathValue("id")
	so, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	an, err := a.ai.LatestAnalysis(r.Context(), sipomai.Feature, id)
	if err != nil {
		log.Printf("sipom ia (ler análise): %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler a análise")
		return
	}
	if an == nil {
		httpx.OK(w, map[string]any{"analysis": nil, "suggestions": []sipomai.Suggestion{}, "sections": []string{}})
		return
	}
	suggestions := []sipomai.Suggestion{}
	_ = json.Unmarshal(an.Suggestions, &suggestions)
	httpx.OK(w, map[string]any{
		"suggestions": suggestions,
		"sections":    nonNil(an.Sections),
		"model":       an.Model,
		"cost_usd":    an.CostUSD,
		"cidade":      a.sipomCidade(so),
		"analysis":    a.analysisJSON(an, so, an.PreviousID != nil),
	})
}

// PUT /api/ops-occurrences/{id}/sipom/ia/{aid}/resultado
//
// O que o analista fez com a análise: aplicadas, não aceitas (estavam à
// vista e ele não marcou) e as que falharam ao gravar, pela posição. Entra
// na próxima verificação.
func (a *app) handleSipomAIResult(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	id, aid := r.PathValue("id"), r.PathValue("aid")
	var req struct {
		Aplicadas []int             `json:"aplicadas"`
		Recusadas []int             `json:"recusadas"`
		Falhas    map[string]string `json:"falhas"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	an, err := a.ai.AnalysisByID(r.Context(), sipomai.Feature, id, aid)
	if err != nil {
		if errors.Is(err, ai.ErrAnalysisNotFound) {
			httpx.Error(w, http.StatusConflict, "a análise não é mais a última desta ocorrência")
			return
		}
		log.Printf("sipom ia resultado: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao ler a análise")
		return
	}
	var list []json.RawMessage
	_ = json.Unmarshal(an.Suggestions, &list)
	valid := func(xs []int) []int {
		out := []int{}
		for _, x := range xs {
			if x >= 0 && x < len(list) {
				out = append(out, x)
			}
		}
		return out
	}
	falhas := map[string]string{}
	for k, v := range req.Falhas {
		if n, err := strconv.Atoi(k); err == nil && n >= 0 && n < len(list) {
			if rs := []rune(strings.TrimSpace(v)); len(rs) > 300 {
				v = string(rs[:300])
			}
			falhas[strconv.Itoa(n)] = strings.TrimSpace(v)
		}
	}
	an.MergeResult(valid(req.Aplicadas), valid(req.Recusadas), falhas)
	fp := ""
	if so, err := a.opsReports.FindByID(r.Context(), id); err == nil {
		fp = sipomFingerprint(so)
	}
	if err := a.ai.SaveAnalysisResult(r.Context(), an, fp); err != nil {
		log.Printf("sipom ia resultado (gravar): %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar o resultado")
		return
	}
	so, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	httpx.OK(w, map[string]any{"analysis": a.analysisJSON(an, so, an.PreviousID != nil)})
}

// sipomCidade é a cidade da ocorrência no catálogo (orientação só da cidade).
func (a *app) sipomCidade(so *opsreport.StoredOccurrence) *sipomRefJSON {
	if so.Sipom.CidadeID == nil || *so.Sipom.CidadeID == 0 {
		return nil
	}
	return &sipomRefJSON{ID: *so.Sipom.CidadeID, Nome: a.sipom.CidadeNome(*so.Sipom.CidadeID)}
}

// sipomFingerprint resume o estado da ficha que a análise considerou: as
// pendências e a última correção. Mudou, a análise pode estar velha.
func sipomFingerprint(so *opsreport.StoredOccurrence) string {
	p := append([]string(nil), so.Sipom.Pending...)
	sort.Strings(p)
	upd := ""
	if so.UpdatedAt != nil {
		upd = so.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	h := sha256.Sum256([]byte(strings.Join(p, ",") + "|" + upd))
	return hex.EncodeToString(h[:8])
}

// sipomPrevious traduz a análise guardada para o pedido da nova verificação.
func (a *app) sipomPrevious(an *ai.Analysis) *sipomai.Previous {
	if an == nil {
		return nil
	}
	p := &sipomai.Previous{
		When:     an.CreatedAt.In(a.agencyLocation()).Format("02/01/2006 15:04"),
		Applied:  map[int]bool{},
		Rejected: map[int]bool{},
		Failed:   map[int]string{},
		Reviewed: an.ResultAt != nil,
	}
	_ = json.Unmarshal(an.Suggestions, &p.Suggestions)
	for _, i := range an.Applied {
		p.Applied[i] = true
	}
	for _, i := range an.Rejected {
		p.Rejected[i] = true
	}
	for k, v := range an.Failed {
		if n, err := strconv.Atoi(k); err == nil {
			p.Failed[n] = v
		}
	}
	return p
}

type sipomAnalysisJSON struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Author    string    `json:"author"`
	Model     string    `json:"model"`
	CostUSD   float64   `json:"cost_usd"`
	// Stale: a ficha mudou depois da análise (por fora dela).
	Stale bool `json:"stale"`
	// Refined: esta análise recebeu a anterior.
	Refined bool `json:"refined"`
	// Status por posição: "aplicada" | "recusada" | "falhou".
	Status map[string]string `json:"status"`
	Errors map[string]string `json:"errors"`
}

func (a *app) analysisJSON(an *ai.Analysis, so *opsreport.StoredOccurrence, refined bool) *sipomAnalysisJSON {
	if an == nil {
		return nil
	}
	st := map[string]string{}
	for _, i := range an.Rejected {
		st[strconv.Itoa(i)] = "recusada"
	}
	for k := range an.Failed {
		st[k] = "falhou"
	}
	for _, i := range an.Applied {
		st[strconv.Itoa(i)] = "aplicada"
	}
	errs := an.Failed
	if errs == nil {
		errs = map[string]string{}
	}
	return &sipomAnalysisJSON{
		ID: an.ID, CreatedAt: an.CreatedAt, Author: an.AuthorName, Model: an.Model, CostUSD: an.CostUSD,
		Stale: an.Fingerprint != "" && an.Fingerprint != sipomFingerprint(so), Refined: refined,
		Status: st, Errors: errs,
	}
}

// sipomReread monta a releitura quando a ficha tem pendência que pode ser
// leitura errada do PDF (ou campo com cara disso) e o PDF original está
// guardado. Sem PDF ou sem achar a ficha nele, não há releitura.
func (a *app) sipomReread(ctx context.Context, so *opsreport.StoredOccurrence) *sipomai.Reread {
	if !sipomai.NeedsReread(so) || so.Page < 1 {
		return nil
	}
	rep, err := a.opsReports.FindReport(ctx, so.ReportID)
	if err != nil {
		log.Printf("sipom ia releitura (relatório): %v", err)
		return nil
	}
	path := filepath.Join(photoDir(), opsreport.ReportsSubdir, rep.FileSHA256+".pdf")
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	// A ocorrência começa na página gravada e pode seguir por mais duas.
	text, err := opsreport.LayoutText(ctx, path, so.Page, so.Page+2)
	if err != nil {
		log.Printf("sipom ia releitura (pdf): %v", err)
		return nil
	}
	return sipomai.BuildReread(so, opsreport.OccurrenceSource(text, so.CIOPS))
}
