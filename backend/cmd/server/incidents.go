package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/incidents"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// publicIncident é a forma JSON da ocorrência.
type publicIncident struct {
	ID                 string             `json:"id"`
	Type               string             `json:"type"`
	OccurredOn         string             `json:"occurred_on"` // YYYY-MM-DD
	OccurredTime       *string            `json:"occurred_time,omitempty"`
	CIOPSRecord        string             `json:"ciops_record"`
	IntelParticipation bool               `json:"intel_participation"`
	HasPhoto           bool               `json:"has_photo"`
	Latitude           *float64           `json:"latitude,omitempty"`
	Longitude          *float64           `json:"longitude,omitempty"`
	Description        string             `json:"description"`
	Involved           []publicInvolved   `json:"involved"`
	CreatedAt          time.Time          `json:"created_at"`
	CreatedBy          string             `json:"created_by"`
	UpdatedAt          time.Time          `json:"updated_at"`
	UpdatedBy          *string            `json:"updated_by,omitempty"`
}

type publicInvolved struct {
	EntityID string `json:"entity_id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Role     string `json:"role"`
	HasPhoto bool   `json:"has_photo"`
	Version  int    `json:"version"`
}

func toPublicIncident(i *incidents.Incident) publicIncident {
	involved := make([]publicInvolved, 0, len(i.Involved))
	for _, e := range i.Involved {
		involved = append(involved, publicInvolved{
			EntityID: e.EntityID, Name: e.Name, Kind: e.Kind, Role: e.Role,
			HasPhoto: e.HasPhoto, Version: e.Version,
		})
	}
	return publicIncident{
		ID: i.ID, Type: i.Type,
		OccurredOn:         i.OccurredOn.Format("2006-01-02"),
		OccurredTime:       i.OccurredTime,
		CIOPSRecord:        i.CIOPSRecord,
		IntelParticipation: i.IntelParticipation,
		HasPhoto:           i.PhotoPath != nil && *i.PhotoPath != "",
		Latitude:           i.Latitude,
		Longitude:          i.Longitude,
		Description:        i.Description,
		Involved:           involved,
		CreatedAt:          i.CreatedAt, CreatedBy: i.CreatedBy,
		UpdatedAt: i.UpdatedAt, UpdatedBy: i.UpdatedBy,
	}
}

// ─── GET /api/incidents ────────────────────────────────────────────────

func (a *app) handleIncidentsList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "incident.read") {
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	res, err := a.incidents.List(r.Context(), incidents.ListOpts{
		Limit:     limit,
		Offset:    offset,
		Type:      strings.TrimSpace(q.Get("type")),
		IntelOnly: q.Get("intel") == "1" || q.Get("intel") == "true",
		Search:    strings.TrimSpace(q.Get("search")),
		DateFrom:  strings.TrimSpace(q.Get("date_from")),
		DateTo:    strings.TrimSpace(q.Get("date_to")),
		SortBy:    strings.TrimSpace(q.Get("sort_by")),
		SortDir:   strings.TrimSpace(q.Get("sort_dir")),
	})
	if err != nil {
		log.Printf("incidents list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar")
		return
	}
	items := make([]publicIncident, 0, len(res.Items))
	for i := range res.Items {
		items = append(items, toPublicIncident(&res.Items[i]))
	}
	httpx.OK(w, map[string]any{
		"items":  items,
		"total":  res.Total,
		"limit":  cmpDefault(limit, 25),
		"offset": offset,
	})
}

// ─── POST /api/incidents ───────────────────────────────────────────────

type involvedInput struct {
	EntityID string `json:"entity_id"`
	Role     string `json:"role"`
}

type createIncidentRequest struct {
	Type               string          `json:"type"`
	OccurredOn         string          `json:"occurred_on"`
	OccurredTime       string          `json:"occurred_time"`
	CIOPSRecord        string          `json:"ciops_record"`
	IntelParticipation bool            `json:"intel_participation"`
	Latitude           *float64        `json:"latitude"`
	Longitude          *float64        `json:"longitude"`
	Description        string          `json:"description"`
	Involved           []involvedInput `json:"involved"`
}

func (a *app) handleIncidentCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "incident.create") {
		return
	}
	me := middleware.UserFrom(r.Context())
	var req createIncidentRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if !incidents.IsValidType(req.Type) {
		httpx.Error(w, http.StatusBadRequest, "tipo inválido (homicidio|apreensao|prisao)")
		return
	}
	occurredOn, err := time.Parse("2006-01-02", strings.TrimSpace(req.OccurredOn))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "occurred_on inválido (esperado YYYY-MM-DD)")
		return
	}
	timePtr, ok := parseClock(req.OccurredTime)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "occurred_time inválido (esperado HH:MM)")
		return
	}

	inc, err := a.incidents.Create(r.Context(), incidents.NewIncident{
		Type:               req.Type,
		OccurredOn:         occurredOn,
		OccurredTime:       timePtr,
		CIOPSRecord:        req.CIOPSRecord,
		IntelParticipation: req.IntelParticipation,
		Latitude:           req.Latitude,
		Longitude:          req.Longitude,
		Description:        req.Description,
		CreatedBy:          me.ID,
	})
	if err != nil {
		log.Printf("incidents create: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao criar")
		return
	}

	// Vínculos opcionais informados na criação.
	for _, inv := range req.Involved {
		if strings.TrimSpace(inv.EntityID) == "" {
			continue
		}
		if err := a.incidents.AddEntity(r.Context(), inc.ID, inv.EntityID, inv.Role, me.ID); err != nil {
			log.Printf("incidents add entity (create): %v", err)
		}
	}
	// Recarrega pra devolver os envolvidos resolvidos.
	if reloaded, err := a.incidents.FindByID(r.Context(), inc.ID); err == nil {
		inc = reloaded
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "incident.create",
		ResourceType: audit.Ptr("incident"),
		ResourceID:   audit.Ptr(inc.ID),
		After:        map[string]any{"type": inc.Type, "occurred_on": inc.OccurredOn.Format("2006-01-02")},
	})
	httpx.Created(w, map[string]any{"incident": toPublicIncident(inc)})
}

// ─── GET /api/incidents/{id} ───────────────────────────────────────────

func (a *app) handleIncidentDetail(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "incident.read") {
		return
	}
	id := r.PathValue("id")
	inc, err := a.incidents.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, incidents.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if inc.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	httpx.OK(w, map[string]any{"incident": toPublicIncident(inc)})
}

// ─── PATCH /api/incidents/{id} ─────────────────────────────────────────

func (a *app) handleIncidentUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "incident.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")

	// Decodifica num mapa de raw pra distinguir "campo ausente" de "campo
	// enviado como null" (necessário pra limpar hora/lat/long).
	var raw map[string]json.RawMessage
	if err := httpx.Decode(r, &raw); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}

	var opts incidents.UpdateOpts
	if v, ok := raw["type"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			httpx.Error(w, http.StatusBadRequest, "type inválido")
			return
		}
		if !incidents.IsValidType(s) {
			httpx.Error(w, http.StatusBadRequest, "tipo inválido (homicidio|apreensao|prisao)")
			return
		}
		opts.Type = &s
	}
	if v, ok := raw["occurred_on"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			httpx.Error(w, http.StatusBadRequest, "occurred_on inválido")
			return
		}
		t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "occurred_on inválido (esperado YYYY-MM-DD)")
			return
		}
		opts.OccurredOn = &t
	}
	if v, ok := raw["occurred_time"]; ok {
		opts.OccurredTimeSet = true
		var s string
		if string(v) != "null" {
			if err := json.Unmarshal(v, &s); err != nil {
				httpx.Error(w, http.StatusBadRequest, "occurred_time inválido")
				return
			}
		}
		tp, valid := parseClock(s)
		if !valid {
			httpx.Error(w, http.StatusBadRequest, "occurred_time inválido (esperado HH:MM)")
			return
		}
		opts.OccurredTime = tp
	}
	if v, ok := raw["ciops_record"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			opts.CIOPSRecord = &s
		}
	}
	if v, ok := raw["intel_participation"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err == nil {
			opts.IntelParticipation = &b
		}
	}
	if v, ok := raw["latitude"]; ok {
		opts.LatitudeSet = true
		if string(v) != "null" {
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				httpx.Error(w, http.StatusBadRequest, "latitude inválida")
				return
			}
			opts.Latitude = &f
		}
	}
	if v, ok := raw["longitude"]; ok {
		opts.LongitudeSet = true
		if string(v) != "null" {
			var f float64
			if err := json.Unmarshal(v, &f); err != nil {
				httpx.Error(w, http.StatusBadRequest, "longitude inválida")
				return
			}
			opts.Longitude = &f
		}
	}
	if v, ok := raw["description"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			opts.Description = &s
		}
	}

	inc, err := a.incidents.Update(r.Context(), id, me.ID, opts)
	if err != nil {
		switch {
		case errors.Is(err, incidents.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		case errors.Is(err, incidents.ErrAlreadyDeleted):
			httpx.Error(w, http.StatusConflict, "ocorrência excluída")
		case errors.Is(err, incidents.ErrInvalidType):
			httpx.Error(w, http.StatusBadRequest, "tipo inválido")
		default:
			log.Printf("incidents update: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar")
		}
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "incident.update",
		ResourceType: audit.Ptr("incident"),
		ResourceID:   audit.Ptr(id),
	})
	httpx.OK(w, map[string]any{"incident": toPublicIncident(inc)})
}

// ─── DELETE /api/incidents/{id} ────────────────────────────────────────

func (a *app) handleIncidentDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "incident.delete") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	before, err := a.incidents.SoftDelete(r.Context(), id, me.ID)
	if err != nil {
		switch {
		case errors.Is(err, incidents.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		case errors.Is(err, incidents.ErrAlreadyDeleted):
			httpx.Error(w, http.StatusConflict, "ocorrência já excluída")
		default:
			log.Printf("incidents delete: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao excluir")
		}
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "incident.delete",
		ResourceType: audit.Ptr("incident"),
		ResourceID:   audit.Ptr(id),
		Before:       map[string]any{"type": before.Type, "occurred_on": before.OccurredOn.Format("2006-01-02")},
	})
	httpx.NoContent(w)
}

// ─── Envolvidos ────────────────────────────────────────────────────────

type addInvolvedRequest struct {
	EntityID string `json:"entity_id"`
	Role     string `json:"role"`
}

func (a *app) handleIncidentEntityAdd(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "incident.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	inc := a.findActiveIncident(w, r, id)
	if inc == nil {
		return
	}
	var req addInvolvedRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if strings.TrimSpace(req.EntityID) == "" {
		httpx.Error(w, http.StatusBadRequest, "entity_id obrigatório")
		return
	}
	if err := a.incidents.AddEntity(r.Context(), id, req.EntityID, req.Role, me.ID); err != nil {
		log.Printf("incidents add entity: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao vincular entidade")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "incident.entity.add",
		ResourceType: audit.Ptr("incident"),
		ResourceID:   audit.Ptr(id),
		After:        map[string]any{"entity_id": req.EntityID, "role": req.Role},
	})
	updated, err := a.incidents.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao recarregar")
		return
	}
	httpx.OK(w, map[string]any{"incident": toPublicIncident(updated)})
}

func (a *app) handleIncidentEntityRemove(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "incident.update") {
		return
	}
	id := r.PathValue("id")
	entityID := r.PathValue("eid")
	inc := a.findActiveIncident(w, r, id)
	if inc == nil {
		return
	}
	if err := a.incidents.RemoveEntity(r.Context(), id, entityID); err != nil {
		log.Printf("incidents remove entity: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao desvincular")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "incident.entity.remove",
		ResourceType: audit.Ptr("incident"),
		ResourceID:   audit.Ptr(id),
		Before:       map[string]any{"entity_id": entityID},
	})
	httpx.NoContent(w)
}

// ─── Foto ──────────────────────────────────────────────────────────────

// incidentPhotoFilename monta o filename canônico da foto da ocorrência.
func incidentPhotoFilename(id, ext string) string { return "incident_" + id + ext }

func (a *app) handleIncidentPhotoUpload(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "incident.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	inc := a.findActiveIncident(w, r, id)
	if inc == nil {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, photoMaxBytes)
	if err := r.ParseMultipartForm(photoMaxBytes); err != nil {
		httpx.Error(w, http.StatusBadRequest, "upload inválido ou maior que 5 MiB")
		return
	}
	file, header, err := r.FormFile(photoFieldName)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "arquivo de foto ausente (campo 'photo')")
		return
	}
	defer file.Close()

	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	head = head[:n]
	mime := http.DetectContentType(head)
	ext := extForMime(mime)
	if ext == "" {
		httpx.Error(w, http.StatusBadRequest,
			"formato não suportado — envie JPEG ou PNG (recebido: "+header.Header.Get("Content-Type")+")")
		return
	}

	dir := photoDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		log.Printf("mkdir photo_dir %s: %v", dir, err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao preparar storage")
		return
	}
	filename := incidentPhotoFilename(id, ext)
	dst := filepath.Join(dir, filename)
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		log.Printf("abrir arquivo destino: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar foto")
		return
	}
	defer out.Close()
	if _, err := out.Write(head); err != nil {
		_ = os.Remove(dst)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar foto")
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		_ = os.Remove(dst)
		httpx.Error(w, http.StatusInternalServerError, "erro ao salvar foto")
		return
	}

	oldPath, err := a.incidents.SetPhotoPath(r.Context(), id, filename, me.ID)
	if err != nil {
		_ = os.Remove(dst)
		log.Printf("set incident photo_path: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao registrar foto")
		return
	}
	if oldPath != "" && oldPath != filename {
		_ = os.Remove(filepath.Join(dir, oldPath))
	}

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "incident.photo.set",
		ResourceType: audit.Ptr("incident"),
		ResourceID:   audit.Ptr(id),
		After:        map[string]any{"photo_path": filename, "mime": mime},
	})

	updated, err := a.incidents.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "erro ao recarregar")
		return
	}
	httpx.OK(w, map[string]any{"incident": toPublicIncident(updated)})
}

func (a *app) handleIncidentPhotoGet(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "incident.read") {
		return
	}
	id := r.PathValue("id")
	inc, err := a.incidents.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}
	if inc.DeletedAt != nil || inc.PhotoPath == nil || *inc.PhotoPath == "" {
		httpx.Error(w, http.StatusNotFound, "sem foto registrada")
		return
	}
	path := filepath.Join(photoDir(), *inc.PhotoPath)
	abs, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(abs, photoDir()) {
		httpx.Error(w, http.StatusNotFound, "sem foto registrada")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}

func (a *app) handleIncidentPhotoDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "incident.update") {
		return
	}
	me := middleware.UserFrom(r.Context())
	id := r.PathValue("id")
	inc := a.findActiveIncident(w, r, id)
	if inc == nil {
		return
	}
	if inc.PhotoPath == nil || *inc.PhotoPath == "" {
		httpx.NoContent(w)
		return
	}
	oldPath, err := a.incidents.SetPhotoPath(r.Context(), id, "", me.ID)
	if err != nil {
		log.Printf("clear incident photo: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover foto")
		return
	}
	if oldPath != "" {
		_ = os.Remove(filepath.Join(photoDir(), oldPath))
	}
	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "incident.photo.delete",
		ResourceType: audit.Ptr("incident"),
		ResourceID:   audit.Ptr(id),
		Before:       map[string]any{"photo_path": oldPath},
	})
	httpx.NoContent(w)
}

// ─── helpers ───────────────────────────────────────────────────────────

// findActiveIncident carrega a ocorrência e garante que não está excluída.
// Em qualquer falha escreve a resposta HTTP e devolve nil.
func (a *app) findActiveIncident(w http.ResponseWriter, r *http.Request, id string) *incidents.Incident {
	inc, err := a.incidents.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, incidents.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
			return nil
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return nil
	}
	if inc.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return nil
	}
	return inc
}

// parseClock valida e normaliza uma hora "HH:MM". Vazio → (nil, true) =
// hora ausente. Inválida → (nil, false).
func parseClock(s string) (*string, bool) {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil, true
	}
	t, err := time.Parse("15:04", v)
	if err != nil {
		return nil, false
	}
	hhmm := t.Format("15:04")
	return &hhmm, true
}
