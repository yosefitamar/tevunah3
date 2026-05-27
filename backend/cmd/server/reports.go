package main

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/reports"
)

// publicReport é a forma JSON do Report. Inclui o número formatado pra
// frontend não precisar conhecer o esquema seq/year.
type publicReport struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	Status          string `json:"status"`
	Number          string `json:"number,omitempty"`
	Seq             *int   `json:"seq,omitempty"`
	Year            *int   `json:"year,omitempty"`
	DocDate         string `json:"doc_date"`
	Subject         string `json:"subject"`
	Origin          string `json:"origin"`
	Diffusion       string `json:"diffusion"`
	PriorDiffusion  string `json:"prior_diffusion"`
	Reference       string `json:"reference"`
	Attachments     string `json:"attachments"`
	Confidentiality string `json:"confidentiality"`
	Visibility      string `json:"visibility"`
	BodyHTML        string `json:"body_html"`

	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  string     `json:"created_by"`
	UpdatedAt  time.Time  `json:"updated_at"`
	UpdatedBy  *string    `json:"updated_by,omitempty"`
	DiffusedAt *time.Time `json:"diffused_at,omitempty"`
	DiffusedBy *string    `json:"diffused_by,omitempty"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	ArchivedBy *string    `json:"archived_by,omitempty"`
}

func toPublicReport(r *reports.Report) publicReport {
	return publicReport{
		ID: r.ID, Kind: r.Kind, Status: r.Status,
		Number: r.Number(), Seq: r.Seq, Year: r.Year,
		DocDate: r.DocDate.Format("2006-01-02"),
		Subject: r.Subject, Origin: r.Origin, Diffusion: r.Diffusion,
		PriorDiffusion: r.PriorDiffusion, Reference: r.Reference,
		Attachments:     r.Attachments,
		Confidentiality: r.Confidentiality,
		Visibility:      r.Visibility,
		BodyHTML:        r.BodyHTML,
		CreatedAt:       r.CreatedAt, CreatedBy: r.CreatedBy,
		UpdatedAt: r.UpdatedAt, UpdatedBy: r.UpdatedBy,
		DiffusedAt: r.DiffusedAt, DiffusedBy: r.DiffusedBy,
		ArchivedAt: r.ArchivedAt, ArchivedBy: r.ArchivedBy,
	}
}

// publicViewer é a forma JSON de um item da lista de viewers.
type publicViewer struct {
	UserID      string    `json:"user_id"`
	UserCode    string    `json:"user_code"`
	DisplayName string    `json:"display_name"`
	GrantedBy   string    `json:"granted_by"`
	GrantedAt   time.Time `json:"granted_at"`
}

func toPublicViewer(v reports.Viewer) publicViewer {
	return publicViewer{
		UserID: v.UserID, UserCode: v.UserCode, DisplayName: v.DisplayName,
		GrantedBy: v.GrantedBy, GrantedAt: v.GrantedAt,
	}
}

type publicQualification struct {
	ID          string          `json:"id"`
	ReportID    string          `json:"report_id"`
	Ord         int             `json:"ord"`
	Kind        string          `json:"kind"`
	EntityID    *string         `json:"entity_id,omitempty"`
	Data        map[string]any  `json:"data"`
	Source      string          `json:"source"`
	ConsultedAt *time.Time      `json:"consulted_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

func toPublicQualification(q *reports.Qualification) publicQualification {
	data := q.Data
	if data == nil {
		data = map[string]any{}
	}
	return publicQualification{
		ID: q.ID, ReportID: q.ReportID, Ord: q.Ord, Kind: q.Kind,
		EntityID: q.EntityID, Data: data, Source: q.Source,
		ConsultedAt: q.ConsultedAt, CreatedAt: q.CreatedAt,
	}
}

// ─── GET /api/reports ──────────────────────────────────────────────────

func (a *app) handleReportsList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.read") {
		return
	}
	me := middleware.UserFrom(r.Context())
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	res, err := a.reports.List(r.Context(), reports.ListOpts{
		Limit:   limit,
		Offset:  offset,
		Status:  strings.TrimSpace(q.Get("status")),
		Search:  strings.TrimSpace(q.Get("search")),
		UserID:  me.ID,
		IsAdmin: hasRole(me.Roles, "administrador"),
	})
	if err != nil {
		log.Printf("reports list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	items := make([]publicReport, 0, len(res.Items))
	for i := range res.Items {
		items = append(items, toPublicReport(&res.Items[i]))
	}
	httpx.OK(w, map[string]any{
		"items":  items,
		"total":  res.Total,
		"limit":  cmpDefault(limit, 25),
		"offset": offset,
	})
}

// ─── POST /api/reports ────────────────────────────────────────────────

type createReportRequest struct {
	Kind            string `json:"kind"`
	DocDate         string `json:"doc_date"`
	Subject         string `json:"subject"`
	Origin          string `json:"origin"`
	Diffusion       string `json:"diffusion"`
	Confidentiality string `json:"confidentiality"`
}

func (a *app) handleReportCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.create") {
		return
	}
	me := middleware.UserFrom(r.Context())
	var req createReportRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	docDate := time.Now()
	if req.DocDate != "" {
		t, err := time.Parse("2006-01-02", req.DocDate)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "doc_date inválido (esperado YYYY-MM-DD)")
			return
		}
		docDate = t
	}
	conf := strings.TrimSpace(req.Confidentiality)
	if conf != "" && !reports.IsValidConfidentiality(conf) {
		httpx.Error(w, http.StatusBadRequest, "confidentiality inválido (sigiloso|secreto|ultrassecreto)")
		return
	}
	rep, err := a.reports.Create(r.Context(), reports.NewReport{
		Kind:            req.Kind,
		DocDate:         docDate,
		Subject:         strings.TrimSpace(req.Subject),
		Origin:          strings.TrimSpace(req.Origin),
		Diffusion:       strings.TrimSpace(req.Diffusion),
		Confidentiality: conf,
		CreatedBy:       me.ID,
	})
	if err != nil {
		log.Printf("reports create: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao criar")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.create",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(rep.ID),
		After:        map[string]any{"kind": rep.Kind, "subject": rep.Subject},
	})
	httpx.OK(w, map[string]any{"report": toPublicReport(rep)})
}

// ─── GET /api/reports/{id} ─────────────────────────────────────────────

func (a *app) handleReportDetail(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.read") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	rep, err := a.reports.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	isAdmin := hasRole(me.Roles, "administrador")
	canAccess, err := a.reports.CanAccess(r.Context(), id, me.ID, isAdmin)
	if err != nil {
		log.Printf("can-access: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao verificar acesso")
		return
	}
	if !canAccess {
		// 404 (não vaza existência).
		httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
		return
	}
	quals, err := a.reports.ListQualifications(r.Context(), id)
	if err != nil {
		log.Printf("reports list qualif: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar qualificações")
		return
	}
	pubQuals := make([]publicQualification, 0, len(quals))
	for i := range quals {
		pubQuals = append(pubQuals, toPublicQualification(&quals[i]))
	}
	httpx.OK(w, map[string]any{
		"report":         toPublicReport(rep),
		"qualifications": pubQuals,
	})
}

// ─── PATCH /api/reports/{id} ───────────────────────────────────────────

type updateReportRequest struct {
	DocDate         *string `json:"doc_date,omitempty"`
	Subject         *string `json:"subject,omitempty"`
	Origin          *string `json:"origin,omitempty"`
	Diffusion       *string `json:"diffusion,omitempty"`
	PriorDiffusion  *string `json:"prior_diffusion,omitempty"`
	Reference       *string `json:"reference,omitempty"`
	Attachments     *string `json:"attachments,omitempty"`
	Confidentiality *string `json:"confidentiality,omitempty"`
	BodyHTML        *string `json:"body_html,omitempty"`
}

func (a *app) handleReportUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	var req updateReportRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if req.Confidentiality != nil && !reports.IsValidConfidentiality(*req.Confidentiality) {
		httpx.Error(w, http.StatusBadRequest, "confidentiality inválido (sigiloso|secreto|ultrassecreto)")
		return
	}
	opts := reports.UpdateOpts{
		Subject:         req.Subject,
		Origin:          req.Origin,
		Diffusion:       req.Diffusion,
		PriorDiffusion:  req.PriorDiffusion,
		Reference:       req.Reference,
		Attachments:     req.Attachments,
		Confidentiality: req.Confidentiality,
		BodyHTML:        req.BodyHTML,
	}
	if req.DocDate != nil {
		t, err := time.Parse("2006-01-02", *req.DocDate)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "doc_date inválido")
			return
		}
		opts.DocDate = &t
	}
	rep, err := a.reports.Update(r.Context(), id, me.ID, opts)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
			return
		}
		if errors.Is(err, reports.ErrNotEditable) {
			httpx.Error(w, http.StatusConflict, "relatório não está em status 'criado'")
			return
		}
		log.Printf("reports update: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.update",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(id),
	})
	httpx.OK(w, map[string]any{"report": toPublicReport(rep)})
}

// ─── POST /api/reports/{id}/diffuse ────────────────────────────────────

func (a *app) handleReportDiffuse(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.diffuse") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	rep, err := a.reports.Diffuse(r.Context(), id, me.ID)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
			return
		}
		if errors.Is(err, reports.ErrInvalidStatus) {
			httpx.Error(w, http.StatusConflict, "relatório não está em status 'criado'")
			return
		}
		log.Printf("reports diffuse: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao difundir")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.diffuse",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(id),
		After:        map[string]any{"number": rep.Number()},
	})
	httpx.OK(w, map[string]any{"report": toPublicReport(rep)})
}

// ─── POST /api/reports/{id}/undiffuse ──────────────────────────────────
//
// Restrita a administrador (permissão report.undiffuse). Exige justificativa
// no corpo da requisição, que é gravada na auditoria.
type undiffuseRequest struct {
	Reason string `json:"reason"`
}

func (a *app) handleReportUndiffuse(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.undiffuse") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")

	var req undiffuseRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if len(reason) < 5 {
		httpx.Error(w, http.StatusBadRequest, "motivo obrigatório (mínimo 5 caracteres)")
		return
	}
	if len(reason) > 1000 {
		httpx.Error(w, http.StatusBadRequest, "motivo muito longo (máx 1000 caracteres)")
		return
	}

	rep, err := a.reports.Undiffuse(r.Context(), id, me.ID)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
			return
		}
		if errors.Is(err, reports.ErrInvalidStatus) {
			httpx.Error(w, http.StatusConflict, "relatório não está em status 'difundido'")
			return
		}
		log.Printf("reports undiffuse: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao reverter difusão")
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.undiffuse",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(id),
		Before:       map[string]any{"status": "difundido"},
		After: map[string]any{
			"status":           "criado",
			"reason":           reason,
			"preserved_number": rep.Number(),
		},
	})
	httpx.OK(w, map[string]any{"report": toPublicReport(rep)})
}

// ─── POST /api/reports/{id}/archive ────────────────────────────────────

func (a *app) handleReportArchive(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.archive") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	rep, err := a.reports.Archive(r.Context(), id, me.ID)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
			return
		}
		if errors.Is(err, reports.ErrInvalidStatus) {
			httpx.Error(w, http.StatusConflict, "relatório não está em status 'difundido'")
			return
		}
		log.Printf("reports archive: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao arquivar")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.archive",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(id),
	})
	httpx.OK(w, map[string]any{"report": toPublicReport(rep)})
}

// ─── DELETE /api/reports/{id} ─────────────────────────────────────────
//
// Destrói (soft delete) um rascunho. Permitido somente quando status='criado'
// e o caller é o autor ou um admin. Auditado.
func (a *app) handleReportDestroy(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.destroy") {
		return
	}
	rep := a.authorOrAdmin(w, r)
	if rep == nil {
		return
	}
	if rep.Status != reports.StatusCriado {
		httpx.Error(w, http.StatusConflict, "apenas rascunhos podem ser destruídos")
		return
	}
	me := middleware.UserFrom(r.Context())
	if err := a.reports.Destroy(r.Context(), rep.ID, me.ID); err != nil {
		if errors.Is(err, reports.ErrInvalidStatus) {
			httpx.Error(w, http.StatusConflict, "apenas rascunhos podem ser destruídos")
			return
		}
		log.Printf("reports destroy: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao destruir rascunho")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.destroy",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(rep.ID),
		Before: map[string]any{
			"status":  rep.Status,
			"subject": rep.Subject,
		},
	})
	httpx.NoContent(w)
}

// ─── Qualifications ────────────────────────────────────────────────────

type createQualificationRequest struct {
	Kind        string         `json:"kind"`
	EntityID    *string        `json:"entity_id,omitempty"`
	Data        map[string]any `json:"data"`
	Source      string         `json:"source"`
	ConsultedAt *string        `json:"consulted_at,omitempty"`
}

func (a *app) handleReportQualificationCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.update") {
		return
	}
	reportID := r.PathValue("id")
	// Bloqueia se report não está em 'criado'.
	rep, err := a.reports.FindByID(r.Context(), reportID)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if rep.Status != reports.StatusCriado {
		httpx.Error(w, http.StatusConflict, "relatório não está em status 'criado'")
		return
	}
	var req createQualificationRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	var consultedAt *time.Time
	if req.ConsultedAt != nil && *req.ConsultedAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ConsultedAt)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "consulted_at inválido (ISO 8601)")
			return
		}
		consultedAt = &t
	}
	q, err := a.reports.AddQualification(r.Context(), reports.NewQualification{
		ReportID:    reportID,
		Kind:        req.Kind,
		EntityID:    req.EntityID,
		Data:        req.Data,
		Source:      req.Source,
		ConsultedAt: consultedAt,
	})
	if err != nil {
		switch {
		case errors.Is(err, reports.ErrQualificationKind):
			httpx.Error(w, http.StatusBadRequest, "kind inválido (esperado militar|civil)")
		case errors.Is(err, reports.ErrMissingEntity):
			httpx.Error(w, http.StatusBadRequest, "qualificação CIVIL exige entity_id")
		case errors.Is(err, reports.ErrUnexpectedEntity):
			httpx.Error(w, http.StatusBadRequest, "qualificação MILITAR não aceita entity_id")
		default:
			log.Printf("qualification create: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao criar qualificação")
		}
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.qualification.create",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(reportID),
		After:        map[string]any{"kind": q.Kind, "qualification_id": q.ID},
	})
	httpx.OK(w, map[string]any{"qualification": toPublicQualification(q)})
}

func (a *app) handleReportQualificationDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.update") {
		return
	}
	reportID := r.PathValue("id")
	rep, err := a.reports.FindByID(r.Context(), reportID)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if rep.Status != reports.StatusCriado {
		httpx.Error(w, http.StatusConflict, "relatório não está em status 'criado'")
		return
	}
	qid := r.PathValue("qid")
	if err := a.reports.DeleteQualification(r.Context(), qid); err != nil {
		log.Printf("qualification delete: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.qualification.delete",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(reportID),
		Before:       map[string]any{"qualification_id": qid},
	})
	httpx.NoContent(w)
}


// ─── Visibilidade & viewers ────────────────────────────────────────────

// authorOrAdmin valida que o caller é o autor do relatório ou um admin.
// Devolve o report carregado pra evitar refetch nos callers. Em qualquer
// falha (não encontrado / sem permissão / DB), escreve a resposta HTTP e
// devolve nil.
func (a *app) authorOrAdmin(w http.ResponseWriter, r *http.Request) *reports.Report {
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	rep, err := a.reports.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
			return nil
		}
		log.Printf("authorOrAdmin: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return nil
	}
	if rep.CreatedBy != me.ID && !hasRole(me.Roles, "administrador") {
		// 403 explícito — autor/admin SABEM da existência (são quem opera).
		httpx.Error(w, http.StatusForbidden, "ação restrita ao autor do relatório")
		return nil
	}
	return rep
}

// PUT /api/reports/{id}/visibility — corpo { "visibility": "aberto"|"restrito" }.
type setVisibilityRequest struct {
	Visibility string `json:"visibility"`
}

func (a *app) handleReportSetVisibility(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.read") {
		return
	}
	rep := a.authorOrAdmin(w, r)
	if rep == nil {
		return
	}
	var req setVisibilityRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	v := strings.TrimSpace(req.Visibility)
	if !reports.IsValidVisibility(v) {
		httpx.Error(w, http.StatusBadRequest, "visibility inválido (aberto|restrito)")
		return
	}
	me := middleware.UserFrom(r.Context())
	before := rep.Visibility
	updated, err := a.reports.SetVisibility(r.Context(), rep.ID, v, me.ID)
	if err != nil {
		if errors.Is(err, reports.ErrInvalidStatus) {
			httpx.Error(w, http.StatusConflict, "relatório difundido não pode ter visibilidade alterada")
			return
		}
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
			return
		}
		log.Printf("set visibility: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao alterar visibilidade")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.visibility.change",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(rep.ID),
		Before:       map[string]any{"visibility": before},
		After:        map[string]any{"visibility": updated.Visibility},
	})
	httpx.OK(w, map[string]any{"report": toPublicReport(updated)})
}

// GET /api/reports/{id}/viewers — só autor/admin enxergam a lista.
func (a *app) handleReportViewersList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.read") {
		return
	}
	rep := a.authorOrAdmin(w, r)
	if rep == nil {
		return
	}
	vs, err := a.reports.ListViewers(r.Context(), rep.ID)
	if err != nil {
		log.Printf("list viewers: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar viewers")
		return
	}
	out := make([]publicViewer, 0, len(vs))
	for _, v := range vs {
		out = append(out, toPublicViewer(v))
	}
	httpx.OK(w, map[string]any{"viewers": out})
}

// PUT /api/reports/{id}/viewers — corpo { "user_ids": ["uuid", ...] }.
type setViewersRequest struct {
	UserIDs []string `json:"user_ids"`
}

func (a *app) handleReportSetViewers(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "report.read") {
		return
	}
	rep := a.authorOrAdmin(w, r)
	if rep == nil {
		return
	}
	var req setViewersRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	me := middleware.UserFrom(r.Context())
	before, after, err := a.reports.SetViewers(r.Context(), rep.ID, req.UserIDs, me.ID)
	if err != nil {
		if errors.Is(err, reports.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "relatório não encontrado")
			return
		}
		if errors.Is(err, reports.ErrInvalidStatus) {
			httpx.Error(w, http.StatusConflict, "relatório difundido não pode ter viewers alterados")
			return
		}
		log.Printf("set viewers: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar viewers")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "report.viewers.set",
		ResourceType: audit.Ptr("report"),
		ResourceID:   audit.Ptr(rep.ID),
		Before:       map[string]any{"user_ids": before},
		After:        map[string]any{"user_ids": after},
	})
	// Devolve a lista resolvida pra o frontend atualizar a UI sem refetch.
	vs, err := a.reports.ListViewers(r.Context(), rep.ID)
	if err != nil {
		log.Printf("list viewers post-set: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar viewers")
		return
	}
	out := make([]publicViewer, 0, len(vs))
	for _, v := range vs {
		out = append(out, toPublicViewer(v))
	}
	httpx.OK(w, map[string]any{"viewers": out})
}
