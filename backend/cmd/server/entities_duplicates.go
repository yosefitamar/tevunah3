package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

type personDuplicateJSON struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	MotherName    *string  `json:"mother_name,omitempty"`
	DateOfBirth   *string  `json:"date_of_birth,omitempty"`
	Score         int      `json:"score"`
	MatchedFields []string `json:"matched_fields"`
}

type duplicatesResultJSON struct {
	CPFTakenBy *personDuplicateJSON  `json:"cpf_taken_by,omitempty"`
	Matches    []personDuplicateJSON `json:"matches"`
}

// GET /api/entities/persons/duplicates?name=...&mother_name=...
//                                    &date_of_birth=...&cpf=...&exclude_id=...
//
// Devolve homônimos por nome (com score 1..3 baseado em quantos campos
// extras casam) e o portador atual de um CPF, se houver. Respeita clearance.
func (a *app) handleEntityPersonDuplicates(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.list") {
		return
	}
	me := middleware.UserFrom(r.Context())
	q := r.URL.Query()

	cpfDigits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, q.Get("cpf"))

	res, err := a.entities.FindPersonDuplicates(r.Context(), entities.DuplicatesQuery{
		Name:         strings.TrimSpace(q.Get("name")),
		MotherName:   strings.TrimSpace(q.Get("mother_name")),
		DateOfBirth:  strings.TrimSpace(q.Get("date_of_birth")),
		CPF:          cpfDigits,
		ExcludeID:    strings.TrimSpace(q.Get("exclude_id")),
		MaxClearance: me.ClearanceLevel,
	})
	if err != nil {
		log.Printf("duplicates: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar duplicates")
		return
	}

	out := duplicatesResultJSON{Matches: make([]personDuplicateJSON, 0, len(res.Matches))}
	if res.CPFTakenBy != nil {
		out.CPFTakenBy = toDuplicateJSON(res.CPFTakenBy)
	}
	for i := range res.Matches {
		out.Matches = append(out.Matches, *toDuplicateJSON(&res.Matches[i]))
	}
	httpx.OK(w, out)
}

func toDuplicateJSON(d *entities.PersonDuplicate) *personDuplicateJSON {
	out := &personDuplicateJSON{
		ID: d.ID, Name: d.Name, Score: d.Score, MatchedFields: d.MatchedFields,
		MotherName: d.MotherName,
	}
	if out.MatchedFields == nil {
		out.MatchedFields = []string{}
	}
	if d.DateOfBirth != nil {
		s := d.DateOfBirth.Format("2006-01-02")
		out.DateOfBirth = &s
	}
	return out
}
