package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/pdf"
	"github.com/belia/tevunah/backend/internal/reports"
)

// GET /api/reports/{id}/download
//
// Gera o PDF do relatório via wkhtmltopdf, registra o download em
// app.report_downloads (forense — vincula user/session/IP ao sha256 do PDF)
// e devolve os bytes com Content-Disposition: attachment.
//
// Drafts (status='criado') também podem ser baixados — saem com watermark
// RASCUNHO e sem número. Útil pra revisão antes da difusão.
func (a *app) handleReportDownload(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.download") {
		return
	}
	id := r.PathValue("id")
	me := middleware.UserFrom(r.Context())
	sess := middleware.SessionFrom(r.Context())

	rep, err := a.reports.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
			return
		}
		log.Printf("reports download find: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar relatório")
		return
	}

	// Reforça filtro de visibilidade — mesmo critério da listagem/detalhe.
	// Sem isso, alguém com URL conhecida do RI bypassa a restrição.
	isAdmin := hasRole(me.Roles, "administrador")
	canAccess, err := a.reports.CanAccess(r.Context(), id, me.ID, isAdmin)
	if err != nil {
		log.Printf("reports download access: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao verificar acesso")
		return
	}
	if !canAccess {
		httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
		return
	}

	quals, err := a.reports.ListQualifications(r.Context(), id)
	if err != nil {
		log.Printf("reports download list qualif: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar qualificações")
		return
	}

	data := a.buildReportData(r.Context(), rep, quals, me.DisplayName)
	data.AgentCode = me.Code

	pdfBytes, err := a.pdf.Render(r.Context(), data)
	if err != nil {
		log.Printf("reports pdf render: %v", err)
		httpx.Error(w, http.StatusBadGateway, "falha ao gerar PDF")
		return
	}

	sum := sha256.Sum256(pdfBytes)
	hashHex := hex.EncodeToString(sum[:])

	tokenHash := ""
	if sess != nil && sess.Token != "" {
		h := sha256.Sum256([]byte(sess.Token))
		tokenHash = hex.EncodeToString(h[:])
	}
	aid, sid, ip, ua := a.actorInfo(r)
	ipStr, uaStr := "", ""
	if ip != nil {
		ipStr = *ip
	}
	if ua != nil {
		uaStr = *ua
	}

	dlID, err := a.reports.RecordDownload(r.Context(), reports.NewDownload{
		ReportID:         id,
		UserID:           me.ID,
		SessionTokenHash: tokenHash,
		IP:               ipStr,
		UserAgent:        uaStr,
		PDFSha256:        hashHex,
	})
	if err != nil {
		log.Printf("record download: %v", err)
		// Não bloqueia a entrega — o download ainda é auditado abaixo.
	}

	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.download",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(id),
		After: map[string]any{
			"download_id": dlID,
			"sha256":      hashHex,
			"status":      rep.Status,
			"number":      rep.Number(),
			"bytes":       len(pdfBytes),
		},
	})

	filename := pdfFilename(rep)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Download-ID", dlID)
	w.Header().Set("X-PDF-SHA256", hashHex)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

// pdfFilename gera um nome de arquivo legível. Difundido vira "RI_NN_AAAA.pdf",
// rascunho vira "RI_RASCUNHO_<id8>.pdf".
func pdfFilename(rep *reports.Report) string {
	if rep.Seq != nil && rep.Year != nil {
		return fmt.Sprintf("RI_%02d_%d.pdf", *rep.Seq, *rep.Year)
	}
	short := rep.ID
	if len(short) > 8 {
		short = short[:8]
	}
	return fmt.Sprintf("RI_RASCUNHO_%s.pdf", short)
}

func (a *app) buildReportData(ctx context.Context, rep *reports.Report, quals []reports.Qualification, generatedBy string) pdf.ReportData {
	d := pdf.ReportData{
		Number:          rep.Number(),
		Status:          rep.Status,
		IsDraft:         rep.Status != reports.StatusDifundido,
		DocDateBR:       rep.DocDate.Format("02/01/2006"),
		DocDateShort:    docDateShort(rep.DocDate),
		Subject:         rep.Subject,
		Origin:          rep.Origin,
		Diffusion:       rep.Diffusion,
		PriorDiffusion:  rep.PriorDiffusion,
		Reference:       rep.Reference,
		Attachments:     rep.Attachments,
		Classification:  classificationDisplay(rep.Confidentiality),
		BodyHTML:        template.HTML(rep.BodyHTML), //nolint:gosec // editor confiável (autor autenticado)
		GeneratedAtBR:   time.Now().Format("02/01/2006 15:04"),
		GeneratedByName: generatedBy,
	}
	if rep.Number() != "" {
		d.DownloadID = "RI Nº " + rep.Number()
	} else {
		d.DownloadID = "RI · RASCUNHO"
	}

	for i := range quals {
		q := &quals[i]
		qd := pdf.QualificationData{
			Kind:   strings.ToUpper(q.Kind),
			Source: q.Source,
			Name:   qualificationName(q),
		}
		switch q.Kind {
		case reports.QualCivil:
			qd.Fields = a.buildCivilFields(ctx, q)
			qd.PhotoData = template.URL(a.civilPhoto(ctx, q))
		case reports.QualMilitar:
			qd.Fields = buildMilitarFields(q)
			if pp, ok := q.Data["photo_path"].(string); ok {
				qd.PhotoData = template.URL(a.pdf.InlinePhoto(pp))
			}
		}
		d.Qualifications = append(d.Qualifications, qd)
	}

	return d
}

// classificationDisplay formata o nível de confidencialidade do RI no estilo
// "L E T R A S   E S P A Ç A D A S" usado no header/footer (substitui o
// antigo "S E C R E T O" hardcoded). Letra qualquer fora dos níveis conhecidos
// cai em "S E C R E T O" como fallback seguro.
func classificationDisplay(level string) string {
	upper := ""
	switch strings.ToLower(strings.TrimSpace(level)) {
	case reports.ConfidentialitySigiloso:
		upper = "SIGILOSO"
	case reports.ConfidentialityUltrassecreto:
		upper = "ULTRASSECRETO"
	default:
		upper = "SECRETO"
	}
	// Insere um espaço entre cada caractere (mantém visual do template original).
	var b strings.Builder
	for i, c := range upper {
		if i > 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// docDateShort formata DDMmmAA em CAIXA, ex.: 16JAN25 — como o template.
func docDateShort(t time.Time) string {
	monthsPT := []string{"JAN", "FEV", "MAR", "ABR", "MAI", "JUN", "JUL", "AGO", "SET", "OUT", "NOV", "DEZ"}
	return fmt.Sprintf("%02d%s%02d", t.Day(), monthsPT[int(t.Month())-1], t.Year()%100)
}

func qualificationName(q *reports.Qualification) string {
	if s, ok := q.Data["nome"].(string); ok {
		return strings.ToUpper(strings.TrimSpace(s))
	}
	return "(SEM NOME)"
}

// buildMilitarFields segue o template oficial — campos do form militar
// mapeados para os rótulos do RELINT. Itens sem valor saem como "*.*.*"
// no renderer (via {{ if .V }} ... {{ else }}*.*.*{{ end }}).
func buildMilitarFields(q *reports.Qualification) []pdf.KV {
	get := func(k string) string {
		if v, ok := q.Data[k].(string); ok {
			return v
		}
		return ""
	}
	return []pdf.KV{
		{K: "ALCUNHA", V: get("nome_guerra")},
		{K: "CPF", V: get("cpf")},
		{K: "POSTO/GRAD.", V: get("posto")},
		{K: "O.M", V: get("om")},
		{K: "IDENT. MILITAR", V: get("identidade")},
		{K: "INF. ADICIONAIS", V: get("info")},
	}
}

// buildCivilFields produz as 6 linhas do template oficial para qualif civil:
// ALCUNHA / CPF / ORCRIM / ENDEREÇOS / VEÍCULOS / INFORMAÇÕES ADICIONAIS.
// Para ORCRIM, ENDEREÇOS, VEÍCULOS — busca live na entidade vinculada (o
// snapshot do data jsonb não tem isso). Falhas silenciosas (entidade pode ter
// sido deletada após a qualificação): campos saem em branco e o template
// imprime "*.*.*".
func (a *app) buildCivilFields(ctx context.Context, q *reports.Qualification) []pdf.KV {
	get := func(k string) string {
		if v, ok := q.Data[k].(string); ok {
			return v
		}
		return ""
	}
	getList := func(k string) string {
		if v, ok := q.Data[k].([]any); ok {
			parts := make([]string, 0, len(v))
			for _, x := range v {
				if s, ok := x.(string); ok && s != "" {
					parts = append(parts, s)
				}
			}
			return strings.Join(parts, ", ")
		}
		return ""
	}

	alcunha := getList("aliases")
	cpf := get("cpf")
	orcrim := ""
	enderecos := ""
	veiculos := ""

	if q.EntityID != nil && *q.EntityID != "" {
		ent, err := a.entities.FindByID(ctx, *q.EntityID)
		if err == nil && ent.Person != nil {
			if ent.Person.OrcrimName != nil && *ent.Person.OrcrimName != "" {
				if ent.Person.OrcrimAlias != nil && *ent.Person.OrcrimAlias != "" {
					orcrim = fmt.Sprintf("%s (%s)", *ent.Person.OrcrimAlias, *ent.Person.OrcrimName)
				} else {
					orcrim = *ent.Person.OrcrimName
				}
			}
			// Endereços
			if addrs, aerr := a.entities.ListAddresses(ctx, ent.ID); aerr == nil {
				lines := make([]string, 0, len(addrs))
				for i := range addrs {
					lines = append(lines, formatAddress(&addrs[i]))
				}
				enderecos = strings.Join(lines, " · ")
			}
			// Veículos via links
			if links, lerr := a.entities.ListLinksForEntity(ctx, ent.ID); lerr == nil {
				vs := []string{}
				for _, lw := range links {
					var v *entities.VehicleSummary
					if lw.FromVehicle != nil {
						v = lw.FromVehicle
					}
					if lw.ToVehicle != nil {
						v = lw.ToVehicle
					}
					if v == nil {
						continue
					}
					vs = append(vs, formatVehicle(v))
				}
				veiculos = strings.Join(vs, " · ")
			}
		} else if err != nil && !errors.Is(err, entities.ErrNotFound) {
			log.Printf("qual civil enrich: %v", err)
		}
	}

	return []pdf.KV{
		{K: "ALCUNHA", V: alcunha},
		{K: "CPF", V: cpf},
		{K: "ORCRIM", V: orcrim},
		{K: "ENDEREÇOS", V: enderecos},
		{K: "VEÍCULOS", V: veiculos},
		{K: "INF. ADICIONAIS", V: get("info")},
	}
}

func (a *app) civilPhoto(ctx context.Context, q *reports.Qualification) string {
	if q.EntityID == nil || *q.EntityID == "" {
		return ""
	}
	ent, err := a.entities.FindByID(ctx, *q.EntityID)
	if err != nil || ent.Person == nil || ent.Person.PhotoPath == nil {
		return ""
	}
	return a.pdf.InlinePhoto(*ent.Person.PhotoPath)
}

func formatAddress(a *entities.PersonAddress) string {
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return strings.TrimSpace(*p)
	}
	parts := []string{}
	street := str(a.Street)
	num := str(a.Number)
	if street != "" {
		if num != "" {
			parts = append(parts, street+", "+num)
		} else {
			parts = append(parts, street)
		}
	}
	if v := str(a.Complement); v != "" {
		parts = append(parts, v)
	}
	if v := str(a.Neighborhood); v != "" {
		parts = append(parts, v)
	}
	cityState := strings.TrimSpace(str(a.City))
	if v := str(a.State); v != "" {
		if cityState != "" {
			cityState += "/" + v
		} else {
			cityState = v
		}
	}
	if cityState != "" {
		parts = append(parts, cityState)
	}
	if v := str(a.CEP); v != "" {
		parts = append(parts, "CEP "+v)
	}
	if v := str(a.Label); v != "" {
		return v + ": " + strings.Join(parts, ", ")
	}
	return strings.Join(parts, ", ")
}

func formatVehicle(v *entities.VehicleSummary) string {
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return strings.TrimSpace(*p)
	}
	parts := []string{}
	if x := str(v.Brand); x != "" {
		parts = append(parts, x)
	}
	if x := str(v.Model); x != "" {
		parts = append(parts, x)
	}
	if x := str(v.Color); x != "" {
		parts = append(parts, x)
	}
	out := strings.Join(parts, " ")
	if p := str(v.Plate); p != "" {
		p = formatPlate(p)
		if out != "" {
			out += " · " + p
		} else {
			out = p
		}
	}
	return strings.ToUpper(out)
}

// formatPlate aplica a máscara AAA-9A99 (insere hífen após a 3ª posição).
// Aceita placas no padrão antigo (ABC1234) e Mercosul (ABC1D23). Se a placa
// não tiver 7 caracteres alfanuméricos, devolve em caixa alta sem mudança.
func formatPlate(p string) string {
	clean := strings.ToUpper(strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		default:
			return -1
		}
	}, p))
	if len(clean) != 7 {
		return clean
	}
	return clean[:3] + "-" + clean[3:]
}

