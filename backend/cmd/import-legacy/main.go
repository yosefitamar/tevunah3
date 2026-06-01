// import-legacy é o transformador da migração do sistema legado (Laravel +
// MySQL) para o Tevunah novo (Go + Postgres). Espera que o schema `legacy.*`
// do Postgres já tenha sido populado pelo pgloader (./tevunah import:load-mysql).
//
// Fluxo:
//   1) Abre transação única (idempotente via tabela import.import_map)
//   2) Importa users → app.users (preserva bcrypt; force MFA setup)
//   3) Importa suspects → app.entities + app.entity_persons (+ foto primária)
//   4) Importa suspect_photos → app.entity_photos (galeria)
//   5) Importa internal_reports → app.reports (+ PDF original como anexo)
//   6) Importa internal_reports_suspects → app.report_qualifications
//   7) Commit + relatório de mídias copiadas/órfãs/faltantes
//
// O bundle deve estar em LEGACY_BUNDLE_PATH com layout:
//
//	<bundle>/
//	  data.sql               (já carregado pelo passo anterior)
//	  media/                 (espelho de storage/app/public/ do Laravel)
//	  pdfs/                  (PDFs originais dos RIs, opcional)
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	idb "github.com/belia/tevunah/backend/internal/db"
)

const (
	sourceTag    = "tevunah-legacy"
	sentinelCode = "SYS-IMPORT"
)

// Importer mantém estado mutável durante uma execução: conexão, transação,
// id do run, id do usuário sentinela, caminho do bundle, contadores e lista
// de cópias pra desfazer em caso de rollback.
type Importer struct {
	db          *sql.DB
	tx          *sql.Tx
	ctx         context.Context
	runID       string
	sentinelID  string
	bundlePath  string
	photoDir    string
	stats       map[string]int
	copiedFiles []string // pra remover se a tx for revertida
	orphans     []string // arquivos em media/ sem registro correspondente
	missing     []string // registros que referenciam arquivos ausentes
}

func main() {
	bundlePath := os.Getenv("LEGACY_BUNDLE_PATH")
	if bundlePath == "" {
		log.Fatal("LEGACY_BUNDLE_PATH não definido (espera-se /mnt/legacy-bundle)")
	}
	photoDir := os.Getenv("PHOTO_DIR")
	if photoDir == "" {
		photoDir = "/var/lib/tevunah/photos"
	}

	// Migrations DSN (role superuser) — precisamos escrever em multi-schema
	// (app.*, import.*) e cleanup do legacy.* fica facilitado.
	dsn := idb.Env("MIGRATIONS_DATABASE_URL", idb.Env("APP_DATABASE_URL", ""))
	if dsn == "" {
		log.Fatal("MIGRATIONS_DATABASE_URL/APP_DATABASE_URL não definido")
	}
	db, err := idb.Open(dsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	imp := &Importer{
		db:         db,
		ctx:        ctx,
		bundlePath: bundlePath,
		photoDir:   photoDir,
		stats:      make(map[string]int),
	}

	// Modo "media-only": pula a transformação de dados e só sincroniza
	// arquivos físicos pra entidades já mapeadas via import_map. Útil quando
	// o usuário fez a importação inicial sem mídias e agora completou o bundle.
	if os.Getenv("IMPORT_MEDIA_ONLY") == "1" {
		if err := imp.runMediaOnly(); err != nil {
			imp.cleanupCopies()
			log.Fatalf("media sync falhou: %v", err)
		}
		imp.report()
		return
	}

	if err := imp.run(); err != nil {
		imp.cleanupCopies()
		log.Fatalf("import falhou: %v", err)
	}
	imp.report()
}

// ─── orquestração ─────────────────────────────────────────────────────────

func (i *Importer) run() error {
	// Resolve o usuário sentinela (criado pela migration 32).
	if err := i.db.QueryRowContext(i.ctx,
		`SELECT id FROM app.users WHERE code = $1`, sentinelCode,
	).Scan(&i.sentinelID); err != nil {
		return fmt.Errorf("sentinela %s: %w", sentinelCode, err)
	}

	// Cria registro do run (fora da tx — pra ficar visível mesmo se rollback).
	if err := i.db.QueryRowContext(i.ctx, `
		INSERT INTO import.import_runs (source, bundle_path, triggered_by)
		VALUES ($1, $2, $3) RETURNING id`,
		sourceTag, i.bundlePath, i.sentinelID,
	).Scan(&i.runID); err != nil {
		return fmt.Errorf("import_runs: %w", err)
	}

	tx, err := i.db.BeginTx(i.ctx, nil)
	if err != nil {
		return err
	}
	i.tx = tx
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	steps := []struct {
		name string
		fn   func() error
	}{
		{"users", i.importUsers},
		{"suspects", i.importSuspects},
		{"suspect_photos", i.importSuspectPhotos},
		{"internal_reports", i.importReports},
		{"internal_reports_suspects", i.importReportQualifications},
	}
	for _, s := range steps {
		log.Printf("▶ %s…", s.name)
		if err := s.fn(); err != nil {
			return fmt.Errorf("%s: %w", s.name, err)
		}
	}

	// Detecta órfãos (arquivos em media/ que não foram referenciados).
	i.scanOrphans()

	// Persiste stats no import_runs.
	if _, err := tx.ExecContext(i.ctx, `
		UPDATE import.import_runs
		   SET finished_at = now(),
		       stats = $1
		 WHERE id = $2`,
		mapToJSONB(i.stats), i.runID,
	); err != nil {
		return fmt.Errorf("update stats: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}

// ─── media-only sync ─────────────────────────────────────────────────────

// runMediaOnly copia arquivos físicos pra entidades já mapeadas via import_map
// sem refazer nenhuma inserção. Tudo dentro de uma transação curta — se o
// disco falhar no meio, as referências no banco ficam consistentes.
func (i *Importer) runMediaOnly() error {
	if err := i.db.QueryRowContext(i.ctx,
		`SELECT id FROM app.users WHERE code = $1`, sentinelCode,
	).Scan(&i.sentinelID); err != nil {
		return fmt.Errorf("sentinela %s: %w", sentinelCode, err)
	}
	if err := i.db.QueryRowContext(i.ctx, `
		INSERT INTO import.import_runs (source, bundle_path, triggered_by)
		VALUES ($1, $2, $3) RETURNING id`,
		sourceTag, i.bundlePath+" (media-only)", i.sentinelID,
	).Scan(&i.runID); err != nil {
		return fmt.Errorf("import_runs: %w", err)
	}

	tx, err := i.db.BeginTx(i.ctx, nil)
	if err != nil {
		return err
	}
	i.tx = tx
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	log.Printf("▶ syncing fotos primárias de suspects…")
	if err := i.syncPrimaryPhotos(); err != nil {
		return fmt.Errorf("primary photos: %w", err)
	}
	log.Printf("▶ syncing galeria…")
	if err := i.syncGalleryPhotos(); err != nil {
		return fmt.Errorf("gallery: %w", err)
	}

	if _, err := tx.ExecContext(i.ctx, `
		UPDATE import.import_runs SET finished_at = now(), stats = $1 WHERE id = $2`,
		mapToJSONB(i.stats), i.runID,
	); err != nil {
		return fmt.Errorf("update stats: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// syncPrimaryPhotos varre legacy.suspects e popula entity_persons.photo_path
// quando: (a) a entidade está mapeada; (b) ainda não tem foto; (c) o arquivo
// existe em bundle/media/. Idempotente — pulamos quem já tem foto.
func (i *Importer) syncPrimaryPhotos() error {
	type row struct {
		legacyID   int64
		entityUUID string
		picPath    string
	}
	rows, err := i.tx.QueryContext(i.ctx, `
		SELECT s.id, m.new_uuid, s.profile_picture_path
		  FROM legacy.suspects s
		  JOIN import.import_map m
		    ON m.source = $1 AND m.legacy_table = 'suspects' AND m.legacy_id = s.id
		  JOIN app.entity_persons ep ON ep.entity_id = m.new_uuid
		 WHERE s.deleted_at IS NULL
		   AND s.profile_picture_path IS NOT NULL
		   AND s.profile_picture_path <> ''
		   AND ep.photo_path IS NULL`, sourceTag)
	if err != nil {
		return err
	}
	var batch []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.legacyID, &r.entityUUID, &r.picPath); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range batch {
		dst, ok := i.copyMedia(r.picPath, r.entityUUID)
		if !ok {
			continue
		}
		if _, err := i.tx.ExecContext(i.ctx, `
			UPDATE app.entity_persons SET photo_path = $1 WHERE entity_id = $2`,
			dst, r.entityUUID,
		); err != nil {
			return err
		}
		i.stats["primary_photos_synced"]++
	}
	return nil
}

// syncGalleryPhotos cria registros em app.entity_photos pras fotos da galeria
// legada (suspect_photos) que ainda não foram mapeadas. Diferente de primary,
// aqui criamos registros novos (não existe linha pra atualizar) — usamos o
// import_map pra evitar duplicação em re-runs.
func (i *Importer) syncGalleryPhotos() error {
	type row struct {
		legacyID   int64
		entityUUID string
		path       string
		desc       sql.NullString
	}
	rows, err := i.tx.QueryContext(i.ctx, `
		SELECT sp.id, m.new_uuid, sp.path, sp.description
		  FROM legacy.suspect_photos sp
		  JOIN import.import_map ms
		    ON ms.source = $1 AND ms.legacy_table = 'suspects' AND ms.legacy_id = sp.suspect_id
		  JOIN import.import_map m
		    ON m.source = $1 AND m.legacy_table = 'suspects' AND m.legacy_id = sp.suspect_id
		  LEFT JOIN import.import_map already
		    ON already.source = $1 AND already.legacy_table = 'suspect_photos' AND already.legacy_id = sp.id
		 WHERE already.new_uuid IS NULL`, sourceTag)
	if err != nil {
		return err
	}
	var batch []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.legacyID, &r.entityUUID, &r.path, &r.desc); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range batch {
		var photoID string
		if err := i.tx.QueryRowContext(i.ctx, `SELECT gen_random_uuid()`).Scan(&photoID); err != nil {
			return err
		}
		dst, ok := i.copyMediaWithUUID(r.path, photoID)
		if !ok {
			continue
		}
		mime := guessMimeByExt(r.path)
		caption := ""
		if r.desc.Valid {
			caption = strings.TrimSpace(r.desc.String)
		}
		if _, err := i.tx.ExecContext(i.ctx, `
			INSERT INTO app.entity_photos
			  (id, entity_id, photo_path, caption, mime, ord, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, 0, $6, $6)`,
			photoID, r.entityUUID, dst, caption, mime, i.sentinelID,
		); err != nil {
			return err
		}
		if err := i.recordMap("suspect_photos", r.legacyID, photoID); err != nil {
			return err
		}
		i.stats["gallery_synced"]++
	}
	return nil
}

// ─── users ────────────────────────────────────────────────────────────────

func (i *Importer) importUsers() error {
	type legacyUser struct {
		id              int64
		name, email, pw string
	}
	var batch []legacyUser
	rows, err := i.tx.QueryContext(i.ctx, `
		SELECT id, name, email, password
		  FROM legacy.users
		 ORDER BY id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var u legacyUser
		if err := rows.Scan(&u.id, &u.name, &u.email, &u.pw); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, u := range batch {
		legacyID, name, email, pwd := u.id, u.name, u.email, u.pw

		if _, hit, err := i.lookupMap("users", legacyID); err != nil {
			return err
		} else if hit {
			i.stats["users_skipped"]++
			continue
		}

		code, err := i.nextUserCode()
		if err != nil {
			return err
		}

		var newID string
		err = i.tx.QueryRowContext(i.ctx, `
			INSERT INTO app.users
			  (code, email, display_name, password_hash,
			   clearance_level, status, must_setup_totp, created_by)
			VALUES ($1, lower($2), $3, $4, 1, 'active', true, $5)
			RETURNING id`,
			code, email, strings.ToUpper(name), pwd, i.sentinelID,
		).Scan(&newID)
		if err != nil {
			return fmt.Errorf("insert user %s: %w", email, err)
		}
		// Papel padrão 'agente' — promoções manuais via admin.
		if _, err := i.tx.ExecContext(i.ctx, `
			INSERT INTO app.user_roles (user_id, role_code, assigned_by)
			VALUES ($1, 'agente', $2)`, newID, i.sentinelID); err != nil {
			return fmt.Errorf("user_roles %s: %w", email, err)
		}
		if err := i.recordMap("users", legacyID, newID); err != nil {
			return err
		}
		i.stats["users_imported"]++
	}
	return nil
}

// ─── suspects → entities ─────────────────────────────────────────────────

func (i *Importer) importSuspects() error {
	// Ignora soft-deletes do legado: o tevunah novo não tem por que herdar
	// linhas marcadas como removidas no Laravel.
	rows, err := i.tx.QueryContext(i.ctx, `
		SELECT id, profile_picture_path, full_name, mother, alias, cpf, orcrim,
		       addresses, vehicles, additional_information,
		       is_dead, is_arrested
		  FROM legacy.suspects
		 WHERE deleted_at IS NULL
		 ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type row struct {
		id                                                    int64
		picPath, name, mother, alias, cpf, orcrim, addresses  sql.NullString
		vehicles, additionalInfo                              sql.NullString
		isDead, isArrested                                    sql.NullBool
	}
	var batch []row
	for rows.Next() {
		var r row
		if err := rows.Scan(
			&r.id, &r.picPath, &r.name, &r.mother, &r.alias, &r.cpf, &r.orcrim,
			&r.addresses, &r.vehicles, &r.additionalInfo,
			&r.isDead, &r.isArrested,
		); err != nil {
			return err
		}
		batch = append(batch, r)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range batch {
		if _, hit, err := i.lookupMap("suspects", r.id); err != nil {
			return err
		} else if hit {
			i.stats["suspects_skipped"]++
			continue
		}

		desc := suspectDescription(r.mother, r.orcrim, r.addresses, r.vehicles, r.additionalInfo)
		name := strings.TrimSpace(r.name.String)
		if name == "" {
			name = "(SEM NOME)"
		}

		var entityID string
		err := i.tx.QueryRowContext(i.ctx, `
			INSERT INTO app.entities
			  (kind, name, description, classification, created_by, updated_by)
			VALUES ('person', $1, $2, 1, $3, $3)
			RETURNING id`,
			strings.ToUpper(name), desc, i.sentinelID,
		).Scan(&entityID)
		if err != nil {
			return fmt.Errorf("entity %d: %w", r.id, err)
		}

		// Foto primária: copia o arquivo e referencia o filename canônico.
		photoFilename := sql.NullString{}
		if r.picPath.Valid && r.picPath.String != "" {
			dst, ok := i.copyMedia(r.picPath.String, entityID)
			if ok {
				photoFilename = sql.NullString{String: dst, Valid: true}
				i.stats["suspect_photos_primary"]++
			}
		}

		aliasArr := nullStringToArray(r.alias)
		if _, err := i.tx.ExecContext(i.ctx, `
			INSERT INTO app.entity_persons
			  (entity_id, aliases, mother_name, cpf, photo_path)
			VALUES ($1, $2, $3, $4, $5)`,
			entityID, aliasArr, nullableUpper(r.mother), normalizeCPF(r.cpf), photoFilename,
		); err != nil {
			return fmt.Errorf("entity_persons %d: %w", r.id, err)
		}

		// Flags is_dead/is_arrested → tags livres.
		if r.isDead.Bool {
			if err := i.addTag(entityID, "falecido"); err != nil {
				return err
			}
		}
		if r.isArrested.Bool {
			if err := i.addTag(entityID, "preso"); err != nil {
				return err
			}
		}

		if err := i.recordMap("suspects", r.id, entityID); err != nil {
			return err
		}
		i.stats["suspects_imported"]++
	}
	return nil
}

// suspectDescription concatena addresses/vehicles/additional_information em
// uma descrição textual estruturada. Cada campo do legado é HTML — limpamos
// tags e decodificamos entidades pra texto plano legível.
func suspectDescription(mother, orcrim, addresses, vehicles, addInfo sql.NullString) string {
	var sb strings.Builder
	addSection := func(title string, v sql.NullString) {
		if !v.Valid || strings.TrimSpace(v.String) == "" {
			return
		}
		txt := strings.TrimSpace(htmlToText(v.String))
		if txt == "" {
			return
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(title)
		sb.WriteString(":\n")
		sb.WriteString(txt)
	}
	if orcrim.Valid && strings.TrimSpace(orcrim.String) != "" {
		addSection("ORCRIM", orcrim)
	}
	addSection("ENDEREÇOS", addresses)
	addSection("VEÍCULOS", vehicles)
	addSection("NOTAS", addInfo)
	_ = mother // já vai pro mother_name; não duplica aqui
	return sb.String()
}

func (i *Importer) addTag(entityID, tag string) error {
	_, err := i.tx.ExecContext(i.ctx, `
		INSERT INTO app.entity_tags (entity_id, tag, added_by)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`,
		entityID, tag, i.sentinelID)
	return err
}

// ─── suspect_photos → entity_photos ──────────────────────────────────────

func (i *Importer) importSuspectPhotos() error {
	type photo struct {
		id, suspectID int64
		path          string
		desc          sql.NullString
	}
	var batch []photo
	rows, err := i.tx.QueryContext(i.ctx, `
		SELECT id, suspect_id, path, description
		  FROM legacy.suspect_photos
		 ORDER BY id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var p photo
		if err := rows.Scan(&p.id, &p.suspectID, &p.path, &p.desc); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, p := range batch {
		id, suspectID, path, desc := p.id, p.suspectID, p.path, p.desc

		if _, hit, err := i.lookupMap("suspect_photos", id); err != nil {
			return err
		} else if hit {
			i.stats["gallery_skipped"]++
			continue
		}

		entityUUID, ok, err := i.lookupMap("suspects", suspectID)
		if err != nil {
			return err
		}
		if !ok {
			log.Printf("  gallery photo %d aponta para suspect %d não mapeado", id, suspectID)
			continue
		}

		// Gera UUID novo pra galeria + copia arquivo com novo nome.
		var photoID string
		err = i.tx.QueryRowContext(i.ctx, `
			SELECT gen_random_uuid()`).Scan(&photoID)
		if err != nil {
			return err
		}
		dst, copied := i.copyMediaWithUUID(path, photoID)
		if !copied {
			i.missing = append(i.missing, fmt.Sprintf("gallery %d: %s", id, path))
			continue
		}

		mime := guessMimeByExt(path)
		caption := ""
		if desc.Valid {
			caption = strings.TrimSpace(desc.String)
		}
		_, err = i.tx.ExecContext(i.ctx, `
			INSERT INTO app.entity_photos
			  (id, entity_id, photo_path, caption, mime, ord, created_by, updated_by)
			VALUES ($1, $2, $3, $4, $5, 0, $6, $6)`,
			photoID, entityUUID, dst, caption, mime, i.sentinelID)
		if err != nil {
			return fmt.Errorf("entity_photos %d: %w", id, err)
		}
		if err := i.recordMap("suspect_photos", id, photoID); err != nil {
			return err
		}
		i.stats["gallery_imported"]++
	}
	return nil
}

// ─── internal_reports → reports ──────────────────────────────────────────

func (i *Importer) importReports() error {
	type legacyReport struct {
		id                              int64
		reportNum, reportYear           string
		subject, content, confLegacy    string
		filename, ref                   sql.NullString
		reportDate                      time.Time
		diffusionDate                   sql.NullTime
		legacyAuthorID                  sql.NullInt64
	}
	var batch []legacyReport
	rows, err := i.tx.QueryContext(i.ctx, `
		SELECT id, report_number, report_year, filename, report_date, diffusion_date,
		       subject, reference, content, confidentiality_level,
		       generated_by_user_id
		  FROM legacy.internal_reports
		 WHERE deleted_at IS NULL
		 ORDER BY id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var r legacyReport
		if err := rows.Scan(
			&r.id, &r.reportNum, &r.reportYear, &r.filename, &r.reportDate, &r.diffusionDate,
			&r.subject, &r.ref, &r.content, &r.confLegacy, &r.legacyAuthorID,
		); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// Origem: o legado não armazenava origem (era texto fixo no template do RI),
	// então carimbamos o mesmo valor usado como default dos RIs novos — o
	// document_title configurado em system_settings (Admin → Título p/ Documentos).
	var origin string
	if err := i.tx.QueryRowContext(i.ctx,
		`SELECT COALESCE(document_title, '') FROM app.system_settings WHERE key = 'singleton'`,
	).Scan(&origin); err != nil {
		return fmt.Errorf("ler document_title p/ origem: %w", err)
	}
	origin = strings.TrimSpace(origin)

	for _, r := range batch {
		legacyID := r.id
		reportNum, reportYear := r.reportNum, r.reportYear
		subject, content, confLegacy := r.subject, r.content, r.confLegacy
		filename, ref := r.filename, r.ref
		reportDate := r.reportDate
		diffusionDate := r.diffusionDate
		legacyAuthorID := r.legacyAuthorID

		if _, hit, err := i.lookupMap("internal_reports", legacyID); err != nil {
			return err
		} else if hit {
			i.stats["reports_skipped"]++
			continue
		}

		// Autor: tenta mapear o do legado; fallback pro sentinela.
		authorID := i.sentinelID
		if legacyAuthorID.Valid {
			if mapped, ok, err := i.lookupMap("users", legacyAuthorID.Int64); err != nil {
				return err
			} else if ok {
				authorID = mapped
			}
		}

		// Status: difundido se diffusion_date preenchido, senão criado.
		// Seq/year são preservados sempre que o legado tiver — incluindo
		// para 'criado'. No fluxo normal do app novo a numeração é alocada
		// na transição criado→difundido, mas no legado existem rascunhos
		// com número reservado que não chegaram a difundir; Diffuse já
		// preserva seq/year pré-existentes (caminho de re-difusão), então
		// quando esses RIs forem efetivamente difundidos manterão o nº legado.
		status := "criado"
		var diffusedAt sql.NullTime
		var diffusedBy sql.NullString
		var seq sql.NullInt64
		var year sql.NullInt64
		if n, err := strconv.Atoi(strings.TrimSpace(reportNum)); err == nil && n > 0 {
			seq = sql.NullInt64{Int64: int64(n), Valid: true}
		}
		if n, err := strconv.Atoi(strings.TrimSpace(reportYear)); err == nil && n > 0 {
			year = sql.NullInt64{Int64: int64(n), Valid: true}
		}
		if diffusionDate.Valid {
			status = "difundido"
			diffusedAt = sql.NullTime{Time: diffusionDate.Time, Valid: true}
			diffusedBy = sql.NullString{String: authorID, Valid: true}
		}

		conf := mapConfidentiality(confLegacy)
		body := html.UnescapeString(content)
		refStr := ""
		if ref.Valid {
			yr := 0
			if year.Valid {
				yr = int(year.Int64)
			}
			refStr = formatLegacyReference(ref.String, yr)
		}

		var reportID string
		err := i.tx.QueryRowContext(i.ctx, `
			INSERT INTO app.reports
			  (status, seq, year, doc_date, subject, origin, reference,
			   body_html, confidentiality,
			   created_at, created_by, updated_at, updated_by,
			   diffused_at, diffused_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9,
			        now(), $10, now(), $10, $11, $12)
			RETURNING id`,
			status, seq, year, reportDate, strings.TrimSpace(subject), origin, refStr,
			body, conf,
			authorID, diffusedAt, diffusedBy,
		).Scan(&reportID)
		if err != nil {
			return fmt.Errorf("insert report %d: %w", legacyID, err)
		}

		// PDF original do legado vira anexo (preserva o documento exato).
		if filename.Valid && strings.TrimSpace(filename.String) != "" {
			if err := i.attachLegacyPDF(reportID, filename.String, authorID); err != nil {
				log.Printf("  pdf anexo report %d: %v", legacyID, err)
			}
		}

		if err := i.recordMap("internal_reports", legacyID, reportID); err != nil {
			return err
		}
		i.stats["reports_imported"]++
	}
	return nil
}

func (i *Importer) attachLegacyPDF(reportID, legacyPath, uploadedBy string) error {
	src := filepath.Join(i.bundlePath, "pdfs", legacyPath)
	info, err := os.Stat(src)
	if err != nil {
		i.missing = append(i.missing, fmt.Sprintf("pdf: %s", legacyPath))
		return nil
	}

	// Cria id novo + filename canônico.
	var attID string
	if err := i.tx.QueryRowContext(i.ctx, `SELECT gen_random_uuid()`).Scan(&attID); err != nil {
		return err
	}
	ext := strings.ToLower(filepath.Ext(legacyPath))
	if ext == "" {
		ext = ".pdf"
	}
	dstFilename := attID + ext
	dstDir := filepath.Join(i.photoDir, "report-attachments")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	dstFull := filepath.Join(dstDir, dstFilename)
	if err := copyFile(src, dstFull); err != nil {
		return err
	}
	i.copiedFiles = append(i.copiedFiles, dstFull)

	original := filepath.Base(legacyPath)
	_, err = i.tx.ExecContext(i.ctx, `
		INSERT INTO app.report_attachments
		  (id, report_id, filename, original_name, mime, size_bytes, uploaded_by)
		VALUES ($1, $2, $3, $4, 'application/pdf', $5, $6)`,
		attID, reportID, dstFilename, original, info.Size(), uploadedBy)
	if err != nil {
		return err
	}
	i.stats["attachments_imported"]++
	return nil
}

// ─── internal_reports_suspects → report_qualifications ───────────────────

func (i *Importer) importReportQualifications() error {
	type qual struct {
		id, reportLegacy, suspectLegacy int64
	}
	var batch []qual
	rows, err := i.tx.QueryContext(i.ctx, `
		SELECT id, internal_report_id, suspect_id
		  FROM legacy.internal_reports_suspects
		 ORDER BY internal_report_id, id`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var q qual
		if err := rows.Scan(&q.id, &q.reportLegacy, &q.suspectLegacy); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, q)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	ord := make(map[string]int) // ord crescente por report_id
	for _, q := range batch {
		id, reportLegacy, suspectLegacy := q.id, q.reportLegacy, q.suspectLegacy

		if _, hit, err := i.lookupMap("internal_reports_suspects", id); err != nil {
			return err
		} else if hit {
			continue
		}

		reportUUID, ok1, err := i.lookupMap("internal_reports", reportLegacy)
		if err != nil {
			return err
		}
		entityUUID, ok2, err := i.lookupMap("suspects", suspectLegacy)
		if err != nil {
			return err
		}
		if !ok1 || !ok2 {
			continue
		}

		// data: snapshot da entidade no formato que o frontend espera
		// (mesmas chaves usadas por QualificationPicker no fluxo normal).
		snapshot, err := i.qualificationSnapshot(entityUUID)
		if err != nil {
			return fmt.Errorf("snapshot qual %d: %w", id, err)
		}

		var qualID string
		err = i.tx.QueryRowContext(i.ctx, `
			INSERT INTO app.report_qualifications
			  (report_id, ord, kind, entity_id, data)
			VALUES ($1, $2, 'civil', $3, $4::jsonb)
			RETURNING id`,
			reportUUID, ord[reportUUID], entityUUID, snapshot,
		).Scan(&qualID)
		if err != nil {
			return fmt.Errorf("qualification %d: %w", id, err)
		}
		ord[reportUUID]++
		if err := i.recordMap("internal_reports_suspects", id, qualID); err != nil {
			return err
		}
		i.stats["qualifications_imported"]++
	}
	return nil
}

// qualificationSnapshot lê a entidade pessoa e devolve o JSON do snapshot
// que vai pro campo data da qualificação. Chaves replicam o formato emitido
// pelo QualificationPicker do frontend pra que o renderer exiba nome/alias/etc.
// O JSON é montado direto no Postgres pra evitar manuseio de text[] em Go.
func (i *Importer) qualificationSnapshot(entityID string) (string, error) {
	var snapshot string
	err := i.tx.QueryRowContext(i.ctx, `
		SELECT jsonb_build_object(
		         'nome',            e.name,
		         'aliases',         COALESCE(to_jsonb(ep.aliases), '[]'::jsonb),
		         'genero',          COALESCE(ep.gender, ''),
		         'data_nascimento', COALESCE(to_char(ep.date_of_birth, 'YYYY-MM-DD'), ''),
		         'nome_mae',        COALESCE(ep.mother_name, ''),
		         'cpf',             COALESCE(ep.cpf, '')
		       )::text
		  FROM app.entities e
		  JOIN app.entity_persons ep ON ep.entity_id = e.id
		 WHERE e.id = $1`, entityID,
	).Scan(&snapshot)
	return snapshot, err
}

// ─── helpers de media ────────────────────────────────────────────────────

// copyMedia copia o arquivo apontado por legacyPath (relativo a bundle/media/)
// pra PHOTO_DIR/<entityID>.<ext>. Devolve (filename canônico, sucesso).
func (i *Importer) copyMedia(legacyPath, entityID string) (string, bool) {
	src := filepath.Join(i.bundlePath, "media", legacyPath)
	if _, err := os.Stat(src); err != nil {
		i.missing = append(i.missing, fmt.Sprintf("photo: %s", legacyPath))
		return "", false
	}
	ext := strings.ToLower(filepath.Ext(legacyPath))
	if ext == "" {
		ext = ".jpg"
	}
	dstFilename := entityID + ext
	dstFull := filepath.Join(i.photoDir, dstFilename)
	if err := copyFile(src, dstFull); err != nil {
		log.Printf("  copy %s → %s: %v", legacyPath, dstFilename, err)
		return "", false
	}
	i.copiedFiles = append(i.copiedFiles, dstFull)
	return dstFilename, true
}

func (i *Importer) copyMediaWithUUID(legacyPath, newUUID string) (string, bool) {
	src := filepath.Join(i.bundlePath, "media", legacyPath)
	if _, err := os.Stat(src); err != nil {
		return "", false
	}
	ext := strings.ToLower(filepath.Ext(legacyPath))
	if ext == "" {
		ext = ".jpg"
	}
	dstFilename := newUUID + ext
	dstFull := filepath.Join(i.photoDir, dstFilename)
	if err := copyFile(src, dstFull); err != nil {
		return "", false
	}
	i.copiedFiles = append(i.copiedFiles, dstFull)
	return dstFilename, true
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func (i *Importer) cleanupCopies() {
	for _, p := range i.copiedFiles {
		_ = os.Remove(p)
	}
}

// scanOrphans varre bundle/media/ e bundle/pdfs/ buscando arquivos que não
// estão referenciados em nenhum registro legado importado. Usa as colunas
// de path do legado pra montar o set de "esperados".
func (i *Importer) scanOrphans() {
	refs := make(map[string]struct{})
	addRef := func(p string) {
		p = strings.TrimSpace(p)
		if p != "" {
			refs[p] = struct{}{}
		}
	}
	gather := func(q string) {
		rows, err := i.tx.QueryContext(i.ctx, q)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var p sql.NullString
			if err := rows.Scan(&p); err == nil && p.Valid {
				addRef(p.String)
			}
		}
	}
	gather(`SELECT profile_picture_path FROM legacy.suspects`)
	gather(`SELECT path FROM legacy.suspect_photos`)
	gather(`SELECT filename FROM legacy.internal_reports`)

	walk := func(subdir string) {
		root := filepath.Join(i.bundlePath, subdir)
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			if _, ok := refs[rel]; !ok {
				i.orphans = append(i.orphans, filepath.Join(subdir, rel))
			}
			return nil
		})
	}
	walk("media")
	walk("pdfs")
}

// ─── import_map helpers ──────────────────────────────────────────────────

func (i *Importer) lookupMap(table string, legacyID int64) (string, bool, error) {
	var uuid string
	err := i.tx.QueryRowContext(i.ctx, `
		SELECT new_uuid FROM import.import_map
		 WHERE source = $1 AND legacy_table = $2 AND legacy_id = $3`,
		sourceTag, table, legacyID,
	).Scan(&uuid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return uuid, true, nil
}

func (i *Importer) recordMap(table string, legacyID int64, newUUID string) error {
	_, err := i.tx.ExecContext(i.ctx, `
		INSERT INTO import.import_map (source, legacy_table, legacy_id, new_uuid, run_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT DO NOTHING`,
		sourceTag, table, legacyID, newUUID, i.runID)
	return err
}

// ─── helpers diversos ────────────────────────────────────────────────────

func (i *Importer) nextUserCode() (string, error) {
	// Mesmo formato do gerador oficial (users.GenerateCode): 4 dígitos
	// aleatórios, únicos. random() do Postgres + checagem de unicidade dentro
	// da transação (enxerga inserts não-commitados do próprio run).
	for attempt := 0; attempt < 50; attempt++ {
		var code string
		err := i.tx.QueryRowContext(i.ctx, `
			SELECT g.c FROM (SELECT lpad((floor(random() * 10000))::int::text, 4, '0') AS c) g
			 WHERE NOT EXISTS (SELECT 1 FROM app.users WHERE code = g.c)`).Scan(&code)
		if err == sql.ErrNoRows {
			continue // colisão — tenta outro
		}
		if err != nil {
			return "", err
		}
		return code, nil
	}
	return "", fmt.Errorf("não foi possível gerar código de agente único após 50 tentativas")
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func htmlToText(s string) string {
	// 1) remove tags HTML; 2) decodifica entidades; 3) compacta whitespace.
	s = htmlTagRe.ReplaceAllString(s, "\n")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\r", "")
	// Compacta múltiplas linhas em branco em uma só.
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

// formatLegacyReference converte o campo `reference` do legado (JSON
// stringified array, ex.: `["49","18"]` ou `[]`) no formato esperado pelo
// app novo: "RI Nº 49/AAAA; RI Nº 18/AAAA". Vazio vira "". Ano usado é o
// do próprio RI (assunção pragmática — o legado não guardava o ano de cada
// referência, mas na prática o user referenciava RIs do mesmo ano).
func formatLegacyReference(raw string, year int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "null" {
		return ""
	}
	var nums []string
	if err := json.Unmarshal([]byte(raw), &nums); err != nil {
		// Fallback: se não é JSON parseável, devolve cru — melhor que perder.
		return raw
	}
	parts := make([]string, 0, len(nums))
	for _, n := range nums {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if year > 0 {
			parts = append(parts, fmt.Sprintf("RI Nº %s/%d", n, year))
		} else {
			parts = append(parts, fmt.Sprintf("RI Nº %s", n))
		}
	}
	return strings.Join(parts, "; ")
}

func mapConfidentiality(legacy string) string {
	switch strings.ToLower(strings.TrimSpace(legacy)) {
	case "reservado", "sigiloso":
		return "sigiloso"
	case "secreto":
		return "secreto"
	case "ultrassecreto":
		return "ultrassecreto"
	default:
		return "sigiloso"
	}
}

var nonDigitsRe = regexp.MustCompile(`\D`)

func normalizeCPF(v sql.NullString) sql.NullString {
	if !v.Valid {
		return v
	}
	digits := nonDigitsRe.ReplaceAllString(v.String, "")
	if digits == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: digits, Valid: true}
}

func nullStr(v sql.NullString) sql.NullString {
	if !v.Valid {
		return v
	}
	return sql.NullString{String: strings.TrimSpace(v.String), Valid: true}
}

func nullableUpper(v sql.NullString) sql.NullString {
	if !v.Valid {
		return v
	}
	t := strings.TrimSpace(v.String)
	if t == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: strings.ToUpper(t), Valid: true}
}

// nullStringToArray transforma um alias único (campo varchar do legado) num
// Postgres text[] de um elemento, ou array vazio se nulo.
func nullStringToArray(v sql.NullString) string {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return "{}"
	}
	// Escape simples pra literal de array — aliases legados são curtos e sem
	// vírgulas/aspas; mesmo assim escapamos backslash e aspas dobradas.
	a := strings.ReplaceAll(strings.TrimSpace(v.String), `\`, `\\`)
	a = strings.ReplaceAll(a, `"`, `\"`)
	return `{"` + strings.ToUpper(a) + `"}`
}

func guessMimeByExt(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func mapToJSONB(m map[string]int) string {
	if len(m) == 0 {
		return "{}"
	}
	var parts []string
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%q:%d", k, v))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// ─── relatório final ─────────────────────────────────────────────────────

func (i *Importer) report() {
	fmt.Println()
	fmt.Println("✓ Importação concluída")
	fmt.Println("─────────────────────────────────────────")
	keys := []string{
		"users_imported", "users_skipped",
		"suspects_imported", "suspects_skipped",
		"suspect_photos_primary",
		"gallery_imported", "gallery_skipped",
		"reports_imported", "reports_skipped",
		"attachments_imported",
		"qualifications_imported",
		"primary_photos_synced", "gallery_synced",
	}
	for _, k := range keys {
		if v, ok := i.stats[k]; ok && v > 0 {
			fmt.Printf("  %-26s %d\n", k, v)
		}
	}
	fmt.Println("─────────────────────────────────────────")
	if len(i.missing) > 0 {
		fmt.Printf("⚠  %d arquivos referenciados não encontrados no bundle:\n", len(i.missing))
		for _, m := range i.missing[:min(20, len(i.missing))] {
			fmt.Printf("    %s\n", m)
		}
		if len(i.missing) > 20 {
			fmt.Printf("    … (+%d)\n", len(i.missing)-20)
		}
	}
	if len(i.orphans) > 0 {
		fmt.Printf("ℹ  %d arquivos em media/pdfs sem registro correspondente:\n", len(i.orphans))
		for _, m := range i.orphans[:min(10, len(i.orphans))] {
			fmt.Printf("    %s\n", m)
		}
		if len(i.orphans) > 10 {
			fmt.Printf("    … (+%d)\n", len(i.orphans)-10)
		}
	}
	fmt.Printf("\nrun_id: %s\n", i.runID)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
