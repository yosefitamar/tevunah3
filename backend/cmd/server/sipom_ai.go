package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

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
				Feature: sipomai.Feature, System: sipomai.System, Prompt: c.prompt,
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
	var cidade *sipomRefJSON
	if so.Sipom.CidadeID != nil && *so.Sipom.CidadeID != 0 {
		cidade = &sipomRefJSON{ID: *so.Sipom.CidadeID, Nome: a.sipom.CidadeNome(*so.Sipom.CidadeID)}
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
	httpx.OK(w, map[string]any{
		"suggestions": suggestions,
		"sections":    sections,
		"model":       res.Model,
		"cost_usd":    cost,
		"cidade":      cidade,
	})
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
