package opsreport

import (
	"context"
	"database/sql"
	"strings"
	"unicode"

	"github.com/belia/tevunah/backend/internal/sipom"
)

// IntelOfficer é um agente da agência na composição da ocorrência, com o
// tipo de policiamento Inteligência. Os dados funcionais vêm do cadastro.
type IntelOfficer struct {
	UserID       string
	Name         string // nome de exibição do agente
	Registration string // matrícula
	RankID       *int   // sipom.postos_graduacoes.id
	Number       string
	WarName      string
	FuncaoID     int // sipom.FuncaoGestor | FuncaoAgente | FuncaoAnalista
}

// IntelMember é a escolha do analista: o agente e a função.
type IntelMember struct {
	UserID   string
	FuncaoID int
}

// Função padrão pelo papel: analista é Analista, gestor é Gestor, os demais
// são Agente.
const intelFuncaoDefault = `
	CASE WHEN EXISTS (SELECT 1 FROM app.user_roles ur WHERE ur.user_id = u.id AND ur.role_code = 'analista') THEN 12
	     WHEN EXISTS (SELECT 1 FROM app.user_roles ur WHERE ur.user_id = u.id AND ur.role_code = 'gestor') THEN 10
	     ELSE 11 END`

// Ordem da composição automática: posto mais alto primeiro.
const intelRankOrder = `
	(SELECT p.ordem FROM sipom.postos_graduacoes p WHERE p.id = u.rank_id) DESC NULLS LAST, u.war_name, u.display_name`

func scanIntelOfficers(rows *sql.Rows) ([]IntelOfficer, error) {
	defer rows.Close()
	out := []IntelOfficer{}
	for rows.Next() {
		var x IntelOfficer
		if err := rows.Scan(&x.UserID, &x.Name, &x.Registration, &x.RankID, &x.Number, &x.WarName, &x.FuncaoID); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// IntelCandidates são os agentes que entram na composição da inteligência:
// ativos, com matrícula e marcados no cadastro. Sem lista gravada, é a
// composição da ocorrência; com ela, é de onde o analista escolhe.
func (r *Repo) IntelCandidates(ctx context.Context) ([]IntelOfficer, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.display_name, u.registration, u.rank_id, u.number, u.war_name, `+intelFuncaoDefault+`
		  FROM app.users u
		 WHERE u.status = 'active' AND u.sipom_composition AND u.registration <> ''
		 ORDER BY `+intelRankOrder)
	if err != nil {
		return nil, err
	}
	return scanIntelOfficers(rows)
}

// intelComposition carrega a composição da inteligência da ocorrência: a
// lista gravada ou, sem ela, os agentes marcados no cadastro.
func (r *Repo) intelComposition(ctx context.Context, id string) (manual bool, list []IntelOfficer, err error) {
	if err = r.db.QueryRowContext(ctx,
		`SELECT intel_composition_manual FROM app.ops_occurrences WHERE id = $1`, id).Scan(&manual); err != nil {
		return false, nil, err
	}
	if !manual {
		list, err = r.IntelCandidates(ctx)
		return false, list, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, u.display_name, u.registration, u.rank_id, u.number, u.war_name, i.funcao_id
		  FROM app.ops_occurrence_intel_officers i
		  JOIN app.users u ON u.id = i.user_id
		 WHERE i.occurrence_id = $1
		 ORDER BY i.position`, id)
	if err != nil {
		return true, nil, err
	}
	list, err = scanIntelOfficers(rows)
	return true, list, err
}

// SetIntelComposition grava a composição da inteligência escolhida (na
// ordem da lista) ou, com nil, devolve a ocorrência ao cadastro dos agentes.
func (r *Repo) SetIntelComposition(ctx context.Context, id string, members []IntelMember) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		UPDATE app.ops_occurrences SET intel_composition_manual = $2
		 WHERE id = $1 AND deleted_at IS NULL`, id, members != nil)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM app.ops_occurrence_intel_officers WHERE occurrence_id = $1`, id); err != nil {
		return err
	}
	for k, m := range members {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO app.ops_occurrence_intel_officers (occurrence_id, user_id, position, funcao_id)
			VALUES ($1, $2, $3, $4)`, id, m.UserID, k, m.FuncaoID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// FreezeIntelComposition grava a composição automática como lista da
// ocorrência — no envio, para que um reenvio (ou um agente que entre ou saia
// da agência depois) não mude quem constou no SIPOM.
func (r *Repo) FreezeIntelComposition(ctx context.Context, so *StoredOccurrence) error {
	if !so.IntelParticipation || so.IntelCompositionManual {
		return nil
	}
	members := make([]IntelMember, 0, len(so.IntelOfficers))
	for _, x := range so.IntelOfficers {
		members = append(members, IntelMember{UserID: x.UserID, FuncaoID: x.FuncaoID})
	}
	return r.SetIntelComposition(ctx, so.ID, members)
}

// intelPayload é a composição da inteligência como vai ao SIPOM: quem tem
// matrícula e ainda não está na guarnição do relatório.
func intelPayload(so *StoredOccurrence, seen map[string]bool) []sipom.PayloadComposicao {
	if !so.IntelParticipation {
		return nil
	}
	var out []sipom.PayloadComposicao
	for _, x := range so.IntelOfficers {
		k := RegistrationKey(x.Registration)
		if k == "" || seen[k] || !sipom.IntelFuncao(x.FuncaoID) {
			continue
		}
		seen[k] = true
		out = append(out, sipom.PayloadComposicao{
			ID: "intel-" + x.UserID, Matricula: strings.TrimSpace(x.Registration),
			PoliciamentoTipoID: sipom.PoliciamentoInteligencia, FuncaoID: x.FuncaoID,
		})
	}
	return out
}

// RegistrationKey reduz a matrícula aos dígitos (e ao X do dígito
// verificador), para reconhecer o mesmo policial escrito com ou sem hífen
// ("3055681-X" e "3055681X").
func RegistrationKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if unicode.IsDigit(r) || r == 'X' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
