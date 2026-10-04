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
// correspondente (incident.read, opsreport.read, report.read, informe.read,
// entity.list).
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
	ByType     map[string]int   `json:"by_type"`
	PrevByType map[string]int   `json:"prev_by_type"`
	Total      int              `json:"total"`
	PrevTotal  int              `json:"prev_total"`
	Series     []dashboardMonth `json:"series"`
	Means      []dashboardFacet `json:"means"`
	Geocoded   int              `json:"geocoded"`
}

// dashboardOpsTotals são as quantidades do relatório operacional num recorte.
type dashboardOpsTotals struct {
	Occurrences int     `json:"occurrences"`
	Weapons     int     `json:"weapons"`
	DrugsGrams  float64 `json:"drugs_grams"`
	Accused     int     `json:"accused"`
	Adolescents int     `json:"adolescents"`
	Vehicles    int     `json:"vehicles"`
}

type dashboardDrugFacet struct {
	Name  string  `json:"name"`
	Grams float64 `json:"grams"`
}

type dashboardOpsMonth struct {
	Month string `json:"month"`
	Count int    `json:"count"`
}

type dashboardOperational struct {
	Current      dashboardOpsTotals   `json:"current"`
	Previous     dashboardOpsTotals   `json:"previous"`
	WeaponKinds  []dashboardFacet     `json:"weapon_kinds"`
	DrugKinds    []dashboardDrugFacet `json:"drug_kinds"`
	VehicleKinds []dashboardFacet     `json:"vehicle_kinds"`
	Series       []dashboardOpsMonth  `json:"series"`
	// Pending: ocorrências do período fora dos números por terem pendência
	// ou aviso — só as verificadas contam.
	Pending int `json:"pending"`
}

// dashboardTerritory soma as fontes de ocorrência que o solicitante enxerga.
type dashboardTerritory struct {
	Cities        []dashboardFacet `json:"cities"`
	Neighborhoods []dashboardFacet `json:"neighborhoods"`
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
	Period       dashboardPeriod       `json:"period"`
	Previous     *dashboardPeriod      `json:"previous,omitempty"`
	SeriesPeriod dashboardPeriod       `json:"series_period"`
	Incidents    *dashboardIncidents   `json:"incidents,omitempty"`
	Operational  *dashboardOperational `json:"operational,omitempty"`
	Territory    *dashboardTerritory   `json:"territory,omitempty"`
	Reports      *dashboardReports     `json:"reports,omitempty"`
	Informes     *dashboardInformes    `json:"informes,omitempty"`
	Entities     *dashboardEntities    `json:"entities,omitempty"`
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

	if allowed["opsreport.read"] {
		st, err := a.dashboard.Operational(ctx, win, allowed["incident.read"])
		if err != nil {
			log.Printf("dashboard: operational: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao montar o painel")
			return
		}
		res.Operational = toDashboardOperational(st)
	}

	if allowed["incident.read"] || allowed["opsreport.read"] {
		cities, hoods, err := a.dashboard.Territory(ctx, win.Current, allowed["incident.read"], allowed["opsreport.read"])
		if err != nil {
			log.Printf("dashboard: territory: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao montar o painel")
			return
		}
		res.Territory = &dashboardTerritory{Cities: toDashboardFacets(cities), Neighborhoods: toDashboardFacets(hoods)}
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
		ByType:     fillKeys(st.ByType, incidentTypes...),
		PrevByType: fillKeys(st.PrevByType, incidentTypes...),
		Series:     make([]dashboardMonth, 0, len(st.Series)),
		Means:      toDashboardFacets(st.Means),
		Geocoded:   st.Geocoded,
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

func toDashboardOperational(st *dashboard.OperationalStats) *dashboardOperational {
	totals := func(t dashboard.OperationalTotals) dashboardOpsTotals {
		return dashboardOpsTotals{
			Occurrences: t.Occurrences, Weapons: t.Weapons, DrugsGrams: t.DrugsGrams,
			Accused: t.Accused, Adolescents: t.Adolescents, Vehicles: t.Vehicles,
		}
	}
	out := &dashboardOperational{
		Current:      totals(st.Current),
		Previous:     totals(st.Previous),
		WeaponKinds:  toDashboardFacets(st.WeaponKinds),
		DrugKinds:    make([]dashboardDrugFacet, 0, len(st.DrugKinds)),
		VehicleKinds: toDashboardFacets(st.VehicleKinds),
		Series:       make([]dashboardOpsMonth, 0, len(st.Series)),
		Pending:      st.Pending,
	}
	for _, d := range st.DrugKinds {
		out.DrugKinds = append(out.DrugKinds, dashboardDrugFacet{Name: d.Name, Grams: d.Grams})
	}
	for _, m := range st.Series {
		out.Series = append(out.Series, dashboardOpsMonth{Month: m.Month, Count: m.Count})
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
