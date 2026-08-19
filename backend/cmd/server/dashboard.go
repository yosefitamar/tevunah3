package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/dashboard"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// ─── GET /api/dashboard ────────────────────────────────────────────────
//
// Painel operacional. Não há permissão própria de dashboard: cada bloco sai
// na resposta apenas se o solicitante tem a ação de leitura do módulo
// correspondente (incident.read, report.read, informe.read, entity.list).
// Quem não tem nenhuma recebe um envelope só com o recorte — e o front
// mostra o vazio, sem 403 (o painel em si não é recurso restrito).
//
// Parâmetros: date_from/date_to (YYYY-MM-DD, default = mês corrente no fuso
// da agência) e all=1 para o acervo inteiro, sem recorte.

type dashboardPeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type dashboardFacet struct {
	Name  string `json:"name"`
	City  string `json:"city,omitempty"`
	Count int    `json:"count"`
}

type dashboardMonth struct {
	Month     string `json:"month"`
	Homicidio int    `json:"homicidio"`
	Apreensao int    `json:"apreensao"`
	Prisao    int    `json:"prisao"`
}

type dashboardIncidents struct {
	ByType        map[string]int   `json:"by_type"`
	PrevByType    map[string]int   `json:"prev_by_type"`
	Total         int              `json:"total"`
	PrevTotal     int              `json:"prev_total"`
	Series        []dashboardMonth `json:"series"`
	Means         []dashboardFacet `json:"means"`
	Cities        []dashboardFacet `json:"cities"`
	Neighborhoods []dashboardFacet `json:"neighborhoods"`
	Geocoded      int              `json:"geocoded"`
}

type dashboardReports struct {
	ByStatus     map[string]int `json:"by_status"`
	Created      int            `json:"created"`
	Diffused     int            `json:"diffused"`
	PrevDiffused int            `json:"prev_diffused"`
}

type dashboardInformes struct {
	Total   int `json:"total"`
	Created int `json:"created"`
	Prev    int `json:"prev"`
}

type dashboardEntities struct {
	ByKind   map[string]int `json:"by_kind"`
	Created  int            `json:"created"`
	Prev     int            `json:"prev"`
	Deceased int            `json:"deceased"`
}

type dashboardResponse struct {
	Period       dashboardPeriod     `json:"period"`
	Previous     *dashboardPeriod    `json:"previous,omitempty"`
	SeriesPeriod dashboardPeriod     `json:"series_period"`
	Incidents    *dashboardIncidents `json:"incidents,omitempty"`
	Reports      *dashboardReports   `json:"reports,omitempty"`
	Informes     *dashboardInformes  `json:"informes,omitempty"`
	Entities     *dashboardEntities  `json:"entities,omitempty"`
}

// incidentTypes fixa a ordem e garante que os três tipos apareçam no JSON
// mesmo zerados — o painel desenha as três colunas sempre.
var incidentTypes = []string{"homicidio", "apreensao", "prisao"}

func (a *app) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	me := middleware.UserFrom(ctx)

	actions, err := a.policy.AllowedActions(ctx, me.Roles)
	if err != nil {
		log.Printf("dashboard: policy: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro de autorização")
		return
	}
	allowed := make(map[string]bool, len(actions))
	for _, act := range actions {
		allowed[act] = true
	}

	q := r.URL.Query()
	from := strings.TrimSpace(q.Get("date_from"))
	to := strings.TrimSpace(q.Get("date_to"))
	if q.Get("all") != "1" && from == "" && to == "" {
		p := dashboard.CurrentMonth(a.tz, time.Now())
		from, to = p.From, p.To
	}

	win, err := dashboard.BuildWindow(from, to, time.Now())
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	res := dashboardResponse{
		Period:       dashboardPeriod{From: win.Current.From, To: win.Current.To},
		SeriesPeriod: dashboardPeriod{From: win.Series.From, To: win.Series.To},
	}
	if win.Previous.From != "" {
		res.Previous = &dashboardPeriod{From: win.Previous.From, To: win.Previous.To}
	}

	access := dashboard.Access{
		UserID:    me.ID,
		Clearance: me.ClearanceLevel,
		IsAdmin:   hasRole(me.Roles, "administrador"),
	}

	if allowed["incident.read"] {
		st, err := a.dashboard.Incidents(ctx, win)
		if err != nil {
			log.Printf("dashboard: incidents: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao montar o painel")
			return
		}
		res.Incidents = toDashboardIncidents(st)
	}

	if allowed["report.read"] {
		st, err := a.dashboard.Reports(ctx, win, access)
		if err != nil {
			log.Printf("dashboard: reports: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao montar o painel")
			return
		}
		res.Reports = &dashboardReports{
			ByStatus:     fillKeys(st.ByStatus, "criado", "difundido", "arquivado"),
			Created:      st.Created,
			Diffused:     st.Diffused,
			PrevDiffused: st.PrevDiffused,
		}
	}

	if allowed["informe.read"] {
		st, err := a.dashboard.Informes(ctx, win, access)
		if err != nil {
			log.Printf("dashboard: informes: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao montar o painel")
			return
		}
		res.Informes = &dashboardInformes{Total: st.Total, Created: st.Created, Prev: st.Prev}
	}

	if allowed["entity.list"] {
		st, err := a.dashboard.Entities(ctx, win)
		if err != nil {
			log.Printf("dashboard: entities: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao montar o painel")
			return
		}
		res.Entities = &dashboardEntities{
			ByKind:   fillKeys(st.ByKind, "person", "organization", "place", "vehicle"),
			Created:  st.Created,
			Prev:     st.Prev,
			Deceased: st.Deceased,
		}
	}

	httpx.OK(w, res)
}

func toDashboardIncidents(st *dashboard.IncidentStats) *dashboardIncidents {
	out := &dashboardIncidents{
		ByType:        fillKeys(st.ByType, incidentTypes...),
		PrevByType:    fillKeys(st.PrevByType, incidentTypes...),
		Series:        make([]dashboardMonth, 0, len(st.Series)),
		Means:         toDashboardFacets(st.Means),
		Cities:        toDashboardFacets(st.Cities),
		Neighborhoods: toDashboardFacets(st.Neighborhoods),
		Geocoded:      st.Geocoded,
	}
	for _, t := range incidentTypes {
		out.Total += out.ByType[t]
		out.PrevTotal += out.PrevByType[t]
	}
	for _, m := range st.Series {
		out.Series = append(out.Series, dashboardMonth{
			Month:     m.Month,
			Homicidio: m.Homicidio,
			Apreensao: m.Apreensao,
			Prisao:    m.Prisao,
		})
	}
	return out
}

func toDashboardFacets(in []dashboard.Facet) []dashboardFacet {
	out := make([]dashboardFacet, 0, len(in))
	for _, f := range in {
		out = append(out, dashboardFacet{Name: f.Name, City: f.City, Count: f.Count})
	}
	return out
}

// fillKeys garante presença das chaves conhecidas (zeradas quando ausentes),
// para o cliente não precisar tratar buraco em cada leitura.
func fillKeys(m map[string]int, keys ...string) map[string]int {
	if m == nil {
		m = map[string]int{}
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			m[k] = 0
		}
	}
	return m
}
