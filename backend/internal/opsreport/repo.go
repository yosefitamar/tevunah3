package opsreport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Link-mode do vínculo pessoa ↔ dossiê.
const (
	LinkAuto   = "auto"
	LinkManual = "manual"
)

var (
	ErrNotFound        = errors.New("ocorrência não encontrada")
	ErrPersonNotFound  = errors.New("pessoa não encontrada na ocorrência")
	ErrAlreadyImported = errors.New("este PDF já foi importado")
	// ErrDuplicateCIOPS: a ficha CIOPS já identifica outra ocorrência do
	// relatório (índice ops_occurrences_ciops_uniq).
	ErrDuplicateCIOPS = errors.New("ficha CIOPS já pertence a outra ocorrência")
)

// Repo encapsula app.ops_reports e app.ops_occurrence*.
type Repo struct {
	db *sql.DB
	// GeoRequired: o geocodificador está ligado. No recálculo da tradução,
	// ocorrência sem coordenada fica com a pendência "coordenada".
	GeoRequired bool
}

func New(db *sql.DB) *Repo { return &Repo{db: db} }

// ─────────────────────────── Importação ────────────────────────────

// ImportedReport é o registro de um PDF importado.
type ImportedReport struct {
	ID                  string
	ReportDate          *time.Time
	FileSHA256          string
	FileName            string
	FileSize            int
	TotalOccurrences    int
	UnitOccurrences     int
	ImportedOccurrences int
	SkippedOccurrences  int
	CreatedAt           time.Time
	CreatedBy           string
	CreatedByName       string
}

// FindReportBySHA devolve o relatório já importado com este hash (nil se não
// houver).
func (r *Repo) FindReportBySHA(ctx context.Context, sha string) (*ImportedReport, error) {
	rows, err := r.db.QueryContext(ctx, reportSelect+` WHERE r.file_sha256 = $1`, sha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	return scanReport(rows)
}

// FindReport devolve um relatório importado por id.
func (r *Repo) FindReport(ctx context.Context, id string) (*ImportedReport, error) {
	rows, err := r.db.QueryContext(ctx, reportSelect+` WHERE r.id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	return scanReport(rows)
}

// ExistingCIOPS devolve, das fichas informadas, as que já estão no acervo
// (ficha → id da ocorrência).
func (r *Repo) ExistingCIOPS(ctx context.Context, fichas []string) (map[string]string, error) {
	out := map[string]string{}
	if len(fichas) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT ciops_record, id FROM app.ops_occurrences
		 WHERE deleted_at IS NULL AND ciops_record = ANY($1)`, fichas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var f, id string
		if err := rows.Scan(&f, &id); err != nil {
			return nil, err
		}
		out[f] = id
	}
	return out, rows.Err()
}

// PersonLink é o vínculo automático decidido antes da gravação: pessoa
// Occurrences[Occ].People[Person] → EntityID.
type PersonLink struct {
	Occ, Person int
	EntityID    string
}

// ImportInput reúne o que a importação grava.
type ImportInput struct {
	FileSHA256  string
	FileName    string
	FileSize    int
	ReportDate  time.Time
	Total       int // ocorrências no PDF (todas as unidades)
	Occurrences []Occurrence
	Links       []PersonLink
	CreatedBy   string
}

// ImportResult diz o que foi gravado.
type ImportResult struct {
	ReportID string
	// Imported: índice em ImportInput.Occurrences → id gravado.
	Imported map[int]string
	// Skipped: fichas CIOPS que já existiam.
	Skipped    []string
	AutoLinked int
}

// Import grava relatório e ocorrências numa transação. Ocorrência cuja ficha
// CIOPS já existe é pulada (ON CONFLICT no índice único parcial), inclusive
// se outra importação concorrente a gravou primeiro.
func (r *Repo) Import(ctx context.Context, in ImportInput) (*ImportResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res := &ImportResult{Imported: map[int]string{}}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO app.ops_reports
		  (report_date, file_sha256, file_name, file_size,
		   total_occurrences, unit_occurrences, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (file_sha256) DO NOTHING
		RETURNING id`,
		nilDate(in.ReportDate), in.FileSHA256, in.FileName, in.FileSize,
		in.Total, len(in.Occurrences), in.CreatedBy,
	).Scan(&res.ReportID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAlreadyImported
	}
	if err != nil {
		return nil, fmt.Errorf("ops_reports: %w", err)
	}

	links := map[[2]int]string{}
	for _, l := range in.Links {
		links[[2]int{l.Occ, l.Person}] = l.EntityID
	}

	for i, o := range in.Occurrences {
		var id string
		err := tx.QueryRowContext(ctx, `
			INSERT INTO app.ops_occurrences
			  (report_id, source_page, natures, base_raw, base_city, cia, pel, bpm,
			   occurred_on, start_time, end_time, teams, ciops_record,
			   place_address, place_neighborhood, place_city,
			   approach_address, approach_neighborhood, approach_city,
			   police_station, delegate, procedure_type, procedure_number,
			   seized_objects, narrative, created_by,
			   intel_participation, intel_matched,
			   sipom_natureza_id, sipom_logradouro, sipom_numeral, sipom_cidade_id,
			   sipom_bairro_id, sipom_area_id, sipom_opm_id, sipom_pendencias,
			   latitude, longitude, geo_precision, geo_source,
			   sipom_procedimento_id, sipom_delegacia_id, sipom_delegado_id)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::time,$11::time,$12,$13,
			        $14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,
			        $29,$30,$31,$32,$33,$34,$35,$36,$37,$38,$39,$40,$41,$42,$43)
			ON CONFLICT (ciops_record) WHERE ciops_record <> '' AND deleted_at IS NULL
			DO NOTHING
			RETURNING id`,
			res.ReportID, o.Page, textArray(o.Natures), o.BaseRaw, o.BaseCity, o.CIA, o.PEL, o.BPM,
			o.OccurredOn, nilStr(o.StartTime), nilStr(o.EndTime), o.Teams, o.CIOPS,
			o.PlaceAddress, o.PlaceNeighborhood, o.PlaceCity,
			o.ApproachAddress, o.ApproachNeighborhood, o.ApproachCity,
			o.PoliceStation, o.Delegate, o.ProcedureType, o.ProcedureNumber,
			o.SeizedObjects, o.Narrative, in.CreatedBy,
			len(o.IntelMatched) > 0, textArray(o.IntelMatched),
			nilInt(o.Sipom.NaturezaID), o.Sipom.Logradouro, o.Sipom.Numeral, nilInt(o.Sipom.CidadeID),
			nilInt(o.Sipom.BairroID), nilInt(o.Sipom.AreaID), nilInt(o.Sipom.OPMID), textArray(o.Sipom.Pending),
			nilFloat(o.Geo.Lat), nilFloat(o.Geo.Lng), o.Geo.Precision, o.Geo.Source,
			nilInt(o.Sipom.ProcedimentoID), nilInt(o.Sipom.DelegaciaID), nilInt(o.Sipom.DelegadoID),
		).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			res.Skipped = append(res.Skipped, o.CIOPS)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("ops_occurrences: %w", err)
		}
		res.Imported[i] = id

		for k, p := range o.People {
			entityID := links[[2]int{i, k}]
			var ent, mode any
			var linkedAt, linkedBy any
			if entityID != "" {
				ent, mode, linkedAt, linkedBy = entityID, LinkAuto, time.Now(), in.CreatedBy
				res.AutoLinked++
			} else {
				mode = ""
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO app.ops_occurrence_people
				  (occurrence_id, position, role, name, mother_name, age, address, note,
				   entity_id, link_mode, linked_at, linked_by)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
				id, k, p.Role, p.Name, p.MotherName, nilInt(p.Age), p.Address, p.Note,
				ent, mode, linkedAt, linkedBy,
			); err != nil {
				return nil, fmt.Errorf("ops_occurrence_people: %w", err)
			}
		}
		for k, w := range o.Weapons {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO app.ops_occurrence_weapons
				  (occurrence_id, position, kind, model, brand, caliber, serial,
				   sipom_tipo_id, sipom_marca_id, sipom_calibre_id)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				id, k, w.Kind, w.Model, w.Brand, w.Caliber, w.Serial,
				nilInt(w.SipomTipoID), nilInt(w.SipomMarcaID), nilInt(w.SipomCalibreID),
			); err != nil {
				return nil, fmt.Errorf("ops_occurrence_weapons: %w", err)
			}
		}
		for k, d := range o.Drugs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO app.ops_occurrence_drugs
				  (occurrence_id, position, description, grams, packages,
				   sipom_droga_id, sipom_quantidade)
				VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				id, k, d.Description, nilFloat(d.Grams), nilInt(d.Packages),
				nilInt(d.SipomDrogaID), nilFloat(d.SipomQuantidade),
			); err != nil {
				return nil, fmt.Errorf("ops_occurrence_drugs: %w", err)
			}
		}
		for k, v := range o.Vehicles {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO app.ops_occurrence_vehicles
				  (occurrence_id, position, kind, brand, model, plate, color,
				   sipom_tipo_codigo, sipom_cor_codigo, sipom_marca_modelo_codigo, sipom_situacao)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
				id, k, v.Kind, v.Brand, v.Model, v.Plate, v.Color,
				nilInt(v.SipomTipoCodigo), nilInt(v.SipomCorCodigo), nilInt(v.SipomMarcaModeloCodigo), v.SipomSituacao,
			); err != nil {
				return nil, fmt.Errorf("ops_occurrence_vehicles: %w", err)
			}
		}
		for k, f := range o.Officers {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO app.ops_occurrence_officers
				  (occurrence_id, position, registration, rank, number, war_name, raw,
				   sipom_equipe, sipom_policiamento_tipo_id, sipom_funcao_id)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
				id, k, f.Registration, f.Rank, f.Number, f.WarName, f.Raw,
				f.SipomEquipe, nilInt(f.SipomPoliciamentoTipoID), nilInt(f.SipomFuncaoID),
			); err != nil {
				return nil, fmt.Errorf("ops_occurrence_officers: %w", err)
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE app.ops_reports
		   SET imported_occurrences = $2, skipped_occurrences = $3
		 WHERE id = $1`,
		res.ReportID, len(res.Imported), len(res.Skipped),
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return res, nil
}

// ListReports devolve os PDFs importados, do mais recente para o mais antigo.
func (r *Repo) ListReports(ctx context.Context, limit, offset int) ([]ImportedReport, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM app.ops_reports`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, reportSelect+`
		ORDER BY r.report_date DESC NULLS LAST, r.created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []ImportedReport{}
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *rep)
	}
	return out, total, rows.Err()
}

const reportSelect = `
	SELECT r.id, r.report_date, r.file_sha256, r.file_name, r.file_size,
	       r.total_occurrences, r.unit_occurrences, r.imported_occurrences,
	       r.skipped_occurrences, r.created_at, r.created_by,
	       COALESCE(u.display_name, '')
	  FROM app.ops_reports r
	  LEFT JOIN app.users u ON u.id = r.created_by`

func scanReport(rows *sql.Rows) (*ImportedReport, error) {
	var rep ImportedReport
	var d sql.NullTime
	if err := rows.Scan(&rep.ID, &d, &rep.FileSHA256, &rep.FileName, &rep.FileSize,
		&rep.TotalOccurrences, &rep.UnitOccurrences, &rep.ImportedOccurrences,
		&rep.SkippedOccurrences, &rep.CreatedAt, &rep.CreatedBy, &rep.CreatedByName,
	); err != nil {
		return nil, err
	}
	if d.Valid {
		t := d.Time
		rep.ReportDate = &t
	}
	return &rep, nil
}

// ─────────────────────────── Consulta ────────────────────────────

// StoredOccurrence é a ocorrência gravada, com id e filhos.
type StoredOccurrence struct {
	ID       string
	ReportID string
	Occurrence
	People    []StoredPerson
	CreatedAt time.Time
	CreatedBy string

	// Participação da inteligência; IntelMode diz se foi a regra ("auto") ou
	// o analista ("manual"). Os termos ficam em Occurrence.IntelMatched.
	IntelParticipation bool
	IntelMode          string

	// Report é a data do relatório (PDF) de onde a ocorrência veio.
	Report *time.Time

	// UpdatedAt/UpdatedByName: última correção do analista (Update). Nil =
	// a ficha ainda é a cópia do PDF.
	UpdatedAt     *time.Time
	UpdatedByName string

	// Contagens (preenchidas na listagem; no detalhe, os slices valem).
	PeopleCount, WeaponCount, DrugCount, VehicleCount int
	// AccusedNames lista os acusados na listagem (busca rápida na tabela).
	AccusedNames []string
}

// StoredPerson é a pessoa gravada, com o vínculo ao dossiê.
type StoredPerson struct {
	ID string
	Person
	EntityID       *string
	EntityName     string
	EntityDeceased bool
	LinkMode       string
	LinkedAt       *time.Time

	// Qualificação do dossiê vinculado — o que o SIPOM pede além do que o
	// relatório traz.
	EntityCPF        string
	EntityGender     string
	EntityMother     string
	EntityBirthDate  *time.Time
	EntityDeceasedOn *time.Time
	EntityHasPhoto   bool
}

// ListOpts controla a listagem de ocorrências.
type ListOpts struct {
	Limit, Offset int
	Search        string // natureza, bairro, cidade, ficha, equipe, pessoas, histórico
	CIA           string
	PEL           string
	Nature        string // contém (sem acento/caixa)
	City          string // cidade da ocorrência (exata, MAIÚSCULAS)
	ReportID      string
	DateFrom      string // YYYY-MM-DD
	DateTo        string
	Intel         string // "" = todas | "1" = com inteligência | "0" = sem
	SortBy        string // occurred_on | ciops_record | place_city | cia
	SortDir       string
}

var occSortable = map[string]string{
	"occurred_on":  "o.occurred_on",
	"ciops_record": "o.ciops_record",
	"place_city":   "o.place_city",
	"cia":          "o.cia",
}

// List devolve uma página de ocorrências com contagens dos filhos.
func (r *Repo) List(ctx context.Context, opts ListOpts) ([]StoredOccurrence, int, error) {
	if opts.Limit <= 0 || opts.Limit > 100 {
		opts.Limit = 25
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	search := strings.TrimSpace(opts.Search)
	args := []any{
		search,                      // $1
		"%" + search + "%",          // $2
		strings.TrimSpace(opts.CIA), // $3
		strings.TrimSpace(opts.PEL), // $4
		"%" + strings.TrimSpace(opts.Nature) + "%",    // $5
		strings.ToUpper(strings.TrimSpace(opts.City)), // $6
		nilStr(strings.TrimSpace(opts.ReportID)),      // $7
		nilStr(strings.TrimSpace(opts.DateFrom)),      // $8
		nilStr(strings.TrimSpace(opts.DateTo)),        // $9
		strings.TrimSpace(opts.Intel),                 // $10
	}
	where := `
		WHERE o.deleted_at IS NULL
		  AND ($1 = '' OR app.norm_txt(array_to_string(o.natures, ' ')) LIKE app.norm_txt($2)
		       OR app.norm_txt(o.place_neighborhood) LIKE app.norm_txt($2)
		       OR app.norm_txt(o.place_city) LIKE app.norm_txt($2)
		       OR app.norm_txt(o.place_address) LIKE app.norm_txt($2)
		       OR lower(o.ciops_record) LIKE lower($2)
		       OR lower(o.procedure_number) LIKE lower($2)
		       OR app.norm_txt(o.teams) LIKE app.norm_txt($2)
		       OR app.norm_txt(o.narrative) LIKE app.norm_txt($2)
		       OR EXISTS (SELECT 1 FROM app.ops_occurrence_people p
		                   WHERE p.occurrence_id = o.id
		                     AND (app.norm_txt(p.name) LIKE app.norm_txt($2)
		                          OR app.norm_txt(p.mother_name) LIKE app.norm_txt($2)))
		       OR EXISTS (SELECT 1 FROM app.ops_occurrence_vehicles v
		                   WHERE v.occurrence_id = o.id AND lower(v.plate) LIKE lower($2))
		       OR EXISTS (SELECT 1 FROM app.ops_occurrence_weapons w
		                   WHERE w.occurrence_id = o.id AND lower(w.serial) LIKE lower($2))
		       OR EXISTS (SELECT 1 FROM app.ops_occurrence_officers f
		                   WHERE f.occurrence_id = o.id
		                     AND (app.norm_txt(f.war_name) LIKE app.norm_txt($2)
		                          OR f.registration LIKE $2)))
		  AND ($3 = '' OR o.cia = $3)
		  AND ($4 = '' OR o.pel = $4)
		  AND ($5 = '%%' OR app.norm_txt(array_to_string(o.natures, ' ')) LIKE app.norm_txt($5))
		  AND ($6 = '' OR o.place_city = $6)
		  AND ($7::uuid IS NULL OR o.report_id = $7::uuid)
		  AND ($8::date IS NULL OR o.occurred_on >= $8::date)
		  AND ($9::date IS NULL OR o.occurred_on <= $9::date)
		  AND ($10 = '' OR o.intel_participation = ($10 = '1'))`

	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM app.ops_occurrences o`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	col, ok := occSortable[opts.SortBy]
	if !ok {
		col = "o.occurred_on"
	}
	dir := "DESC"
	if strings.ToLower(opts.SortDir) == "asc" {
		dir = "ASC"
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+occSelect+`,
		       (SELECT COUNT(*) FROM app.ops_occurrence_people p WHERE p.occurrence_id = o.id),
		       (SELECT COUNT(*) FROM app.ops_occurrence_weapons w WHERE w.occurrence_id = o.id),
		       (SELECT COUNT(*) FROM app.ops_occurrence_drugs d WHERE d.occurrence_id = o.id),
		       (SELECT COUNT(*) FROM app.ops_occurrence_vehicles v WHERE v.occurrence_id = o.id),
		       COALESCE((SELECT jsonb_agg(p.name ORDER BY p.position)
		                   FROM app.ops_occurrence_people p
		                  WHERE p.occurrence_id = o.id AND p.role = 'ACUSADO'), '[]'::jsonb)::text
		  FROM app.ops_occurrences o`+where+`
		 ORDER BY `+col+` `+dir+`, o.start_time `+dir+` NULLS LAST, o.created_at DESC
		 LIMIT $11 OFFSET $12`,
		append(args, opts.Limit, opts.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []StoredOccurrence{}
	for rows.Next() {
		var so StoredOccurrence
		var arr occArrays
		var accused string
		dest := append(occDest(&so, &arr), &so.PeopleCount, &so.WeaponCount, &so.DrugCount, &so.VehicleCount, &accused)
		if err := rows.Scan(dest...); err != nil {
			return nil, 0, err
		}
		if err := arr.decode(&so); err != nil {
			return nil, 0, err
		}
		if err := decodeStrings(accused, &so.AccusedNames); err != nil {
			return nil, 0, err
		}
		out = append(out, so)
	}
	return out, total, rows.Err()
}

// Facets são os valores distintos para os filtros da tela.
type Facets struct {
	CIAs    []string
	PELs    []string
	Cities  []string
	Natures []string
}

// ListFacets devolve CIAs, PELs, cidades e naturezas já registradas.
func (r *Repo) ListFacets(ctx context.Context) (*Facets, error) {
	f := &Facets{}
	queries := []struct {
		dst *[]string
		sql string
	}{
		{&f.CIAs, `SELECT DISTINCT cia FROM app.ops_occurrences WHERE deleted_at IS NULL AND cia <> '' ORDER BY 1`},
		{&f.PELs, `SELECT DISTINCT pel FROM app.ops_occurrences WHERE deleted_at IS NULL AND pel <> '' ORDER BY 1`},
		{&f.Cities, `SELECT DISTINCT place_city FROM app.ops_occurrences WHERE deleted_at IS NULL AND place_city <> '' ORDER BY 1`},
		{&f.Natures, `SELECT DISTINCT n FROM app.ops_occurrences, unnest(natures) n WHERE deleted_at IS NULL ORDER BY 1`},
	}
	for _, q := range queries {
		rows, err := r.db.QueryContext(ctx, q.sql)
		if err != nil {
			return nil, err
		}
		*q.dst = []string{}
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				rows.Close()
				return nil, err
			}
			*q.dst = append(*q.dst, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return f, nil
}

const occSelect = `
	o.id, o.report_id, o.source_page, to_jsonb(o.natures)::text, o.base_raw, o.base_city, o.cia, o.pel, o.bpm,
	o.occurred_on, COALESCE(to_char(o.start_time, 'HH24:MI'), ''), COALESCE(to_char(o.end_time, 'HH24:MI'), ''),
	o.teams, o.ciops_record,
	o.place_address, o.place_neighborhood, o.place_city,
	o.approach_address, o.approach_neighborhood, o.approach_city,
	o.police_station, o.delegate, o.procedure_type, o.procedure_number,
	o.seized_objects, o.narrative, o.created_at, o.created_by,
	o.intel_participation, o.intel_mode, to_jsonb(o.intel_matched)::text,
	o.sipom_natureza_id, o.sipom_logradouro, o.sipom_numeral, o.sipom_cidade_id, o.sipom_bairro_id,
	o.sipom_area_id, o.sipom_opm_id, to_jsonb(o.sipom_pendencias)::text, to_jsonb(o.sipom_manual)::text,
	(SELECT r.report_date FROM app.ops_reports r WHERE r.id = o.report_id),
	o.updated_at, COALESCE((SELECT u.display_name FROM app.users u WHERE u.id = o.updated_by), ''),
	o.latitude, o.longitude, o.geo_precision, o.geo_source,
	o.sipom_procedimento_id, o.sipom_delegacia_id, o.sipom_delegado_id`

// occArrays recebe as colunas de array de occSelect, que chegam como JSON
// (to_jsonb) e são decodificadas por decode.
type occArrays struct {
	natures, intelMatched, sipomPending, sipomManual string
}

func (a *occArrays) decode(so *StoredOccurrence) error {
	for _, x := range []struct {
		js  string
		dst *[]string
	}{
		{a.natures, &so.Natures},
		{a.intelMatched, &so.IntelMatched},
		{a.sipomPending, &so.Sipom.Pending},
		{a.sipomManual, &so.Sipom.Manual},
	} {
		if err := decodeStrings(x.js, x.dst); err != nil {
			return err
		}
	}
	return nil
}

// occDest aponta as colunas de occSelect para a struct.
func occDest(so *StoredOccurrence, arr *occArrays) []any {
	o := &so.Occurrence
	return []any{
		&so.ID, &so.ReportID, &o.Page, &arr.natures, &o.BaseRaw, &o.BaseCity, &o.CIA, &o.PEL, &o.BPM,
		&o.OccurredOn, &o.StartTime, &o.EndTime, &o.Teams, &o.CIOPS,
		&o.PlaceAddress, &o.PlaceNeighborhood, &o.PlaceCity,
		&o.ApproachAddress, &o.ApproachNeighborhood, &o.ApproachCity,
		&o.PoliceStation, &o.Delegate, &o.ProcedureType, &o.ProcedureNumber,
		&o.SeizedObjects, &o.Narrative, &so.CreatedAt, &so.CreatedBy,
		&so.IntelParticipation, &so.IntelMode, &arr.intelMatched,
		&o.Sipom.NaturezaID, &o.Sipom.Logradouro, &o.Sipom.Numeral, &o.Sipom.CidadeID, &o.Sipom.BairroID,
		&o.Sipom.AreaID, &o.Sipom.OPMID, &arr.sipomPending, &arr.sipomManual,
		&so.Report,
		&so.UpdatedAt, &so.UpdatedByName,
		&o.Geo.Lat, &o.Geo.Lng, &o.Geo.Precision, &o.Geo.Source,
		&o.Sipom.ProcedimentoID, &o.Sipom.DelegaciaID, &o.Sipom.DelegadoID,
	}
}

func decodeStrings(js string, dst *[]string) error {
	*dst = []string{}
	if js == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(js), dst); err != nil {
		return fmt.Errorf("decode array: %w", err)
	}
	return nil
}

// FindByID carrega a ocorrência com todos os filhos.
func (r *Repo) FindByID(ctx context.Context, id string) (*StoredOccurrence, error) {
	var so StoredOccurrence
	var arr occArrays
	err := r.db.QueryRowContext(ctx,
		`SELECT `+occSelect+` FROM app.ops_occurrences o WHERE o.id = $1 AND o.deleted_at IS NULL`, id,
	).Scan(occDest(&so, &arr)...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := arr.decode(&so); err != nil {
		return nil, err
	}

	people, err := r.listPeople(ctx, id)
	if err != nil {
		return nil, err
	}
	so.People = people

	var errMat error
	so.Weapons, so.Drugs, so.Vehicles, errMat = r.materials(ctx, id)
	if errMat != nil {
		return nil, errMat
	}
	if err := r.each(ctx, `SELECT registration, rank, number, war_name, raw,
		       sipom_equipe, sipom_policiamento_tipo_id, sipom_funcao_id FROM app.ops_occurrence_officers
		WHERE occurrence_id = $1 ORDER BY position`, id, func(rows *sql.Rows) error {
		var f Officer
		if err := rows.Scan(&f.Registration, &f.Rank, &f.Number, &f.WarName, &f.Raw,
			&f.SipomEquipe, &f.SipomPoliciamentoTipoID, &f.SipomFuncaoID); err != nil {
			return err
		}
		so.Officers = append(so.Officers, f)
		return nil
	}); err != nil {
		return nil, err
	}
	return &so, nil
}

// materials carrega armas, drogas e veículos da ocorrência, com a tradução
// para o SIPOM.
func (r *Repo) materials(ctx context.Context, id string) (weapons []Weapon, drugs []Drug, vehicles []Vehicle, err error) {
	if err := r.each(ctx, `SELECT kind, model, brand, caliber, serial,
		       sipom_tipo_id, sipom_marca_id, sipom_calibre_id, sipom_manual
		  FROM app.ops_occurrence_weapons WHERE occurrence_id = $1 ORDER BY position`, id, func(rows *sql.Rows) error {
		var w Weapon
		if err := rows.Scan(&w.Kind, &w.Model, &w.Brand, &w.Caliber, &w.Serial,
			&w.SipomTipoID, &w.SipomMarcaID, &w.SipomCalibreID, &w.SipomManual); err != nil {
			return err
		}
		weapons = append(weapons, w)
		return nil
	}); err != nil {
		return nil, nil, nil, err
	}
	if err := r.each(ctx, `SELECT description, grams::float8, packages,
		       sipom_droga_id, sipom_quantidade::float8, sipom_manual
		  FROM app.ops_occurrence_drugs WHERE occurrence_id = $1 ORDER BY position`, id, func(rows *sql.Rows) error {
		var d Drug
		var g sql.NullFloat64
		var p sql.NullInt64
		if err := rows.Scan(&d.Description, &g, &p, &d.SipomDrogaID, &d.SipomQuantidade, &d.SipomManual); err != nil {
			return err
		}
		if g.Valid {
			d.Grams = &g.Float64
		}
		if p.Valid {
			n := int(p.Int64)
			d.Packages = &n
		}
		drugs = append(drugs, d)
		return nil
	}); err != nil {
		return nil, nil, nil, err
	}
	if err := r.each(ctx, `SELECT kind, brand, model, plate, color,
		       sipom_tipo_codigo, sipom_cor_codigo, sipom_marca_modelo_codigo, sipom_situacao, sipom_manual
		  FROM app.ops_occurrence_vehicles WHERE occurrence_id = $1 ORDER BY position`, id, func(rows *sql.Rows) error {
		var v Vehicle
		if err := rows.Scan(&v.Kind, &v.Brand, &v.Model, &v.Plate, &v.Color,
			&v.SipomTipoCodigo, &v.SipomCorCodigo, &v.SipomMarcaModeloCodigo, &v.SipomSituacao, &v.SipomManual); err != nil {
			return err
		}
		vehicles = append(vehicles, v)
		return nil
	}); err != nil {
		return nil, nil, nil, err
	}
	return weapons, drugs, vehicles, nil
}

func (r *Repo) each(ctx context.Context, q, id string, fn func(*sql.Rows) error) error {
	rows, err := r.db.QueryContext(ctx, q, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := fn(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (r *Repo) listPeople(ctx context.Context, occID string) ([]StoredPerson, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.role, p.name, p.mother_name, p.age, p.address, p.note,
		       p.entity_id, COALESCE(e.name, ''), COALESCE(ep.deceased, false),
		       p.link_mode, p.linked_at,
		       COALESCE(ep.cpf, ''), COALESCE(ep.gender, ''), COALESCE(ep.mother_name, ''),
		       ep.date_of_birth, ep.deceased_on, COALESCE(ep.photo_path, '') <> ''
		  FROM app.ops_occurrence_people p
		  LEFT JOIN app.entities e ON e.id = p.entity_id
		  LEFT JOIN app.entity_persons ep ON ep.entity_id = p.entity_id
		 WHERE p.occurrence_id = $1
		 ORDER BY p.position`, occID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StoredPerson{}
	for rows.Next() {
		var sp StoredPerson
		var age sql.NullInt64
		var ent sql.NullString
		var linkedAt sql.NullTime
		if err := rows.Scan(&sp.ID, &sp.Role, &sp.Name, &sp.MotherName, &age, &sp.Address, &sp.Note,
			&ent, &sp.EntityName, &sp.EntityDeceased, &sp.LinkMode, &linkedAt,
			&sp.EntityCPF, &sp.EntityGender, &sp.EntityMother,
			&sp.EntityBirthDate, &sp.EntityDeceasedOn, &sp.EntityHasPhoto); err != nil {
			return nil, err
		}
		if age.Valid {
			n := int(age.Int64)
			sp.Age = &n
		}
		if ent.Valid {
			sp.EntityID = &ent.String
		}
		if linkedAt.Valid {
			sp.LinkedAt = &linkedAt.Time
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

// ─────────────────────────── Edição ────────────────────────────

// Edit são as correções do analista sobre a ocorrência importada — o PDF
// erra data, hora, ficha, bairro. Campo nil = não tocar. Os filhos (pessoas,
// armas, drogas, veículos, composição) não entram aqui.
type Edit struct {
	Natures    *[]string
	OccurredOn *time.Time
	StartTime  *string // "HH:MM"; "" limpa
	EndTime    *string
	Teams      *string
	CIOPS      *string
	BaseCity   *string
	CIA        *string
	PEL        *string

	PlaceAddress         *string
	PlaceNeighborhood    *string
	PlaceCity            *string
	ApproachAddress      *string
	ApproachNeighborhood *string
	ApproachCity         *string

	PoliceStation   *string
	Delegate        *string
	ProcedureType   *string
	ProcedureNumber *string

	SeizedObjects *string
	Narrative     *string
}

// Update aplica as correções. Segue a grafia da importação: naturezas,
// bairros e cidades em MAIÚSCULAS (é o que agrupa); ficha em maiúsculas e sem
// espaços; endereço e histórico como digitados. Quem chama refaz a tradução
// para o SIPOM e a marcação de inteligência, que dependem destes campos.
func (r *Repo) Update(ctx context.Context, id, actor string, e Edit) error {
	trim := func(p *string) any {
		if p == nil {
			return nil
		}
		return strings.TrimSpace(*p)
	}
	upper := func(p *string) any {
		if p == nil {
			return nil
		}
		return strings.ToUpper(strings.TrimSpace(*p))
	}
	clock := func(p *string) (bool, any) {
		if p == nil {
			return false, nil
		}
		return true, nilStr(strings.TrimSpace(*p))
	}
	var natures any
	if e.Natures != nil {
		list := []string{}
		for _, n := range *e.Natures {
			if n = strings.ToUpper(strings.Join(strings.Fields(n), " ")); n != "" {
				list = append(list, n)
			}
		}
		natures = list
	}
	var day any
	if e.OccurredOn != nil {
		day = e.OccurredOn.Format("2006-01-02")
	}
	var ciops any
	if e.CIOPS != nil {
		ciops = strings.ToUpper(strings.Join(strings.Fields(*e.CIOPS), ""))
	}
	setStart, start := clock(e.StartTime)
	setEnd, end := clock(e.EndTime)

	res, err := r.db.ExecContext(ctx, `
		UPDATE app.ops_occurrences SET
		  natures               = COALESCE($2::text[], natures),
		  occurred_on           = COALESCE($3::date, occurred_on),
		  start_time            = CASE WHEN $4 THEN $5::time ELSE start_time END,
		  end_time              = CASE WHEN $6 THEN $7::time ELSE end_time END,
		  teams                 = COALESCE($8::text, teams),
		  ciops_record          = COALESCE($9::text, ciops_record),
		  base_city             = COALESCE($10::text, base_city),
		  cia                   = COALESCE($11::text, cia),
		  pel                   = COALESCE($12::text, pel),
		  place_address         = COALESCE($13::text, place_address),
		  place_neighborhood    = COALESCE($14::text, place_neighborhood),
		  place_city            = COALESCE($15::text, place_city),
		  approach_address      = COALESCE($16::text, approach_address),
		  approach_neighborhood = COALESCE($17::text, approach_neighborhood),
		  approach_city         = COALESCE($18::text, approach_city),
		  police_station        = COALESCE($19::text, police_station),
		  delegate              = COALESCE($20::text, delegate),
		  procedure_type        = COALESCE($21::text, procedure_type),
		  procedure_number      = COALESCE($22::text, procedure_number),
		  seized_objects        = COALESCE($23::text, seized_objects),
		  narrative             = COALESCE($24::text, narrative),
		  updated_at            = now(),
		  updated_by            = $25
		WHERE id = $1 AND deleted_at IS NULL`,
		id, natures, day, setStart, start, setEnd, end,
		trim(e.Teams), ciops, upper(e.BaseCity), upper(e.CIA), upper(e.PEL),
		trim(e.PlaceAddress), upper(e.PlaceNeighborhood), upper(e.PlaceCity),
		trim(e.ApproachAddress), upper(e.ApproachNeighborhood), upper(e.ApproachCity),
		trim(e.PoliceStation), trim(e.Delegate), trim(e.ProcedureType), trim(e.ProcedureNumber),
		trim(e.SeizedObjects), trim(e.Narrative), actor,
	)
	if err != nil {
		if strings.Contains(err.Error(), "ops_occurrences_ciops_uniq") {
			return ErrDuplicateCIOPS
		}
		return fmt.Errorf("ops_occurrences update: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ─────────────────────────── Coordenada ────────────────────────────

// SetGeo grava a coordenada da ocorrência — a do geocodificador ("auto", com
// a precisão) ou a do analista ("manual"). Lat/Lng nil limpa: a ocorrência
// volta a "não localizada". Quem chama refaz a tradução, que carrega a
// pendência de coordenada.
func (r *Repo) SetGeo(ctx context.Context, id string, g Geo) error {
	if !g.Located() {
		g = Geo{}
	}
	return r.execOne(ctx, `
		UPDATE app.ops_occurrences
		   SET latitude = $2, longitude = $3, geo_precision = $4, geo_source = $5
		 WHERE id = $1 AND deleted_at IS NULL`,
		id, nilFloat(g.Lat), nilFloat(g.Lng), g.Precision, g.Source)
}

// Unlocated devolve as ocorrências sem coordenada, para a geocodificação do
// acervo (o que foi importado antes de o geocodificador existir, ou enquanto
// ele estava fora do ar). Só os campos de endereço vêm preenchidos.
func (r *Repo) Unlocated(ctx context.Context) ([]StoredOccurrence, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.id, o.place_address, o.place_neighborhood, o.place_city,
		       o.approach_address, o.approach_neighborhood, o.approach_city
		  FROM app.ops_occurrences o
		 WHERE o.deleted_at IS NULL AND o.latitude IS NULL
		 ORDER BY o.occurred_on DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StoredOccurrence{}
	for rows.Next() {
		var so StoredOccurrence
		o := &so.Occurrence
		if err := rows.Scan(&so.ID, &o.PlaceAddress, &o.PlaceNeighborhood, &o.PlaceCity,
			&o.ApproachAddress, &o.ApproachNeighborhood, &o.ApproachCity); err != nil {
			return nil, err
		}
		out = append(out, so)
	}
	return out, rows.Err()
}

// SetIntelMatched regrava a marcação automática de inteligência de uma
// ocorrência (o histórico ou a equipe mudaram). Não toca em decisão manual.
func (r *Repo) SetIntelMatched(ctx context.Context, id string, matched []string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE app.ops_occurrences
		   SET intel_participation = $2, intel_matched = $3
		 WHERE id = $1 AND deleted_at IS NULL AND intel_mode = 'auto'`,
		id, len(matched) > 0, textArray(matched))
	return err
}

// CountPending conta as ocorrências com alguma das pendências informadas.
func (r *Repo) CountPending(ctx context.Context, codes ...string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM app.ops_occurrences
		 WHERE deleted_at IS NULL AND sipom_pendencias && $1`, codes).Scan(&n)
	return n, err
}

// ─────────────────────────── Vínculo com dossiê ────────────────────────────

// FindPerson devolve uma pessoa da ocorrência.
func (r *Repo) FindPerson(ctx context.Context, occID, personID string) (*StoredPerson, error) {
	people, err := r.listPeople(ctx, occID)
	if err != nil {
		return nil, err
	}
	for i := range people {
		if people[i].ID == personID {
			return &people[i], nil
		}
	}
	return nil, ErrPersonNotFound
}

// LinkPerson vincula (entityID != "") ou desvincula (entityID == "") a pessoa
// ao dossiê.
func (r *Repo) LinkPerson(ctx context.Context, occID, personID, entityID, mode, actor string) error {
	var res sql.Result
	var err error
	if entityID == "" {
		res, err = r.db.ExecContext(ctx, `
			UPDATE app.ops_occurrence_people
			   SET entity_id = NULL, link_mode = '', linked_at = NULL, linked_by = NULL
			 WHERE id = $1 AND occurrence_id = $2`, personID, occID)
	} else {
		res, err = r.db.ExecContext(ctx, `
			UPDATE app.ops_occurrence_people
			   SET entity_id = $3, link_mode = $4, linked_at = now(), linked_by = $5
			 WHERE id = $1 AND occurrence_id = $2`, personID, occID, entityID, mode, actor)
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrPersonNotFound
	}
	return nil
}

// PersonDeceased informa se o dossiê está marcado como óbito.
func (r *Repo) PersonDeceased(ctx context.Context, entityID string) (bool, error) {
	var d bool
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(deceased, false) FROM app.entity_persons WHERE entity_id = $1`, entityID).Scan(&d)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return d, err
}

// ─────────────────────────── helpers ────────────────────────────

func nilStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nilInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nilFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nilDate(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// textArray: o driver pgx converte []string em text[] direto.
// ─────────────────────────── SIPOM ────────────────────────────

// SipomSource é o que a tradução para o SIPOM lê de uma ocorrência gravada.
type SipomSource struct {
	ID string
	Occurrence
	Manual []string
}

// SipomSources carrega as ocorrências (não excluídas) para recalcular a
// tradução — todas, ou só a informada.
func (r *Repo) SipomSources(ctx context.Context, onlyID string) ([]SipomSource, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.id, to_jsonb(o.natures)::text, COALESCE(to_char(o.start_time, 'HH24:MI'), ''), o.ciops_record,
		       o.place_address, o.place_neighborhood, o.place_city,
		       o.approach_address, o.approach_neighborhood, o.approach_city,
		       o.cia, o.bpm, o.teams, o.sipom_natureza_id, to_jsonb(o.sipom_manual)::text,
		       (SELECT COUNT(*) FROM app.ops_occurrence_officers f WHERE f.occurrence_id = o.id),
		       o.sipom_logradouro, o.sipom_numeral, o.sipom_area_id, o.sipom_opm_id,
		       o.latitude, o.longitude,
		       o.police_station, o.delegate, o.procedure_type, o.procedure_number,
		       o.sipom_procedimento_id, o.sipom_delegacia_id, o.sipom_delegado_id
		  FROM app.ops_occurrences o
		 WHERE o.deleted_at IS NULL AND ($1 = '' OR o.id::text = $1)
		 ORDER BY o.occurred_on`, onlyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SipomSource
	for rows.Next() {
		var s SipomSource
		var natures, manual string
		var officers int
		if err := rows.Scan(&s.ID, &natures, &s.StartTime, &s.CIOPS,
			&s.PlaceAddress, &s.PlaceNeighborhood, &s.PlaceCity,
			&s.ApproachAddress, &s.ApproachNeighborhood, &s.ApproachCity,
			&s.CIA, &s.BPM, &s.Teams, &s.Sipom.NaturezaID, &manual, &officers,
			&s.Sipom.Logradouro, &s.Sipom.Numeral, &s.Sipom.AreaID, &s.Sipom.OPMID,
			&s.Geo.Lat, &s.Geo.Lng,
			&s.PoliceStation, &s.Delegate, &s.ProcedureType, &s.ProcedureNumber,
			&s.Sipom.ProcedimentoID, &s.Sipom.DelegaciaID, &s.Sipom.DelegadoID); err != nil {
			return nil, err
		}
		if err := decodeStrings(natures, &s.Natures); err != nil {
			return nil, err
		}
		if err := decodeStrings(manual, &s.Manual); err != nil {
			return nil, err
		}
		s.Officers = make([]Officer, officers)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Envolvidos (nome e se têm dossiê): decidem o aviso de pessoa sem
	// dossiê.
	idx := map[string]int{}
	for i := range out {
		idx[out[i].ID] = i
	}
	prows, err := r.db.QueryContext(ctx, `
		SELECT p.occurrence_id, p.name, p.entity_id IS NOT NULL
		  FROM app.ops_occurrence_people p
		  JOIN app.ops_occurrences o ON o.id = p.occurrence_id
		 WHERE o.deleted_at IS NULL AND ($1 = '' OR o.id::text = $1)
		 ORDER BY p.occurrence_id, p.position`, onlyID)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	for prows.Next() {
		var occID, name string
		var linked bool
		if err := prows.Scan(&occID, &name, &linked); err != nil {
			return nil, err
		}
		if i, ok := idx[occID]; ok {
			out[i].People = append(out[i].People, Person{Name: name})
			out[i].peopleLinked = append(out[i].peopleLinked, linked)
		}
	}
	if err := prows.Err(); err != nil {
		return nil, err
	}
	// Materiais, que também são traduzidos.
	for i := range out {
		w, d, v, err := r.materials(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Weapons, out[i].Drugs, out[i].Vehicles = w, d, v
	}
	return out, nil
}

// SaveSipom grava a tradução de uma ocorrência. Campos que o analista fixou
// (sipom_manual) ficam como estão; a composição também, se "composicao"
// estiver entre eles. As pendências são sempre regravadas.
func (r *Repo) SaveSipom(ctx context.Context, id string, f SipomFields, officers []Officer) error {
	return r.SaveSipomAll(ctx, id, f, officers, nil, nil, nil)
}

// SaveSipomAll é SaveSipom com os materiais traduzidos (armas, drogas e
// veículos, na ordem das posições). Item fixado pelo analista (sipom_manual)
// não é tocado; listas nil não são gravadas.
func (r *Repo) SaveSipomAll(ctx context.Context, id string, f SipomFields, officers []Officer,
	weapons []Weapon, drugs []Drug, vehicles []Vehicle) error {
	manual := map[string]bool{}
	var cur []string
	var js string
	if err := r.db.QueryRowContext(ctx,
		`SELECT to_jsonb(sipom_manual)::text FROM app.ops_occurrences WHERE id = $1`, id).Scan(&js); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if err := decodeStrings(js, &cur); err != nil {
		return err
	}
	for _, m := range cur {
		manual[m] = true
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		UPDATE app.ops_occurrences SET
		  sipom_natureza_id = CASE WHEN $2 THEN sipom_natureza_id ELSE $3::integer END,
		  sipom_logradouro  = CASE WHEN $4 THEN sipom_logradouro ELSE $5 END,
		  sipom_numeral     = CASE WHEN $4 THEN sipom_numeral ELSE $6 END,
		  sipom_cidade_id   = CASE WHEN $7 THEN sipom_cidade_id ELSE $8::integer END,
		  sipom_bairro_id   = CASE WHEN $7 THEN sipom_bairro_id ELSE $9::integer END,
		  sipom_area_id     = CASE WHEN $10 THEN sipom_area_id ELSE $11::integer END,
		  sipom_opm_id      = CASE WHEN $12 THEN sipom_opm_id ELSE $13::integer END,
		  sipom_pendencias  = $14,
		  sipom_procedimento_id = CASE WHEN $15 THEN sipom_procedimento_id ELSE $16::integer END,
		  sipom_delegacia_id    = CASE WHEN $17 THEN sipom_delegacia_id ELSE $18::integer END,
		  sipom_delegado_id     = CASE WHEN $19 THEN sipom_delegado_id ELSE $20::integer END
		WHERE id = $1`,
		id, manual[SipomFieldNatureza], nilInt(f.NaturezaID),
		manual[SipomFieldEndereco], f.Logradouro, f.Numeral,
		manual[SipomFieldLocal], nilInt(f.CidadeID), nilInt(f.BairroID),
		manual[SipomFieldArea], nilInt(f.AreaID),
		manual[SipomFieldOPM], nilInt(f.OPMID),
		textArray(f.Pending),
		manual[SipomFieldProcedimento], nilInt(f.ProcedimentoID),
		manual[SipomFieldDelegacia], nilInt(f.DelegaciaID),
		manual[SipomFieldDelegado], nilInt(f.DelegadoID),
	); err != nil {
		return fmt.Errorf("sipom occurrence: %w", err)
	}
	for k, w := range weapons {
		if _, err := tx.ExecContext(ctx, `
			UPDATE app.ops_occurrence_weapons
			   SET sipom_tipo_id = $3, sipom_marca_id = $4, sipom_calibre_id = $5
			 WHERE occurrence_id = $1 AND position = $2 AND NOT sipom_manual`,
			id, k, nilInt(w.SipomTipoID), nilInt(w.SipomMarcaID), nilInt(w.SipomCalibreID)); err != nil {
			return fmt.Errorf("sipom weapons: %w", err)
		}
	}
	for k, d := range drugs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE app.ops_occurrence_drugs
			   SET sipom_droga_id = $3, sipom_quantidade = $4
			 WHERE occurrence_id = $1 AND position = $2 AND NOT sipom_manual`,
			id, k, nilInt(d.SipomDrogaID), nilFloat(d.SipomQuantidade)); err != nil {
			return fmt.Errorf("sipom drugs: %w", err)
		}
	}
	for k, v := range vehicles {
		if _, err := tx.ExecContext(ctx, `
			UPDATE app.ops_occurrence_vehicles
			   SET sipom_tipo_codigo = $3, sipom_cor_codigo = $4, sipom_marca_modelo_codigo = $5, sipom_situacao = $6
			 WHERE occurrence_id = $1 AND position = $2 AND NOT sipom_manual`,
			id, k, nilInt(v.SipomTipoCodigo), nilInt(v.SipomCorCodigo), nilInt(v.SipomMarcaModeloCodigo), v.SipomSituacao); err != nil {
			return fmt.Errorf("sipom vehicles: %w", err)
		}
	}
	if !manual[SipomFieldComposicao] {
		for k, o := range officers {
			if _, err := tx.ExecContext(ctx, `
				UPDATE app.ops_occurrence_officers
				   SET sipom_equipe = $3, sipom_policiamento_tipo_id = $4, sipom_funcao_id = $5
				 WHERE occurrence_id = $1 AND position = $2`,
				id, k, o.SipomEquipe, nilInt(o.SipomPoliciamentoTipoID), nilInt(o.SipomFuncaoID),
			); err != nil {
				return fmt.Errorf("sipom officers: %w", err)
			}
		}
	}
	return tx.Commit()
}

// SetSipomNatureza fixa a natureza SIPOM escolhida pelo analista (marca
// "natureza" como manual) ou, com naturezaID nil, devolve a decisão ao
// de-para. Quem chama recalcula a ocorrência em seguida.
func (r *Repo) SetSipomNatureza(ctx context.Context, id string, naturezaID *int) error {
	var q string
	var args []any
	if naturezaID != nil {
		q = `UPDATE app.ops_occurrences
		        SET sipom_natureza_id = $2,
		            sipom_manual = CASE WHEN $3 = ANY(sipom_manual) THEN sipom_manual
		                                ELSE array_append(sipom_manual, $3) END
		      WHERE id = $1 AND deleted_at IS NULL`
		args = []any{id, *naturezaID, SipomFieldNatureza}
	} else {
		q = `UPDATE app.ops_occurrences
		        SET sipom_manual = array_remove(sipom_manual, $2)
		      WHERE id = $1 AND deleted_at IS NULL`
		args = []any{id, SipomFieldNatureza}
	}
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// sipomManualSQL marca ($2) ou desmarca o campo como fixado pelo analista.
const sipomManualAdd = `CASE WHEN $2 = ANY(sipom_manual) THEN sipom_manual ELSE array_append(sipom_manual, $2) END`
const sipomManualDel = `array_remove(sipom_manual, $2)`

func (r *Repo) execOne(ctx context.Context, q string, args ...any) error {
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetSipomArea fixa a área da unidade (companhia territorial) ou, com nil,
// devolve ao cálculo pelo catálogo.
func (r *Repo) SetSipomArea(ctx context.Context, id string, areaID *int) error {
	if areaID == nil {
		return r.execOne(ctx, `UPDATE app.ops_occurrences SET sipom_manual = `+sipomManualDel+`
			WHERE id = $1 AND deleted_at IS NULL`, id, SipomFieldArea)
	}
	return r.execOne(ctx, `UPDATE app.ops_occurrences SET sipom_area_id = $3, sipom_manual = `+sipomManualAdd+`
		WHERE id = $1 AND deleted_at IS NULL`, id, SipomFieldArea, *areaID)
}

// SetSipomOPM fixa a companhia que atendeu ou, com nil, devolve ao cálculo.
func (r *Repo) SetSipomOPM(ctx context.Context, id string, opmID *int) error {
	if opmID == nil {
		return r.execOne(ctx, `UPDATE app.ops_occurrences SET sipom_manual = `+sipomManualDel+`
			WHERE id = $1 AND deleted_at IS NULL`, id, SipomFieldOPM)
	}
	return r.execOne(ctx, `UPDATE app.ops_occurrences SET sipom_opm_id = $3, sipom_manual = `+sipomManualAdd+`
		WHERE id = $1 AND deleted_at IS NULL`, id, SipomFieldOPM, *opmID)
}

// SetSipomEndereco fixa logradouro e número ou, com reset, devolve à
// separação automática do endereço do relatório.
func (r *Repo) SetSipomEndereco(ctx context.Context, id, logradouro, numeral string, reset bool) error {
	if reset {
		return r.execOne(ctx, `UPDATE app.ops_occurrences SET sipom_manual = `+sipomManualDel+`
			WHERE id = $1 AND deleted_at IS NULL`, id, SipomFieldEndereco)
	}
	return r.execOne(ctx, `UPDATE app.ops_occurrences
		SET sipom_logradouro = $3, sipom_numeral = $4, sipom_manual = `+sipomManualAdd+`
		WHERE id = $1 AND deleted_at IS NULL`, id, SipomFieldEndereco, logradouro, numeral)
}

// SetSipomComposicao grava a composição definida pelo analista (equipe,
// tipo e função de cada policial, na ordem da lista) ou, com nil, devolve à
// regra automática.
func (r *Repo) SetSipomComposicao(ctx context.Context, id string, officers []Officer) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := `UPDATE app.ops_occurrences SET sipom_manual = ` + sipomManualAdd + ` WHERE id = $1 AND deleted_at IS NULL`
	if officers == nil {
		q = `UPDATE app.ops_occurrences SET sipom_manual = ` + sipomManualDel + ` WHERE id = $1 AND deleted_at IS NULL`
	}
	res, err := tx.ExecContext(ctx, q, id, SipomFieldComposicao)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	for k, o := range officers {
		if _, err := tx.ExecContext(ctx, `
			UPDATE app.ops_occurrence_officers
			   SET sipom_equipe = $3, sipom_policiamento_tipo_id = $4, sipom_funcao_id = $5
			 WHERE occurrence_id = $1 AND position = $2`,
			id, k, o.SipomEquipe, nilInt(o.SipomPoliciamentoTipoID), nilInt(o.SipomFuncaoID)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetSipomProcedimento fixa o que o analista decidiu do procedimento: cada
// campo informado (não nil) é gravado e marcado como manual; reset devolve
// os três ao automático. Quem chama recalcula em seguida.
func (r *Repo) SetSipomProcedimento(ctx context.Context, id string, proc, delegacia, delegado *int, reset bool) error {
	if reset {
		return r.execOne(ctx, `
			UPDATE app.ops_occurrences
			   SET sipom_manual = array_remove(array_remove(array_remove(sipom_manual, $2), $3), $4)
			 WHERE id = $1 AND deleted_at IS NULL`,
			id, SipomFieldProcedimento, SipomFieldDelegacia, SipomFieldDelegado)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, f := range []struct {
		col, field string
		val        *int
	}{
		{"sipom_procedimento_id", SipomFieldProcedimento, proc},
		{"sipom_delegacia_id", SipomFieldDelegacia, delegacia},
		{"sipom_delegado_id", SipomFieldDelegado, delegado},
	} {
		if f.val == nil {
			continue
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE app.ops_occurrences
			   SET `+f.col+` = $3, sipom_manual = `+sipomManualAdd+`
			 WHERE id = $1 AND deleted_at IS NULL`, id, f.field, *f.val)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
	}
	return tx.Commit()
}

// SetWeaponSipom fixa a tradução de uma arma (posição na lista) ou, com
// reset, devolve ao automático.
func (r *Repo) SetWeaponSipom(ctx context.Context, id string, pos int, tipo, marca, calibre *int, reset bool) error {
	return r.execOne(ctx, `
		UPDATE app.ops_occurrence_weapons w
		   SET sipom_tipo_id = $4, sipom_marca_id = $5, sipom_calibre_id = $6, sipom_manual = $3
		  FROM app.ops_occurrences o
		 WHERE w.occurrence_id = $1 AND w.position = $2 AND o.id = w.occurrence_id AND o.deleted_at IS NULL`,
		id, pos, !reset, nilInt(tipo), nilInt(marca), nilInt(calibre))
}

// SetDrugSipom fixa a tradução de uma droga ou, com reset, devolve ao
// automático.
func (r *Repo) SetDrugSipom(ctx context.Context, id string, pos int, droga *int, quantidade *float64, reset bool) error {
	return r.execOne(ctx, `
		UPDATE app.ops_occurrence_drugs d
		   SET sipom_droga_id = $4, sipom_quantidade = $5, sipom_manual = $3
		  FROM app.ops_occurrences o
		 WHERE d.occurrence_id = $1 AND d.position = $2 AND o.id = d.occurrence_id AND o.deleted_at IS NULL`,
		id, pos, !reset, nilInt(droga), nilFloat(quantidade))
}

// SetVehicleSipom fixa a tradução de um veículo ou, com reset, devolve ao
// automático.
func (r *Repo) SetVehicleSipom(ctx context.Context, id string, pos int, tipo, cor, marcaModelo *int, situacao int, reset bool) error {
	return r.execOne(ctx, `
		UPDATE app.ops_occurrence_vehicles v
		   SET sipom_tipo_codigo = $4, sipom_cor_codigo = $5, sipom_marca_modelo_codigo = $6,
		       sipom_situacao = $7, sipom_manual = $3
		  FROM app.ops_occurrences o
		 WHERE v.occurrence_id = $1 AND v.position = $2 AND o.id = v.occurrence_id AND o.deleted_at IS NULL`,
		id, pos, !reset, nilInt(tipo), nilInt(cor), nilInt(marcaModelo), situacao)
}

// ConfirmSipomNaturezas confirma a natureza sugerida pelo de-para nas
// ocorrências informadas (fixa como do analista) e devolve as alteradas.
// Só mexe nas que estão pendentes de confirmação.
func (r *Repo) ConfirmSipomNaturezas(ctx context.Context, ids []string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		UPDATE app.ops_occurrences
		   SET sipom_manual = CASE WHEN $2 = ANY(sipom_manual) THEN sipom_manual
		                           ELSE array_append(sipom_manual, $2) END
		 WHERE id::text = ANY($1) AND deleted_at IS NULL
		   AND sipom_natureza_id IS NOT NULL
		   AND $3 = ANY(sipom_pendencias)
		RETURNING id`, ids, SipomFieldNatureza, "natureza_confirmar")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SipomQueueRow é uma linha da fila de envio ao SIPOM.
type SipomQueueRow struct {
	ID          string
	CIOPS       string
	OccurredOn  time.Time
	StartTime   string
	Natures     []string
	NaturezaID  *int
	City        string
	Neigh       string
	AreaID      *int
	OPMID       *int
	Pending     []string
	People      int
	Unlinked    int
	Officers    int
	IntelPartic bool
}

// SipomQueue lista as ocorrências para a fila de envio, das mais recentes
// para as mais antigas, no período informado (YYYY-MM-DD, "" = sem limite).
func (r *Repo) SipomQueue(ctx context.Context, dateFrom, dateTo string) ([]SipomQueueRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.id, o.ciops_record, o.occurred_on, COALESCE(to_char(o.start_time, 'HH24:MI'), ''),
		       to_jsonb(o.natures)::text, o.sipom_natureza_id, o.place_city, o.place_neighborhood,
		       o.sipom_area_id, o.sipom_opm_id, to_jsonb(o.sipom_pendencias)::text,
		       (SELECT COUNT(*) FROM app.ops_occurrence_people p WHERE p.occurrence_id = o.id),
		       (SELECT COUNT(*) FROM app.ops_occurrence_people p WHERE p.occurrence_id = o.id AND p.entity_id IS NULL),
		       (SELECT COUNT(*) FROM app.ops_occurrence_officers f WHERE f.occurrence_id = o.id),
		       o.intel_participation
		  FROM app.ops_occurrences o
		 WHERE o.deleted_at IS NULL
		   AND ($1::date IS NULL OR o.occurred_on >= $1::date)
		   AND ($2::date IS NULL OR o.occurred_on <= $2::date)
		 ORDER BY o.occurred_on DESC, o.start_time DESC NULLS LAST`,
		nilStr(dateFrom), nilStr(dateTo))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SipomQueueRow{}
	for rows.Next() {
		var x SipomQueueRow
		var natures, pending string
		if err := rows.Scan(&x.ID, &x.CIOPS, &x.OccurredOn, &x.StartTime, &natures, &x.NaturezaID,
			&x.City, &x.Neigh, &x.AreaID, &x.OPMID, &pending, &x.People, &x.Unlinked, &x.Officers,
			&x.IntelPartic); err != nil {
			return nil, err
		}
		if err := decodeStrings(natures, &x.Natures); err != nil {
			return nil, err
		}
		if err := decodeStrings(pending, &x.Pending); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// SeenNature é uma natureza que aparece no acervo, com a contagem.
type SeenNature struct {
	Nature string
	Count  int
}

// SeenNatures lista as naturezas das ocorrências gravadas (como o relatório
// escreveu), da mais frequente para a menos.
func (r *Repo) SeenNatures(ctx context.Context) ([]SeenNature, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT n, COUNT(*) FROM app.ops_occurrences, unnest(natures) n
		 WHERE deleted_at IS NULL GROUP BY n ORDER BY 2 DESC, 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SeenNature{}
	for rows.Next() {
		var x SeenNature
		if err := rows.Scan(&x.Nature, &x.Count); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// Campos da tradução que o analista pode fixar (valores de sipom_manual).
const (
	SipomFieldNatureza     = "natureza"
	SipomFieldEndereco     = "endereco"
	SipomFieldLocal        = "local"
	SipomFieldArea         = "area"
	SipomFieldOPM          = "opm"
	SipomFieldComposicao   = "composicao"
	SipomFieldProcedimento = "procedimento"
	SipomFieldDelegacia    = "delegacia"
	SipomFieldDelegado     = "delegado"
)

// ─────────────────────────── Inteligência ────────────────────────────

// Modo da marcação de participação da inteligência.
const (
	IntelAuto   = "auto"
	IntelManual = "manual"
)

// SetIntel grava a decisão do analista. A partir daqui a ocorrência fica em
// modo manual e a reaplicação dos termos não a altera mais. Os termos que a
// regra encontrou são mantidos, para mostrar o que ela tinha visto.
func (r *Repo) SetIntel(ctx context.Context, id string, value bool) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE app.ops_occurrences
		   SET intel_participation = $2, intel_mode = 'manual'
		 WHERE id = $1 AND deleted_at IS NULL`, id, value)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReapplyResult resume uma reaplicação dos termos.
type ReapplyResult struct {
	Checked int // ocorrências em modo automático avaliadas
	Marked  int // passaram a ter participação
	Cleared int // deixaram de ter
}

// ReapplyIntel reavalia as ocorrências em modo automático com o comparador
// atual — é como os termos novos alcançam o que já foi importado. As
// decisões manuais ficam intocadas.
func (r *Repo) ReapplyIntel(ctx context.Context, match func(texts ...string) []string) (*ReapplyResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, narrative, teams, intel_participation, to_jsonb(intel_matched)::text
		  FROM app.ops_occurrences
		 WHERE deleted_at IS NULL AND intel_mode = 'auto'
		 FOR UPDATE`)
	if err != nil {
		return nil, err
	}
	type change struct {
		id      string
		matched []string
	}
	res := &ReapplyResult{}
	var changes []change
	for rows.Next() {
		var o Occurrence
		var id, prevJSON string
		var prev bool
		if err := rows.Scan(&id, &o.Narrative, &o.Teams, &prev, &prevJSON); err != nil {
			rows.Close()
			return nil, err
		}
		res.Checked++
		matched := match(o.IntelTexts()...)
		var before []string
		if err := decodeStrings(prevJSON, &before); err != nil {
			rows.Close()
			return nil, err
		}
		now := len(matched) > 0
		switch {
		case now && !prev:
			res.Marked++
		case !now && prev:
			res.Cleared++
		}
		if now != prev || strings.Join(before, "\x00") != strings.Join(matched, "\x00") {
			changes = append(changes, change{id, matched})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, c := range changes {
		if _, err := tx.ExecContext(ctx, `
			UPDATE app.ops_occurrences
			   SET intel_participation = $2, intel_matched = $3
			 WHERE id = $1`, c.id, len(c.matched) > 0, textArray(c.matched)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return res, nil
}

func textArray(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}

// ─────────────────────────── Por entidade ────────────────────────────

// EntityOccurrence é uma ocorrência do relatório vista a partir da pessoa
// vinculada.
type EntityOccurrence struct {
	ID           string
	Natures      []string
	OccurredOn   time.Time
	StartTime    string
	CIA          string
	PEL          string
	City         string
	Neighborhood string
	CIOPS        string
	Role         string
}

// ListByEntity devolve as ocorrências em que a entidade foi vinculada a uma
// pessoa do relatório (automática ou manualmente).
func (r *Repo) ListByEntity(ctx context.Context, entityID string) ([]EntityOccurrence, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.id, to_jsonb(o.natures)::text, o.occurred_on,
		       COALESCE(to_char(o.start_time, 'HH24:MI'), ''),
		       o.cia, o.pel, o.place_city, o.place_neighborhood, o.ciops_record, p.role
		  FROM app.ops_occurrence_people p
		  JOIN app.ops_occurrences o ON o.id = p.occurrence_id
		 WHERE p.entity_id = $1 AND o.deleted_at IS NULL
		 ORDER BY o.occurred_on DESC, o.start_time DESC NULLS LAST`, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EntityOccurrence{}
	for rows.Next() {
		var e EntityOccurrence
		var natures string
		if err := rows.Scan(&e.ID, &natures, &e.OccurredOn, &e.StartTime, &e.CIA, &e.PEL,
			&e.City, &e.Neighborhood, &e.CIOPS, &e.Role); err != nil {
			return nil, err
		}
		if err := decodeStrings(natures, &e.Natures); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
