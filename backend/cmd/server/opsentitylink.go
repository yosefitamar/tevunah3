package main

import (
	"log"
	"net/http"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/intel"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/opsreport"
)

// Dossiê novo × relatório operacional: quando uma pessoa é cadastrada, o
// sistema procura as ocorrências em que ela já apareceu como envolvido sem
// dossiê. A comparação é pelo som (intel.NameMatch) — o relatório é digitado
// à mão, e "Sousa" e "Souza" são a mesma pessoa.

// opsEntityMatchJSON é uma ocorrência em que o nome do dossiê aparece.
type opsEntityMatchJSON struct {
	OccurrenceID string   `json:"occurrence_id"`
	PersonID     string   `json:"person_id"`
	CIOPS        string   `json:"ciops_record"`
	OccurredOn   string   `json:"occurred_on"`
	Natures      []string `json:"natures"`
	Role         string   `json:"role"`
	Name         string   `json:"name"`
	MotherName   string   `json:"mother_name"`
	// Approx: o nome no relatório está grafado de outro jeito.
	Approx bool `json:"approx"`
}

// opsEntityLinksJSON é o que a criação do dossiê devolve sobre o relatório
// operacional: o que foi vinculado sozinho e o que fica para o analista.
type opsEntityLinksJSON struct {
	// Linked: nome e mãe batem — vínculo criado.
	Linked []opsEntityMatchJSON `json:"linked"`
	// Suggestions: só o nome bate (o relatório, ou o dossiê, não traz a
	// mãe) — o analista confere e vincula.
	Suggestions []opsEntityMatchJSON `json:"suggestions"`
}

// linkOpsPeopleTo procura o dossiê recém-criado entre os envolvidos sem
// dossiê e vincula os que casam por nome e mãe. Homônimo com mãe diferente é
// outra pessoa e fica de fora; sem a mãe de um dos lados vira sugestão. Quem
// não pode alterar o relatório operacional só recebe as sugestões; quem não
// pode lê-lo, nada. Falha aqui não desfaz a criação do dossiê: só registra.
func (a *app) linkOpsPeopleTo(r *http.Request, e *entities.Entity) *opsEntityLinksJSON {
	if e.Kind != entities.KindPerson || !a.hasPerm(r, "opsreport.read") {
		return nil
	}
	ctx := r.Context()
	people, err := a.opsReports.UnlinkedPeople(ctx)
	if err != nil {
		log.Printf("opsreport unlinked people: %v", err)
		return nil
	}
	mother := ""
	if e.Person != nil && e.Person.MotherName != nil {
		mother = *e.Person.MotherName
	}
	canLink := a.hasPerm(r, "opsreport.update")
	me := middleware.UserFrom(ctx)
	out := &opsEntityLinksJSON{Linked: []opsEntityMatchJSON{}, Suggestions: []opsEntityMatchJSON{}}
	for _, p := range people {
		lv := intel.NameMatch(p.Name, e.Name)
		if lv == intel.NameNone {
			continue
		}
		m := opsEntityMatchJSON{
			OccurrenceID: p.OccurrenceID, PersonID: p.ID, CIOPS: p.CIOPS,
			OccurredOn: p.OccurredOn.Format("2006-01-02"), Natures: p.Natures,
			Role: p.Role, Name: p.Name, MotherName: p.MotherName, Approx: lv != intel.NameExact,
		}
		if m.Natures == nil {
			m.Natures = []string{}
		}
		if len(intel.NameTokens(p.MotherName)) == 0 || len(intel.NameTokens(mother)) == 0 {
			out.Suggestions = append(out.Suggestions, m)
			continue
		}
		mlv := intel.NameMatch(p.MotherName, mother)
		if mlv == intel.NameNone {
			continue // homônimo com outra mãe
		}
		if !canLink {
			out.Suggestions = append(out.Suggestions, m)
			continue
		}
		if err := a.opsReports.LinkPerson(ctx, p.OccurrenceID, p.ID, e.ID, opsreport.LinkAuto, me.ID); err != nil {
			log.Printf("opsreport auto link on entity create: %v", err)
			out.Suggestions = append(out.Suggestions, m)
			continue
		}
		m.Approx = m.Approx || mlv != intel.NameExact
		aid, sid, ip, ua := a.actorInfo(r)
		occID := p.OccurrenceID
		_ = a.audit.Log(ctx, audit.Entry{
			ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
			Action:       "opsreport.person.link",
			ResourceType: audit.Ptr("ops_occurrence"),
			ResourceID:   &occID,
			Reason:       audit.Ptr("vínculo automático na criação do dossiê (nome e mãe)"),
			Before:       map[string]any{"person_id": p.ID, "name": p.Name, "entity_id": nil, "link_mode": ""},
			After:        map[string]any{"entity_id": e.ID, "link_mode": opsreport.LinkAuto},
		})
		a.sipomRecomputeOne(ctx, p.OccurrenceID)
		out.Linked = append(out.Linked, m)
	}
	return out
}
