package main

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/opsreport"
)

// POST /api/ops-occurrences/{id}/descartar  {"motivo": "..."}
//
// Descarta a ocorrência importada que não deveria estar no acervo: exclusão
// lógica, com o motivo no trilho de auditoria. Ela sai das listagens, da fila
// do SIPOM e dos dossiês, e a reimportação da mesma ficha não a traz de volta.
// A já enviada ao SIPOM não é descartada: lá ela continua existindo.
func (a *app) handleOpsOccurrenceDiscard(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.delete") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	var req struct {
		Motivo string `json:"motivo"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	motivo := strings.Join(strings.Fields(req.Motivo), " ")
	if n := utf8.RuneCountInString(motivo); n < 5 || n > 500 {
		httpx.Error(w, http.StatusBadRequest, "informe o motivo do descarte (5 a 500 caracteres)")
		return
	}
	before, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	if before.Sipom.SentAt != nil {
		httpx.Error(w, http.StatusConflict,
			"a ocorrência já foi enviada ao SIPOM — descartá-la aqui não a apaga lá")
		return
	}
	if err := a.opsReports.Discard(r.Context(), id, me.ID, motivo); err != nil {
		if errors.Is(err, opsreport.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
			return
		}
		log.Printf("opsreport discard: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao descartar")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "opsreport.occurrence.discard",
		ResourceType: audit.Ptr("ops_occurrence"),
		ResourceID:   &id,
		Before: map[string]any{
			"ciops_record": before.CIOPS,
			"occurred_on":  before.OccurredOn.Format("2006-01-02"),
			"natures":      before.Natures,
			"place_city":   before.PlaceCity,
		},
		After: map[string]any{"motivo": motivo},
	})
	httpx.NoContent(w)
}
