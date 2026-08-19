package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/incidents"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// maxReportChars limita o relatório colado. O documento real tem alguns
// milhares de caracteres; acima disso é colagem de conversa inteira, que só
// gastaria trabalho do parser sem render campo.
const maxReportChars = 50_000

type parseReportRequest struct {
	Text string `json:"text"`
	// ResolveLink permite ao cliente pedir a importação sem tocar no Google
	// (nil = resolve, que é o padrão da tela).
	ResolveLink *bool `json:"resolve_link,omitempty"`
}

type parsedPersonJSON struct {
	Name        string `json:"name"`
	MotherName  string `json:"mother_name,omitempty"`
	DateOfBirth string `json:"date_of_birth,omitempty"`
	CPF         string `json:"cpf,omitempty"`
	Alias       string `json:"alias,omitempty"`
	// Role vem vazio quando o papel não pôde ser deduzido com segurança
	// (sobrevivente, tentativa de homicídio) — a tela obriga a escolher.
	Role string `json:"role"`
	// Status é o que o relatório declarou: "obito" | "vivo" | "".
	Status string `json:"status,omitempty"`
	// Notes reúne endereço, antecedentes e demais dados que a ocorrência não
	// guarda; viram descrição do dossiê se a pessoa for cadastrada.
	Notes string `json:"notes,omitempty"`
	// Matches são dossiês já cadastrados que podem ser esta pessoa, com o
	// mesmo score da tela de cadastro de entidade. Vazio = ninguém parecido.
	Matches []personDuplicateJSON `json:"matches"`
}

type parsedReportJSON struct {
	Type         string             `json:"type"`
	Means        string             `json:"means"`
	MeansDetail  string             `json:"means_detail"`
	OccurredOn   string             `json:"occurred_on"`
	OccurredTime string             `json:"occurred_time"`
	CIOPSRecord  string             `json:"ciops_record"`
	City         string             `json:"city"`
	Neighborhood string             `json:"neighborhood"`
	Description  string             `json:"description"`
	MapsURL      string             `json:"maps_url,omitempty"`
	Latitude     *float64           `json:"latitude,omitempty"`
	Longitude    *float64           `json:"longitude,omitempty"`
	People       []parsedPersonJSON `json:"people"`
	Warnings     []string           `json:"warnings"`
}

// POST /api/incidents/parse
//
// Lê o relatório de CVLI colado do grupo operacional e devolve os campos
// reconhecidos para PRÉ-PREENCHER o formulário — nada é gravado aqui. O
// analista confere campo a campo antes de registrar a ocorrência.
func (a *app) handleIncidentParseReport(w http.ResponseWriter, r *http.Request) {
	// Mesma permissão do cadastro: quem não pode registrar ocorrência não
	// tem por que extrair campos para uma.
	if !a.requirePerm(w, r, "incident.create") {
		return
	}
	me := middleware.UserFrom(r.Context())
	var req parseReportRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		httpx.Error(w, http.StatusBadRequest, "texto vazio")
		return
	}
	if len([]rune(text)) > maxReportChars {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "texto muito longo (máx. 50.000 caracteres)")
		return
	}

	p := incidents.ParseReport(text)

	// Coordenadas: o link do relatório é encurtado e não as carrega, então
	// só saem seguindo o redirect. Falha vira aviso — o cadastro segue sem o
	// ponto no mapa, que o analista pode colar depois.
	resolved := false
	if p.Latitude == nil && p.MapsURL != "" && (req.ResolveLink == nil || *req.ResolveLink) {
		point, err := incidents.ResolveMapsLink(r.Context(), p.MapsURL)
		switch {
		case err == nil && point.HasCoords:
			lat, lng := point.Latitude, point.Longitude
			p.Latitude, p.Longitude = &lat, &lng
			resolved = true
		case point.Address != "":
			// O link resolveu, mas para uma busca por endereço — comum quando
			// quem compartilhou usou a barra de pesquisa em vez do marcador.
			w := fmt.Sprintf("O link do Maps não aponta um ponto, e sim a busca por %q — "+
				"informe as coordenadas manualmente.", point.Address)
			if c := incidents.FindCityMention(point.Address); c != "" && p.City != "" && c != p.City {
				w += fmt.Sprintf(" Atenção: esse endereço fica em %s, e o relatório informou %s.", c, p.City)
			}
			p.Warnings = append(p.Warnings, w)
		default:
			log.Printf("incidents parse: resolver maps: %v", err)
			p.Warnings = append(p.Warnings,
				"Não foi possível obter as coordenadas do link do Google Maps — abra o link e informe o ponto manualmente.")
		}
	}

	out := parsedReportJSON{
		Type: p.Type, Means: p.Means, MeansDetail: p.MeansDetail,
		OccurredOn: p.OccurredOn, OccurredTime: p.OccurredTime,
		CIOPSRecord: p.CIOPSRecord,
		City: p.City, Neighborhood: p.Neighborhood,
		Description: p.Description, MapsURL: p.MapsURL,
		Latitude: p.Latitude, Longitude: p.Longitude,
		People:   make([]parsedPersonJSON, 0, len(p.People)),
		Warnings: p.Warnings,
	}
	if out.Warnings == nil {
		out.Warnings = []string{}
	}
	// Candidatos de dossiê por pessoa citada: é o passo que o analista faria
	// à mão, uma busca por vez, e o que evita cadastrar em duplicidade
	// alguém que já tem ficha.
	canSearch := a.hasPerm(r, "entity.list")
	for _, person := range p.People {
		item := parsedPersonJSON{
			Name: person.Name, MotherName: person.MotherName,
			DateOfBirth: person.DateOfBirth, CPF: person.CPF, Alias: person.Alias,
			Role: person.Role, Status: person.Status, Notes: person.Notes,
			Matches: []personDuplicateJSON{},
		}
		if canSearch {
			res, err := a.entities.FindPersonDuplicates(r.Context(), entities.DuplicatesQuery{
				Name:         person.Name,
				MotherName:   person.MotherName,
				DateOfBirth:  person.DateOfBirth,
				CPF:          person.CPF,
				MaxClearance: me.ClearanceLevel,
			})
			if err != nil {
				log.Printf("incidents parse: duplicates: %v", err)
			} else {
				// CPF é identificador: quem já o detém encabeça a lista, mesmo
				// que o nome esteja grafado de outro jeito no dossiê.
				if res.CPFTakenBy != nil {
					cpfMatch := *toDuplicateJSON(res.CPFTakenBy)
					// O repo pontua homônimos por nome/mãe/nascimento; quem casa
					// por CPF não passa por lá, então o critério é rotulado aqui.
					cpfMatch.MatchedFields = []string{"cpf"}
					cpfMatch.Score = 3
					item.Matches = append(item.Matches, cpfMatch)
				}
				for i := range res.Matches {
					d := toDuplicateJSON(&res.Matches[i])
					if res.CPFTakenBy != nil && d.ID == res.CPFTakenBy.ID {
						continue
					}
					item.Matches = append(item.Matches, *d)
				}
			}
		}
		out.People = append(out.People, item)
	}

	// O parse não altera estado, mas a resolução do link é uma requisição a
	// terceiro partindo da agência — isso fica registrado. O texto do
	// relatório NÃO entra no log.
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "incident.parse_report",
		ResourceType: audit.Ptr("incident"),
		After: map[string]any{
			"maps_lookup": resolved,
			"people":      len(out.People),
			"chars":       len([]rune(text)),
		},
	})
	httpx.OK(w, map[string]any{"parsed": out})
}
