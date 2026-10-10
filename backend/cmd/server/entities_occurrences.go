package main

import (
	"errors"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// entityOccurrenceJSON é uma linha da seção OCORRÊNCIAS do dossiê. As duas
// fontes — cadastro de CVLI e relatório operacional — viram a mesma forma;
// Source diz qual drawer abrir.
type entityOccurrenceJSON struct {
	Source       string   `json:"source"` // "cvli" | "operacional"
	ID           string   `json:"id"`
	OccurredOn   string   `json:"occurred_on"`
	Time         string   `json:"time"`
	Type         string   `json:"type,omitempty"` // só CVLI
	Natures      []string `json:"natures"`        // só operacional
	Unit         string   `json:"unit,omitempty"` // CIA/PEL, só operacional
	City         string   `json:"city"`
	Neighborhood string   `json:"neighborhood"`
	CIOPSRecord  string   `json:"ciops_record"`
	Role         string   `json:"role"`
}

// GET /api/entities/{id}/occurrences
//
// Onde a entidade aparece. Cada fonte só entra se o solicitante lê o módulo
// dela (incident.read, opsreport.read); a entidade segue a mesma guarda do
// dossiê (clearance e exclusão).
func (a *app) handleEntityOccurrences(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.read") {
		return
	}
	me := middleware.UserFrom(r.Context())
	e, err := a.entities.FindByID(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		log.Printf("entity occurrences: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if e.Classification > me.ClearanceLevel || e.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}

	out := []entityOccurrenceJSON{}
	if a.hasPerm(r, "incident.read") {
		items, err := a.incidents.ListByEntity(r.Context(), e.ID)
		if err != nil {
			log.Printf("entity occurrences (cvli): %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
			return
		}
		for _, it := range items {
			j := entityOccurrenceJSON{
				Source: "cvli", ID: it.ID, OccurredOn: it.OccurredOn.Format("2006-01-02"),
				Type: it.Type, Natures: []string{}, City: it.City, Neighborhood: it.Neighborhood,
				CIOPSRecord: it.CIOPSRecord, Role: it.Role,
			}
			if it.OccurredTime != nil {
				j.Time = *it.OccurredTime
			}
			out = append(out, j)
		}
	}
	if a.hasPerm(r, "opsreport.read") {
		items, err := a.opsReports.ListByEntity(r.Context(), e.ID)
		if err != nil {
			log.Printf("entity occurrences (ops): %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
			return
		}
		for _, it := range items {
			out = append(out, entityOccurrenceJSON{
				Source: "operacional", ID: it.ID, OccurredOn: it.OccurredOn.Format("2006-01-02"),
				Time: it.StartTime, Natures: nonNil(it.Natures),
				Unit: strings.Join(nonEmpty(it.CIA, it.PEL), " · "),
				City: it.City, Neighborhood: it.Neighborhood, CIOPSRecord: it.CIOPS, Role: it.Role,
			})
		}
	}
	// Uma linha do tempo só, independente da fonte.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].OccurredOn != out[j].OccurredOn {
			return out[i].OccurredOn > out[j].OccurredOn
		}
		return out[i].Time > out[j].Time
	})
	httpx.OK(w, map[string]any{"items": out})
}

func nonEmpty(ss ...string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
