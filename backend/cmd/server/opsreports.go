package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/occurrences"
	"github.com/belia/tevunah/backend/internal/opsreport"
)

// Relatório Operacional: o PDF diário do CPRAIO traz as ocorrências de todos
// os batalhões; só as do batalhão da agência (OPS_REPORT_UNIT, padrão
// "2º BPRAIO") entram no acervo. O recorte é pelo BPM da linha "Base:", então
// companhia ou pelotão novos entram sem cadastro prévio.

const (
	opsReportMaxBytes  = 20 << 20 // 20 MiB — o relatório diário tem < 1 MiB
	opsReportFieldName = "file"
	opsReportsSubdir   = "ops-reports"
)

// ─────────────────────────── JSON ────────────────────────────

type opsPersonJSON struct {
	ID         string  `json:"id,omitempty"`
	Role       string  `json:"role"`
	Name       string  `json:"name"`
	MotherName string  `json:"mother_name"`
	Age        *int    `json:"age"`
	Address    string  `json:"address"`
	Note       string  `json:"note"`
	EntityID   *string `json:"entity_id"`
	EntityName string  `json:"entity_name,omitempty"`
	// EntityDeceased avisa a tela quando o dossiê vinculado está em óbito.
	EntityDeceased bool   `json:"entity_deceased,omitempty"`
	LinkMode       string `json:"link_mode"`
	// Matches: dossiês candidatos (homônimos), só para quem não está
	// vinculado. LinkWarning explica por que não houve vínculo automático.
	Matches     []personDuplicateJSON `json:"matches"`
	LinkWarning string                `json:"link_warning,omitempty"`
}

type opsWeaponJSON struct {
	Kind    string `json:"kind"`
	Model   string `json:"model"`
	Brand   string `json:"brand"`
	Caliber string `json:"caliber"`
	Serial  string `json:"serial"`
}

type opsDrugJSON struct {
	Description string   `json:"description"`
	Grams       *float64 `json:"grams"`
	Packages    *int     `json:"packages"`
}

type opsVehicleJSON struct {
	Kind  string `json:"kind"`
	Brand string `json:"brand"`
	Model string `json:"model"`
	Plate string `json:"plate"`
	Color string `json:"color"`
}

type opsOfficerJSON struct {
	Registration string `json:"registration"`
	Rank         string `json:"rank"`
	Number       string `json:"number"`
	WarName      string `json:"war_name"`
	// Composição no SIPOM: equipe, tipo de policiamento e função.
	SipomEquipe       string `json:"sipom_equipe"`
	SipomPoliciamento string `json:"sipom_policiamento"`
	SipomFuncao       string `json:"sipom_funcao"`
}

type opsOccurrenceJSON struct {
	ID       string `json:"id,omitempty"`
	ReportID string `json:"report_id,omitempty"`
	Page     int    `json:"page"`

	Natures  []string `json:"natures"`
	BaseRaw  string   `json:"base_raw"`
	BaseCity string   `json:"base_city"`
	CIA      string   `json:"cia"`
	PEL      string   `json:"pel"`
	BPM      string   `json:"bpm"`

	OccurredOn string `json:"occurred_on"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	Teams      string `json:"teams"`
	CIOPS      string `json:"ciops_record"`

	PlaceAddress         string `json:"place_address"`
	PlaceNeighborhood    string `json:"place_neighborhood"`
	PlaceCity            string `json:"place_city"`
	ApproachAddress      string `json:"approach_address"`
	ApproachNeighborhood string `json:"approach_neighborhood"`
	ApproachCity         string `json:"approach_city"`

	PoliceStation   string `json:"police_station"`
	Delegate        string `json:"delegate"`
	ProcedureType   string `json:"procedure_type"`
	ProcedureNumber string `json:"procedure_number"`

	SeizedObjects string `json:"seized_objects"`
	Narrative     string `json:"narrative"`

	// Coordenada do local do fato: null = não localizada. geo_precision vale
	// para o ponto automático ("porta" | "rua" | "bairro" — este aproximado);
	// geo_source diz quem o definiu ("auto" | "manual").
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
	GeoPrecision string   `json:"geo_precision"`
	GeoSource    string   `json:"geo_source"`

	People   []opsPersonJSON  `json:"people"`
	Weapons  []opsWeaponJSON  `json:"weapons"`
	Drugs    []opsDrugJSON    `json:"drugs"`
	Vehicles []opsVehicleJSON `json:"vehicles"`
	Officers []opsOfficerJSON `json:"officers"`

	// Participação da inteligência. Na prévia vem só da regra (modo "auto");
	// gravada, pode ter sido decidida pelo analista ("manual").
	IntelParticipation bool     `json:"intel_participation"`
	IntelMode          string   `json:"intel_mode"`
	IntelMatched       []string `json:"intel_matched"`

	// Tradução para o SIPOM (destino do envio); nil sem catálogo.
	Sipom *sipomOccurrenceJSON `json:"sipom"`

	// Fotos anexadas pelo analista (só na ficha gravada); o binário sai por
	// GET /api/ops-occurrences/{id}/photos/{pid}.
	Photos []opsPhotoJSON `json:"photos"`

	// Só na prévia: "new" (será gravada) ou "duplicate" (ficha CIOPS já no
	// acervo), e o id existente no segundo caso.
	Status     string   `json:"status,omitempty"`
	ExistingID string   `json:"existing_id,omitempty"`
	Warnings   []string `json:"warnings"`
	// Só na prévia, contra o cadastro manual de Ocorrências. LinkedIncidentID:
	// cadastro com a mesma ficha — é a mesma ocorrência, e a listagem as une.
	// PossibleDuplicates: cadastros de ficha DIFERENTE com a mesma data, hora
	// e lugar — sinal de ficha digitada errado no cadastro.
	LinkedIncidentID   string                   `json:"linked_incident_id,omitempty"`
	PossibleDuplicates []duplicateCandidateJSON `json:"possible_duplicates,omitempty"`

	CreatedAt *time.Time `json:"created_at,omitempty"`
	// Última correção do analista; ausentes = a ficha é a cópia do PDF.
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
	UpdatedByName string     `json:"updated_by_name,omitempty"`
}

// Resumo para a listagem.
type opsOccurrenceRowJSON struct {
	ID            string   `json:"id"`
	ReportID      string   `json:"report_id"`
	Natures       []string `json:"natures"`
	BaseCity      string   `json:"base_city"`
	CIA           string   `json:"cia"`
	PEL           string   `json:"pel"`
	OccurredOn    string   `json:"occurred_on"`
	StartTime     string   `json:"start_time"`
	EndTime       string   `json:"end_time"`
	Teams         string   `json:"teams"`
	CIOPS         string   `json:"ciops_record"`
	Neighborhood  string   `json:"place_neighborhood"`
	City          string   `json:"place_city"`
	ProcedureType string   `json:"procedure_type"`
	AccusedNames  []string `json:"accused_names"`
	PeopleCount   int      `json:"people_count"`
	WeaponCount   int      `json:"weapon_count"`
	DrugCount     int      `json:"drug_count"`
	VehicleCount  int      `json:"vehicle_count"`
	Intel         bool     `json:"intel_participation"`
}

type opsReportJSON struct {
	ID                  string    `json:"id"`
	ReportDate          *string   `json:"report_date"`
	FileName            string    `json:"file_name"`
	FileSize            int       `json:"file_size"`
	TotalOccurrences    int       `json:"total_occurrences"`
	UnitOccurrences     int       `json:"unit_occurrences"`
	ImportedOccurrences int       `json:"imported_occurrences"`
	SkippedOccurrences  int       `json:"skipped_occurrences"`
	CreatedAt           time.Time `json:"created_at"`
	CreatedBy           string    `json:"created_by"`
	CreatedByName       string    `json:"created_by_name"`
}

func toOpsReportJSON(r *opsreport.ImportedReport) opsReportJSON {
	out := opsReportJSON{
		ID: r.ID, FileName: r.FileName, FileSize: r.FileSize,
		TotalOccurrences: r.TotalOccurrences, UnitOccurrences: r.UnitOccurrences,
		ImportedOccurrences: r.ImportedOccurrences, SkippedOccurrences: r.SkippedOccurrences,
		CreatedAt: r.CreatedAt, CreatedBy: r.CreatedBy, CreatedByName: r.CreatedByName,
	}
	if r.ReportDate != nil {
		s := r.ReportDate.Format("2006-01-02")
		out.ReportDate = &s
	}
	return out
}

func (a *app) toOpsOccurrenceJSON(o *opsreport.Occurrence) opsOccurrenceJSON {
	out := opsOccurrenceJSON{
		Page: o.Page, Natures: nonNil(o.Natures),
		BaseRaw: o.BaseRaw, BaseCity: o.BaseCity, CIA: o.CIA, PEL: o.PEL, BPM: o.Unit(),
		StartTime: o.StartTime, EndTime: o.EndTime, Teams: o.Teams, CIOPS: o.CIOPS,
		PlaceAddress: o.PlaceAddress, PlaceNeighborhood: o.PlaceNeighborhood, PlaceCity: o.PlaceCity,
		ApproachAddress: o.ApproachAddress, ApproachNeighborhood: o.ApproachNeighborhood, ApproachCity: o.ApproachCity,
		PoliceStation: o.PoliceStation, Delegate: o.Delegate,
		ProcedureType: o.ProcedureType, ProcedureNumber: o.ProcedureNumber,
		SeizedObjects: o.SeizedObjects, Narrative: o.Narrative,
		Latitude: o.Geo.Lat, Longitude: o.Geo.Lng,
		GeoPrecision: o.Geo.Precision, GeoSource: o.Geo.Source,
		People:   []opsPersonJSON{},
		Weapons:  make([]opsWeaponJSON, 0, len(o.Weapons)),
		Drugs:    make([]opsDrugJSON, 0, len(o.Drugs)),
		Vehicles: make([]opsVehicleJSON, 0, len(o.Vehicles)),
		Officers: make([]opsOfficerJSON, 0, len(o.Officers)),
		Warnings: nonNil(o.Warnings),
		Photos:   []opsPhotoJSON{},

		IntelParticipation: len(o.IntelMatched) > 0,
		IntelMode:          opsreport.IntelAuto,
		IntelMatched:       nonNil(o.IntelMatched),
	}
	if !o.OccurredOn.IsZero() {
		out.OccurredOn = o.OccurredOn.Format("2006-01-02")
	}
	for _, p := range o.People {
		out.People = append(out.People, opsPersonJSON{
			Role: p.Role, Name: p.Name, MotherName: p.MotherName, Age: p.Age,
			Address: p.Address, Note: p.Note, Matches: []personDuplicateJSON{},
		})
	}
	for _, w := range o.Weapons {
		out.Weapons = append(out.Weapons, opsWeaponJSON{Kind: w.Kind, Model: w.Model, Brand: w.Brand, Caliber: w.Caliber, Serial: w.Serial})
	}
	for _, d := range o.Drugs {
		out.Drugs = append(out.Drugs, opsDrugJSON{Description: d.Description, Grams: d.Grams, Packages: d.Packages})
	}
	for _, v := range o.Vehicles {
		out.Vehicles = append(out.Vehicles, opsVehicleJSON{Kind: v.Kind, Brand: v.Brand, Model: v.Model, Plate: v.Plate, Color: v.Color})
	}
	for _, f := range o.Officers {
		pol, fn := a.sipomOfficerLabels(f)
		out.Officers = append(out.Officers, opsOfficerJSON{
			Registration: f.Registration, Rank: f.Rank, Number: f.Number, WarName: f.WarName,
			SipomEquipe: f.SipomEquipe, SipomPoliciamento: pol, SipomFuncao: fn,
		})
	}
	out.Sipom = a.sipomJSON(o, opsreport.SipomPeopleParsed(o))
	return out
}

func nonNil(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}

// ─────────────────────────── Upload + parse ────────────────────────────

type parsedUpload struct {
	sha      string
	name     string
	data     []byte
	report   *opsreport.Report
	mine     []opsreport.Occurrence
	byUnit   map[string]int
	existing map[string]string // ficha CIOPS → id já gravado
}

// readOpsUpload lê o PDF do multipart, extrai e recorta o batalhão. Em erro,
// já respondeu.
func (a *app) readOpsUpload(w http.ResponseWriter, r *http.Request) (*parsedUpload, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, opsReportMaxBytes+1<<20)
	if err := r.ParseMultipartForm(opsReportMaxBytes); err != nil {
		httpx.Error(w, http.StatusBadRequest, "upload inválido ou maior que 20 MiB")
		return nil, false
	}
	file, header, err := r.FormFile(opsReportFieldName)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "arquivo ausente (campo 'file')")
		return nil, false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, opsReportMaxBytes+1))
	if err != nil || len(data) > opsReportMaxBytes {
		httpx.Error(w, http.StatusBadRequest, "arquivo maior que 20 MiB")
		return nil, false
	}
	if http.DetectContentType(data) != "application/pdf" {
		httpx.Error(w, http.StatusBadRequest, "envie o relatório em PDF")
		return nil, false
	}

	pages, err := opsreport.Extract(r.Context(), data)
	if err != nil {
		if errors.Is(err, opsreport.ErrNoText) {
			httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
			return nil, false
		}
		log.Printf("opsreport extract: %v", err)
		httpx.Error(w, http.StatusUnprocessableEntity, "não foi possível ler o PDF")
		return nil, false
	}
	rep := opsreport.Parse(pages)
	if len(rep.Occurrences) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity,
			"nenhuma ocorrência encontrada — confira se é o Relatório Diário de Ocorrências")
		return nil, false
	}

	sum := sha256.Sum256(data)
	up := &parsedUpload{
		sha: hex.EncodeToString(sum[:]), name: filepath.Base(header.Filename),
		data: data, report: rep, mine: rep.ForUnit(a.opsUnit), byUnit: map[string]int{},
	}
	for _, o := range rep.Occurrences {
		up.byUnit[o.Unit()]++
	}
	fichas := make([]string, 0, len(up.mine))
	for _, o := range up.mine {
		if o.CIOPS != "" {
			fichas = append(fichas, o.CIOPS)
		}
	}
	up.existing, err = a.opsReports.ExistingCIOPS(r.Context(), fichas)
	if err != nil {
		log.Printf("opsreport existing: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao consultar o acervo")
		return nil, false
	}

	// Participação da inteligência: termos configurados contra histórico e
	// equipe. Vale para a prévia e para a gravação, que relê o PDF.
	matcher, err := a.intel.Matcher(r.Context())
	if err != nil {
		log.Printf("opsreport intel keywords: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao carregar os termos de inteligência")
		return nil, false
	}
	// Tradução para o SIPOM: a natureza sai do de-para; sem ele, a ocorrência
	// entra pendente de natureza e o recálculo resolve depois.
	nm, err := a.sipomMap.Load(r.Context())
	if err != nil {
		log.Printf("opsreport sipom map: %v", err)
	}
	// Referência de área aprendida (cidade + bairro): sem ela, a área sai só
	// do catálogo e o que ele não resolve entra pendente.
	areas, err := a.sipomAreas.Load(r.Context())
	if err != nil {
		log.Printf("opsreport sipom area rules: %v", err)
	}
	terms, err := a.sipomTerms.Load(r.Context())
	if err != nil {
		log.Printf("opsreport sipom terms: %v", err)
	}
	// Coordenada das ocorrências novas, pelo geocodificador da agência. Tem
	// prazo: a leitura do PDF não pode ficar presa a ele. O que não for
	// localizado (ou não der tempo) entra com a pendência de coordenada e pode
	// ser refeito depois, na ficha ou em lote (./tevunah geo:backfill).
	if a.geocoder.Enabled() {
		gctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		for i := range up.mine {
			if _, dup := up.existing[up.mine[i].CIOPS]; dup {
				continue
			}
			geo, err := a.locate(gctx, &up.mine[i])
			if err != nil {
				log.Printf("opsreport geocode: %v", err)
				up.report.Warnings = append(up.report.Warnings,
					"Geocodificador indisponível — parte das ocorrências entra sem coordenada, como pendência.")
				break
			}
			up.mine[i].Geo = geo
		}
	}
	refs := opsreport.SipomRefs{Natures: nm, Areas: areas, GeoRequired: a.geocoder.Enabled(), Terms: terms}
	for i := range up.mine {
		up.mine[i].IntelMatched = matcher.Match(up.mine[i].IntelTexts()...)
		opsreport.TranslateSipom(a.sipom, refs, &up.mine[i])
	}
	return up, true
}

// ─────────────────────────── Vínculo automático ────────────────────────────

// personCandidates busca dossiês para a pessoa e decide o vínculo automático.
//
// Regra: vincula sozinho só quando nome E mãe batem (normalizados) com
// exatamente UM dossiê visível no clearance de quem importa, e esse dossiê
// não está em óbito (a menos que o próprio relatório anote o óbito). Qualquer
// outro caso volta como sugestão para o analista decidir. Sem nome da mãe —
// vítimas e testemunhas nunca o trazem — não há vínculo automático.
func (a *app) personCandidates(ctx context.Context, p opsreport.Person, clearance int) (auto string, matches []personDuplicateJSON, warning string, err error) {
	matches = []personDuplicateJSON{}
	res, err := a.entities.FindPersonDuplicates(ctx, entities.DuplicatesQuery{
		Name: p.Name, MotherName: p.MotherName, MaxClearance: clearance,
	})
	if err != nil {
		return "", matches, "", err
	}
	var strong []string
	for i := range res.Matches {
		d := res.Matches[i]
		matches = append(matches, *toDuplicateJSON(&d))
		for _, f := range d.MatchedFields {
			if f == "mother_name" {
				strong = append(strong, d.ID)
			}
		}
	}
	switch {
	case len(matches) == 0:
		return "", matches, "", nil
	case p.MotherName == "":
		return "", matches, "Sem nome da mãe no relatório — confira o homônimo antes de vincular.", nil
	case len(strong) == 0:
		return "", matches, "Homônimo com mãe diferente — não vinculado.", nil
	case len(strong) > 1:
		return "", matches, fmt.Sprintf("%d dossiês com o mesmo nome e mãe — escolha o correto.", len(strong)), nil
	}
	dead, err := a.opsReports.PersonDeceased(ctx, strong[0])
	if err != nil {
		return "", matches, "", err
	}
	if dead && !strings.Contains(strings.ToUpper(p.Note), "ÓBITO") {
		return "", matches, "O dossiê que casa está marcado como óbito — confira antes de vincular.", nil
	}
	return strong[0], matches, "", nil
}

// ─────────────────────────── POST /api/ops-reports/preview ────────────────

// Lê o PDF e devolve o que seria gravado — nada é persistido.
func (a *app) handleOpsReportPreview(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.import") {
		return
	}
	me := middleware.UserFrom(r.Context())
	up, ok := a.readOpsUpload(w, r)
	if !ok {
		return
	}
	prev, err := a.opsReports.FindReportBySHA(r.Context(), up.sha)
	if err != nil {
		log.Printf("opsreport sha: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao consultar o acervo")
		return
	}

	// Cruzamento com o cadastro manual de Ocorrências, só para quem o lê.
	canSeeIncidents := a.hasPerm(r, "incident.read")
	linked := map[string]string{}
	if canSeeIncidents {
		fichas := make([]string, 0, len(up.mine))
		for _, o := range up.mine {
			fichas = append(fichas, o.CIOPS)
		}
		if linked, err = a.occurrences.IncidentsByCIOPS(r.Context(), fichas); err != nil {
			log.Printf("opsreport incidents by ficha: %v", err)
			linked = map[string]string{}
		}
	}

	canSearch := a.hasPerm(r, "entity.list")
	occs := make([]opsOccurrenceJSON, 0, len(up.mine))
	newCount := 0
	for i := range up.mine {
		o := &up.mine[i]
		j := a.toOpsOccurrenceJSON(o)
		j.Status = "new"
		if id, dup := up.existing[o.CIOPS]; dup {
			j.Status, j.ExistingID = "duplicate", id
		} else {
			newCount++
		}
		if canSeeIncidents && j.Status == "new" {
			if id, ok := linked[occurrences.NormCIOPS(o.CIOPS)]; ok {
				j.LinkedIncidentID = id
			} else {
				dups, err := a.occurrences.PossibleDuplicates(r.Context(), occurrences.DuplicateQuery{
					Subject: occurrences.Subject{
						CIOPS: o.CIOPS, OccurredOn: o.OccurredOn, Time: o.StartTime,
						City: o.PlaceCity, Neighborhood: o.PlaceNeighborhood,
					},
					Scope: occurrences.Scope{Manual: true},
				})
				if err != nil {
					log.Printf("opsreport possible duplicates: %v", err)
				} else if len(dups) > 0 {
					j.PossibleDuplicates = toDuplicateCandidatesJSON(dups)
				}
			}
		}
		if canSearch && j.Status == "new" {
			for k, p := range o.People {
				auto, matches, warn, err := a.personCandidates(r.Context(), p, me.ClearanceLevel)
				if err != nil {
					log.Printf("opsreport candidates: %v", err)
					continue
				}
				j.People[k].Matches, j.People[k].LinkWarning = matches, warn
				if auto != "" {
					j.People[k].EntityID, j.People[k].LinkMode = &auto, opsreport.LinkAuto
					for _, m := range matches {
						if m.ID == auto {
							j.People[k].EntityName = m.Name
						}
					}
				}
			}
		}
		occs = append(occs, j)
	}

	out := map[string]any{
		"unit":             a.opsUnit,
		"file_name":        up.name,
		"total":            len(up.report.Occurrences),
		"unit_total":       len(up.mine),
		"new_total":        newCount,
		"by_unit":          up.byUnit,
		"occurrences":      occs,
		"warnings":         nonNil(up.report.Warnings),
		"report_date":      nil,
		"already_imported": nil,
	}
	if !up.report.ReportDate.IsZero() {
		out["report_date"] = up.report.ReportDate.Format("2006-01-02")
	}
	if prev != nil {
		out["already_imported"] = toOpsReportJSON(prev)
	}
	httpx.OK(w, out)
}

// ─────────────────────────── POST /api/ops-reports ────────────────────────

// Importa o PDF: relê o arquivo no servidor (a prévia não é confiável como
// entrada), grava as ocorrências novas do batalhão e aplica os vínculos
// automáticos.
func (a *app) handleOpsReportImport(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.import") {
		return
	}
	me := middleware.UserFrom(r.Context())
	up, ok := a.readOpsUpload(w, r)
	if !ok {
		return
	}
	if len(up.mine) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("o relatório não traz ocorrências do %s", a.opsUnit))
		return
	}

	var links []opsreport.PersonLink
	if a.hasPerm(r, "entity.list") {
		for i, o := range up.mine {
			if _, dup := up.existing[o.CIOPS]; dup {
				continue
			}
			for k, p := range o.People {
				auto, _, _, err := a.personCandidates(r.Context(), p, me.ClearanceLevel)
				if err != nil {
					log.Printf("opsreport candidates: %v", err)
					continue
				}
				if auto != "" {
					links = append(links, opsreport.PersonLink{Occ: i, Person: k, EntityID: auto})
				}
			}
		}
	}

	// O PDF original fica guardado para conferência. Nome = hash: o mesmo
	// arquivo nunca é gravado duas vezes.
	dir := filepath.Join(photoDir(), opsReportsSubdir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Printf("mkdir %s: %v", dir, err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao preparar storage")
		return
	}
	dst := filepath.Join(dir, up.sha+".pdf")
	if err := os.WriteFile(dst, up.data, 0o640); err != nil {
		log.Printf("gravar pdf: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao guardar o PDF")
		return
	}

	res, err := a.opsReports.Import(r.Context(), opsreport.ImportInput{
		FileSHA256: up.sha, FileName: up.name, FileSize: len(up.data),
		ReportDate: up.report.ReportDate, Total: len(up.report.Occurrences),
		Occurrences: up.mine, Links: links, CreatedBy: me.ID,
	})
	if errors.Is(err, opsreport.ErrAlreadyImported) {
		httpx.Error(w, http.StatusConflict, "este PDF já foi importado")
		return
	}
	if err != nil {
		_ = os.Remove(dst)
		log.Printf("opsreport import: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao importar")
		return
	}

	ids := make([]string, 0, len(res.Imported))
	intelMarked := 0
	for i := range up.mine {
		if id, ok := res.Imported[i]; ok {
			ids = append(ids, id)
			if len(up.mine[i].IntelMatched) > 0 {
				intelMarked++
			}
		}
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "opsreport.import",
		ResourceType: audit.Ptr("ops_report"),
		ResourceID:   &res.ReportID,
		After: map[string]any{
			"file_sha256":    up.sha,
			"file_name":      up.name,
			"report_date":    up.report.ReportDate.Format("2006-01-02"),
			"total":          len(up.report.Occurrences),
			"unit_total":     len(up.mine),
			"imported":       len(res.Imported),
			"skipped_ciops":  res.Skipped,
			"auto_linked":    res.AutoLinked,
			"intel_marked":   intelMarked,
			"occurrence_ids": ids,
		},
	})

	// Os vínculos automáticos só existem depois de gravar: refaz a tradução
	// das ocorrências novas para o aviso de pessoa sem dossiê sair certo.
	if a.sipom != nil {
		for _, id := range ids {
			if _, _, err := a.opsReports.RecomputeSipom(r.Context(), a.sipom, id); err != nil {
				log.Printf("opsreport sipom after import: %v", err)
			}
		}
	}

	rep, err := a.opsReports.FindReport(r.Context(), res.ReportID)
	if err != nil {
		log.Printf("opsreport find: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "importado, mas houve erro ao ler o resultado")
		return
	}
	httpx.Created(w, map[string]any{
		"report":         toOpsReportJSON(rep),
		"occurrence_ids": ids,
		"skipped_ciops":  nonNil(res.Skipped),
		"auto_linked":    res.AutoLinked,
	})
}

// ─────────────────────────── GET /api/ops-reports ────────────────────────

func (a *app) handleOpsReportsList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	items, total, err := a.opsReports.ListReports(r.Context(), limit, offset)
	if err != nil {
		log.Printf("opsreport list reports: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	out := make([]opsReportJSON, 0, len(items))
	for i := range items {
		out = append(out, toOpsReportJSON(&items[i]))
	}
	httpx.OK(w, map[string]any{"items": out, "total": total})
}

// ─────────────────────────── GET /api/ops-reports/{id}/file ────────────────

func (a *app) handleOpsReportFile(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	rep, err := a.opsReports.FindReport(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, opsreport.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
			return
		}
		log.Printf("opsreport file: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	f, err := os.Open(filepath.Join(photoDir(), opsReportsSubdir, rep.FileSHA256+".pdf"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "arquivo não encontrado")
		return
	}
	defer f.Close()

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "opsreport.file.download",
		ResourceType: audit.Ptr("ops_report"),
		ResourceID:   &rep.ID,
	})
	name := rep.FileName
	if name == "" {
		name = "relatorio-operacional.pdf"
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", name))
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "", rep.CreatedAt, f)
}

// ─────────────────────────── GET /api/ops-occurrences ────────────────────

func (a *app) handleOpsOccurrencesList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	items, total, err := a.opsReports.List(r.Context(), opsreport.ListOpts{
		Limit: limit, Offset: offset,
		Search: q.Get("search"), CIA: q.Get("cia"), PEL: q.Get("pel"),
		Nature: q.Get("nature"), City: q.Get("city"), ReportID: q.Get("report_id"),
		DateFrom: q.Get("date_from"), DateTo: q.Get("date_to"), Intel: q.Get("intel"),
		SortBy: q.Get("sort_by"), SortDir: q.Get("sort_dir"),
	})
	if err != nil {
		log.Printf("opsreport list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	out := make([]opsOccurrenceRowJSON, 0, len(items))
	for _, it := range items {
		out = append(out, opsOccurrenceRowJSON{
			ID: it.ID, ReportID: it.ReportID, Natures: nonNil(it.Natures),
			BaseCity: it.BaseCity, CIA: it.CIA, PEL: it.PEL,
			OccurredOn: it.OccurredOn.Format("2006-01-02"),
			StartTime:  it.StartTime, EndTime: it.EndTime, Teams: it.Teams, CIOPS: it.CIOPS,
			Neighborhood: it.PlaceNeighborhood, City: it.PlaceCity, ProcedureType: it.ProcedureType,
			AccusedNames: nonNil(it.AccusedNames),
			PeopleCount:  it.PeopleCount, WeaponCount: it.WeaponCount,
			DrugCount: it.DrugCount, VehicleCount: it.VehicleCount,
			Intel: it.IntelParticipation,
		})
	}
	httpx.OK(w, map[string]any{"items": out, "total": total})
}

func (a *app) handleOpsOccurrencesFacets(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	f, err := a.opsReports.ListFacets(r.Context())
	if err != nil {
		log.Printf("opsreport facets: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	httpx.OK(w, map[string]any{
		"cias": f.CIAs, "pels": f.PELs, "cities": f.Cities, "natures": f.Natures,
	})
}

// ─────────────────────────── GET /api/ops-occurrences/{id} ────────────────

func (a *app) handleOpsOccurrenceDetail(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.read") {
		return
	}
	a.respondOpsOccurrence(w, r, r.PathValue("id"))
}

// respondOpsOccurrence escreve a ficha completa da ocorrência — o detalhe, e
// a resposta da edição.
func (a *app) respondOpsOccurrence(w http.ResponseWriter, r *http.Request, id string) {
	me := middleware.UserFrom(r.Context())
	so, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, opsreport.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
			return
		}
		log.Printf("opsreport detail: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	out := a.toOpsOccurrenceJSON(&so.Occurrence)
	out.ID, out.ReportID, out.CreatedAt = so.ID, so.ReportID, &so.CreatedAt
	out.UpdatedAt, out.UpdatedByName = so.UpdatedAt, so.UpdatedByName
	out.IntelParticipation, out.IntelMode = so.IntelParticipation, so.IntelMode
	out.Warnings = []string{}
	out.People = make([]opsPersonJSON, 0, len(so.People))
	canSearch := a.hasPerm(r, "entity.list")
	for _, sp := range so.People {
		pj := opsPersonJSON{
			ID: sp.ID, Role: sp.Role, Name: sp.Name, MotherName: sp.MotherName, Age: sp.Age,
			Address: sp.Address, Note: sp.Note, EntityID: sp.EntityID,
			EntityName: sp.EntityName, EntityDeceased: sp.EntityDeceased, LinkMode: sp.LinkMode,
			Matches: []personDuplicateJSON{},
		}
		// Sugestões só para quem ainda não tem dossiê: o acervo cresce depois
		// da importação, então o candidato de hoje pode não existir ontem.
		if sp.EntityID == nil && canSearch {
			_, matches, warn, err := a.personCandidates(r.Context(), sp.Person, me.ClearanceLevel)
			if err != nil {
				log.Printf("opsreport candidates: %v", err)
			} else {
				pj.Matches, pj.LinkWarning = matches, warn
			}
		}
		out.People = append(out.People, pj)
	}
	out.Sipom = a.sipomJSON(&so.Occurrence, opsreport.SipomPeople(so))
	a.markAreaLearned(r.Context(), &so.Occurrence, out.Sipom)
	out.Photos = toOpsPhotosJSON(so.Photos)
	httpx.OK(w, map[string]any{"occurrence": out})
}

// ─────────────────────────── PATCH /api/ops-occurrences/{id} ──────────────

// opsOccurrenceEditRequest: só o que vier é alterado (campo ausente ou null
// fica como está). Hora "" limpa.
type opsOccurrenceEditRequest struct {
	Natures    *[]string `json:"natures"`
	OccurredOn *string   `json:"occurred_on"`
	StartTime  *string   `json:"start_time"`
	EndTime    *string   `json:"end_time"`
	Teams      *string   `json:"teams"`
	CIOPS      *string   `json:"ciops_record"`
	BaseCity   *string   `json:"base_city"`
	CIA        *string   `json:"cia"`
	PEL        *string   `json:"pel"`

	PlaceAddress         *string `json:"place_address"`
	PlaceNeighborhood    *string `json:"place_neighborhood"`
	PlaceCity            *string `json:"place_city"`
	ApproachAddress      *string `json:"approach_address"`
	ApproachNeighborhood *string `json:"approach_neighborhood"`
	ApproachCity         *string `json:"approach_city"`

	PoliceStation   *string `json:"police_station"`
	Delegate        *string `json:"delegate"`
	ProcedureType   *string `json:"procedure_type"`
	ProcedureNumber *string `json:"procedure_number"`

	SeizedObjects *string `json:"seized_objects"`
	Narrative     *string `json:"narrative"`
}

// opsEditSnapshot são os campos editáveis, para o antes/depois da auditoria.
func opsEditSnapshot(so *opsreport.StoredOccurrence) map[string]any {
	o := &so.Occurrence
	return map[string]any{
		"natures": nonNil(o.Natures), "occurred_on": o.OccurredOn.Format("2006-01-02"),
		"start_time": o.StartTime, "end_time": o.EndTime, "teams": o.Teams, "ciops_record": o.CIOPS,
		"base_city": o.BaseCity, "cia": o.CIA, "pel": o.PEL,
		"place_address": o.PlaceAddress, "place_neighborhood": o.PlaceNeighborhood, "place_city": o.PlaceCity,
		"approach_address": o.ApproachAddress, "approach_neighborhood": o.ApproachNeighborhood,
		"approach_city":  o.ApproachCity,
		"police_station": o.PoliceStation, "delegate": o.Delegate,
		"procedure_type": o.ProcedureType, "procedure_number": o.ProcedureNumber,
		"seized_objects": o.SeizedObjects, "narrative": o.Narrative,
		"latitude": fmtCoord(o.Geo.Lat), "longitude": fmtCoord(o.Geo.Lng), "geo_precision": o.Geo.Precision,
	}
}

func fmtCoord(p *float64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatFloat(*p, 'f', 6, 64)
}

// Correção da ocorrência importada pelo analista. O PDF original segue
// guardado; o que muda aqui vai para o trilho com o antes e o depois de cada
// campo alterado. A tradução para o SIPOM e a marcação automática de
// inteligência são refeitas, porque dependem dos campos corrigidos.
func (a *app) handleOpsOccurrenceUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	var req opsOccurrenceEditRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	edit := opsreport.Edit{
		Natures: req.Natures, Teams: req.Teams, CIOPS: req.CIOPS,
		BaseCity: req.BaseCity, CIA: req.CIA, PEL: req.PEL,
		PlaceAddress: req.PlaceAddress, PlaceNeighborhood: req.PlaceNeighborhood, PlaceCity: req.PlaceCity,
		ApproachAddress: req.ApproachAddress, ApproachNeighborhood: req.ApproachNeighborhood,
		ApproachCity:  req.ApproachCity,
		PoliceStation: req.PoliceStation, Delegate: req.Delegate,
		ProcedureType: req.ProcedureType, ProcedureNumber: req.ProcedureNumber,
		SeizedObjects: req.SeizedObjects, Narrative: req.Narrative,
	}
	if req.OccurredOn != nil {
		t, err := time.Parse("2006-01-02", strings.TrimSpace(*req.OccurredOn))
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "occurred_on inválido (esperado YYYY-MM-DD)")
			return
		}
		edit.OccurredOn = &t
	}
	for _, c := range []struct {
		in  *string
		out **string
		lbl string
	}{{req.StartTime, &edit.StartTime, "start_time"}, {req.EndTime, &edit.EndTime, "end_time"}} {
		if c.in == nil {
			continue
		}
		hhmm, ok := parseClock(*c.in)
		if !ok {
			httpx.Error(w, http.StatusBadRequest, c.lbl+" inválido (esperado HH:MM)")
			return
		}
		v := ""
		if hhmm != nil {
			v = *hhmm
		}
		*c.out = &v
	}

	before, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	// A ficha CIOPS identifica a ocorrência: não pode passar a ser a de
	// outra do relatório.
	if req.CIOPS != nil {
		other, err := a.occurrences.OpsByCIOPS(r.Context(), *req.CIOPS)
		if err != nil {
			log.Printf("opsreport update (ficha): %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao verificar a ficha CIOPS")
			return
		}
		if other != nil && other.ID != id {
			duplicateConflict(w, conflictCIOPSDuplicate,
				"ficha CIOPS já pertence a outra ocorrência do relatório", []occurrences.Candidate{*other})
			return
		}
	}

	if err := a.opsReports.Update(r.Context(), id, me.ID, edit); err != nil {
		switch {
		case errors.Is(err, opsreport.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		case errors.Is(err, opsreport.ErrDuplicateCIOPS):
			duplicateConflict(w, conflictCIOPSDuplicate,
				"ficha CIOPS já pertence a outra ocorrência do relatório", nil)
		default:
			log.Printf("opsreport update: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao gravar")
		}
		return
	}

	// Histórico ou equipe podem ter mudado: refaz a marcação automática de
	// inteligência (decisão manual do analista fica como está).
	if after, err := a.opsReports.FindByID(r.Context(), id); err == nil {
		// Endereço corrigido: a coordenada automática é refeita para o novo
		// endereço (a que o analista informou fica como está). Não localizou,
		// a ocorrência volta a ficar sem coordenada — pendente.
		was, now := opsreport.SipomInput(&before.Occurrence), opsreport.SipomInput(&after.Occurrence)
		moved := was.Address != now.Address || was.Neighborhood != now.Neighborhood || was.City != now.City
		if moved && a.geocoder.Enabled() && after.Geo.Source != opsreport.GeoManual {
			if geo, err := a.locate(r.Context(), &after.Occurrence); err != nil {
				log.Printf("opsreport update (geocode): %v", err)
			} else if err := a.opsReports.SetGeo(r.Context(), id, geo); err != nil {
				log.Printf("opsreport update (geo): %v", err)
			}
		}
		if matcher, err := a.intel.Matcher(r.Context()); err != nil {
			log.Printf("opsreport update (intel): %v", err)
		} else if err := a.opsReports.SetIntelMatched(r.Context(), id, matcher.Match(after.IntelTexts()...)); err != nil {
			log.Printf("opsreport update (intel): %v", err)
		}
	}
	a.sipomRecomputeOne(r.Context(), id)

	after, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "gravado, mas houve erro ao ler o resultado")
		return
	}
	// O trilho guarda só o que mudou, com o valor de antes e o de depois.
	was, now := opsEditSnapshot(before), opsEditSnapshot(after)
	changedBefore, changedAfter := map[string]any{}, map[string]any{}
	for k, v := range now {
		if fmt.Sprint(was[k]) != fmt.Sprint(v) {
			changedBefore[k], changedAfter[k] = was[k], v
		}
	}
	changedAfter["sipom_pendencias"] = nonNil(after.Sipom.Pending)
	changedBefore["sipom_pendencias"] = nonNil(before.Sipom.Pending)
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "opsreport.occurrence.update",
		ResourceType: audit.Ptr("ops_occurrence"),
		ResourceID:   &id,
		Before:       changedBefore,
		After:        changedAfter,
	})
	a.respondOpsOccurrence(w, r, id)
}

// ─────────── PUT/DELETE /api/ops-occurrences/{id}/people/{pid}/entity ───────

type opsLinkRequest struct {
	EntityID string `json:"entity_id"`
}

func (a *app) handleOpsPersonLink(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	occID, pid := r.PathValue("id"), r.PathValue("pid")
	var req opsLinkRequest
	if err := httpx.Decode(r, &req); err != nil || strings.TrimSpace(req.EntityID) == "" {
		httpx.Error(w, http.StatusBadRequest, "entity_id obrigatório")
		return
	}
	ent, err := a.entities.FindByID(r.Context(), strings.TrimSpace(req.EntityID))
	if err != nil || ent.DeletedAt != nil || ent.Classification > me.ClearanceLevel {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}
	if ent.Kind != entities.KindPerson {
		httpx.Error(w, http.StatusBadRequest, "só é possível vincular a um dossiê de pessoa")
		return
	}
	before, err := a.opsReports.FindPerson(r.Context(), occID, pid)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "pessoa não encontrada na ocorrência")
		return
	}
	if err := a.opsReports.LinkPerson(r.Context(), occID, pid, ent.ID, opsreport.LinkManual, me.ID); err != nil {
		log.Printf("opsreport link: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao vincular")
		return
	}
	a.auditOpsLink(r, "opsreport.person.link", occID, before, &ent.ID)
	a.sipomRecomputeOne(r.Context(), occID)
	httpx.OK(w, map[string]any{"entity_id": ent.ID, "entity_name": ent.Name, "link_mode": opsreport.LinkManual})
}

func (a *app) handleOpsPersonUnlink(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	occID, pid := r.PathValue("id"), r.PathValue("pid")
	before, err := a.opsReports.FindPerson(r.Context(), occID, pid)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "pessoa não encontrada na ocorrência")
		return
	}
	me := middleware.UserFrom(r.Context())
	if err := a.opsReports.LinkPerson(r.Context(), occID, pid, "", "", me.ID); err != nil {
		log.Printf("opsreport unlink: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao desvincular")
		return
	}
	a.auditOpsLink(r, "opsreport.person.unlink", occID, before, nil)
	a.sipomRecomputeOne(r.Context(), occID)
	httpx.NoContent(w)
}

func (a *app) auditOpsLink(r *http.Request, action, occID string, before *opsreport.StoredPerson, entityID *string) {
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       action,
		ResourceType: audit.Ptr("ops_occurrence"),
		ResourceID:   &occID,
		Before: map[string]any{
			"person_id": before.ID, "name": before.Name,
			"entity_id": before.EntityID, "link_mode": before.LinkMode,
		},
		After: map[string]any{"entity_id": entityID},
	})
}

// ─────────────────── PUT /api/ops-occurrences/{id}/intel ───────────────────

type opsIntelRequest struct {
	Value *bool `json:"value"`
}

// Marca ou desmarca a participação da inteligência. A decisão do analista
// vira modo manual e prevalece sobre os termos dali em diante.
func (a *app) handleOpsOccurrenceIntel(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	id := r.PathValue("id")
	var req opsIntelRequest
	if err := httpx.Decode(r, &req); err != nil || req.Value == nil {
		httpx.Error(w, http.StatusBadRequest, "value obrigatório (true|false)")
		return
	}
	before, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	if err := a.opsReports.SetIntel(r.Context(), id, *req.Value); err != nil {
		if errors.Is(err, opsreport.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
			return
		}
		log.Printf("opsreport intel: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "opsreport.intel.set",
		ResourceType: audit.Ptr("ops_occurrence"),
		ResourceID:   &id,
		Before: map[string]any{
			"intel_participation": before.IntelParticipation,
			"intel_mode":          before.IntelMode,
			"intel_matched":       nonNil(before.IntelMatched),
		},
		After: map[string]any{
			"intel_participation": *req.Value,
			"intel_mode":          opsreport.IntelManual,
		},
	})
	httpx.OK(w, map[string]any{
		"intel_participation": *req.Value,
		"intel_mode":          opsreport.IntelManual,
		"intel_matched":       nonNil(before.IntelMatched),
	})
}
