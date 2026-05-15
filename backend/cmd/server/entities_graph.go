package main

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// publicGraphNode é o DTO de um nó no JSON do grafo. Inclui kind, name e a
// classification (pra UI poder pintar a pill) + summary de veículo opcional.
type publicGraphNode struct {
	ID             string                `json:"id"`
	Kind           string                `json:"kind"`
	Name           string                `json:"name"`
	Classification int                   `json:"classification"`
	Version        int                   `json:"version"`
	HasPhoto       bool                  `json:"has_photo"`
	Alias          *string               `json:"alias,omitempty"`
	OrcrimAlias    *string               `json:"orcrim_alias,omitempty"`
	Vehicle        *publicVehicleSummary `json:"vehicle,omitempty"`
}

// publicGraphEdge é a aresta direcional. Não traz datas de validade — o grafo
// é uma visão "agora"; histórico de vínculos é checado na lista do dossiê.
type publicGraphEdge struct {
	ID           string `json:"id"`
	From         string `json:"from"`
	To           string `json:"to"`
	RelationType string `json:"relation_type"`
	Note         string `json:"note,omitempty"`
}

type publicGraph struct {
	CenterID  string            `json:"center_id"`
	Depth     int               `json:"depth"`
	Nodes     []publicGraphNode `json:"nodes"`
	Edges     []publicGraphEdge `json:"edges"`
	Truncated bool              `json:"truncated"`
}

// handleEntityGraph atende GET /api/entities/{id}/graph?depth=N.
// Valida depth (1..3, default 2), checa visibilidade do centro pelo clearance
// do chamador, chama o domain BuildGraph e audita o evento entity.graph.view.
func (a *app) handleEntityGraph(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.read") {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	me := middleware.UserFrom(r.Context())

	depth := 2
	if s := r.URL.Query().Get("depth"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			depth = v
		}
	}
	if depth < 1 {
		depth = 1
	}
	if depth > 3 {
		depth = 3
	}

	// Pré-check de visibilidade do nó central — espelha o handler de links.
	center, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if center.Classification > me.ClearanceLevel || center.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}

	g, err := a.entities.BuildGraph(r.Context(), id, depth, me.ClearanceLevel)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		log.Printf("graph build: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao montar grafo")
		return
	}

	out := publicGraph{
		CenterID:  g.CenterID,
		Depth:     g.Depth,
		Truncated: g.Truncated,
		Nodes:     make([]publicGraphNode, 0, len(g.Nodes)),
		Edges:     make([]publicGraphEdge, 0, len(g.Edges)),
	}
	for _, n := range g.Nodes {
		out.Nodes = append(out.Nodes, publicGraphNode{
			ID:             n.ID,
			Kind:           string(n.Kind),
			Name:           n.Name,
			Classification: n.Classification,
			Version:        n.Version,
			HasPhoto:       n.HasPhoto,
			Alias:          n.Alias,
			OrcrimAlias:    n.OrcrimAlias,
			Vehicle:        toPublicVehicleSummary(n.Vehicle),
		})
	}
	for _, e := range g.Edges {
		out.Edges = append(out.Edges, publicGraphEdge{
			ID:           e.ID,
			From:         e.From,
			To:           e.To,
			RelationType: string(e.RelationType),
			Note:         e.Note,
		})
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := center.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.graph.view",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(id),
		ResourceClassification: &classPtr,
		After: map[string]any{
			"center_id": g.CenterID,
			"depth":     g.Depth,
			"nodes":     len(out.Nodes),
			"edges":     len(out.Edges),
			"truncated": g.Truncated,
		},
	})

	httpx.OK(w, out)
}
