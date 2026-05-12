package main

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// publicEntity é o envelope JSON enviado ao frontend.
type publicEntity struct {
	ID             string     `json:"id"`
	Kind           string     `json:"kind"`
	Name           string     `json:"name"`
	Description    string     `json:"description,omitempty"`
	Classification int        `json:"classification"`
	Version        int        `json:"version"`
	Tags           []string   `json:"tags"`
	CreatedAt      time.Time  `json:"created_at"`
	CreatedBy      string     `json:"created_by"`
	UpdatedAt      time.Time  `json:"updated_at"`
	UpdatedBy      string     `json:"updated_by"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
	Attrs          any        `json:"attrs,omitempty"`
}

type personAttrsJSON struct {
	Aliases     []string `json:"aliases"`
	Gender      *string  `json:"gender,omitempty"`
	DateOfBirth *string  `json:"date_of_birth,omitempty"`
	MotherName  *string  `json:"mother_name,omitempty"`
	CPF         *string  `json:"cpf,omitempty"`
	HasPhoto    bool     `json:"has_photo"`
	OrcrimID    *string  `json:"orcrim_id,omitempty"`
	OrcrimName  *string  `json:"orcrim_name,omitempty"`
	OrcrimAlias *string  `json:"orcrim_alias,omitempty"`
}

type organizationAttrsJSON struct {
	Aliases   []string `json:"aliases"`
	LegalName *string  `json:"legal_name,omitempty"`
	TaxID     *string  `json:"tax_id,omitempty"`
	FoundedAt *string  `json:"founded_at,omitempty"`
}

type placeAttrsJSON struct {
	Address   *string  `json:"address,omitempty"`
	Country   *string  `json:"country,omitempty"`
	Region    *string  `json:"region,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

func toPublicEntity(e *entities.Entity) publicEntity {
	tags := e.Tags
	if tags == nil {
		tags = []string{}
	}
	pe := publicEntity{
		ID: e.ID, Kind: string(e.Kind), Name: e.Name, Description: e.Description,
		Classification: e.Classification, Version: e.Version, Tags: tags,
		CreatedAt: e.CreatedAt, CreatedBy: e.CreatedBy,
		UpdatedAt: e.UpdatedAt, UpdatedBy: e.UpdatedBy,
		DeletedAt: e.DeletedAt,
	}
	switch e.Kind {
	case entities.KindPerson:
		if e.Person != nil {
			a := personAttrsJSON{
				Aliases:     e.Person.Aliases,
				Gender:      e.Person.Gender,
				MotherName:  e.Person.MotherName,
				CPF:         e.Person.CPF,
				HasPhoto:    e.Person.PhotoPath != nil && *e.Person.PhotoPath != "",
				OrcrimID:    e.Person.OrcrimID,
				OrcrimName:  e.Person.OrcrimName,
				OrcrimAlias: e.Person.OrcrimAlias,
			}
			if e.Person.DateOfBirth != nil {
				s := e.Person.DateOfBirth.Format("2006-01-02")
				a.DateOfBirth = &s
			}
			if a.Aliases == nil {
				a.Aliases = []string{}
			}
			pe.Attrs = a
		}
	case entities.KindOrganization:
		if e.Organization != nil {
			aliases := e.Organization.Aliases
			if aliases == nil {
				aliases = []string{}
			}
			a := organizationAttrsJSON{
				Aliases:   aliases,
				LegalName: e.Organization.LegalName,
				TaxID:     e.Organization.TaxID,
			}
			if e.Organization.FoundedAt != nil {
				s := e.Organization.FoundedAt.Format("2006-01-02")
				a.FoundedAt = &s
			}
			pe.Attrs = a
		}
	case entities.KindPlace:
		if e.Place != nil {
			pe.Attrs = placeAttrsJSON{
				Address:   e.Place.Address,
				Country:   e.Place.Country,
				Region:    e.Place.Region,
				Latitude:  e.Place.Latitude,
				Longitude: e.Place.Longitude,
			}
		}
	}
	return pe
}

// ─────────────────────────── GET /api/entities ─────────────────────────

func (a *app) handleEntitiesList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.list") {
		return
	}
	me := middleware.UserFrom(r.Context())
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	classification, _ := strconv.Atoi(q.Get("classification"))

	kind := entities.Kind(q.Get("kind"))
	if kind != "" && !kind.IsValid() {
		httpx.Error(w, http.StatusBadRequest, "kind inválido")
		return
	}

	// ?deleted=only ativa a visão Lixeira. Gateamos por entity.restore — quem
	// não pode restaurar, não vê a lixeira (registros removidos somem para ele).
	onlyDeleted := q.Get("deleted") == "only"
	if onlyDeleted {
		if !a.requirePerm(w, r, "entity.restore") {
			return
		}
	}

	res, err := a.entities.List(r.Context(), entities.ListOpts{
		Limit:          limit,
		Offset:         offset,
		Kind:           kind,
		Classification: classification,
		Tag:            strings.TrimSpace(q.Get("tag")),
		Search:         strings.TrimSpace(q.Get("search")),
		SortBy:         q.Get("sort_by"),
		SortDir:        q.Get("sort_dir"),
		MaxClearance:   me.ClearanceLevel,
		OnlyDeleted:    onlyDeleted,
	})
	if err != nil {
		log.Printf("entities list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar entidades")
		return
	}
	items := make([]publicEntity, 0, len(res.Items))
	for i := range res.Items {
		items = append(items, toPublicEntity(&res.Items[i]))
	}
	httpx.OK(w, map[string]any{
		"items":  items,
		"total":  res.Total,
		"limit":  cmpDefault(limit, 25),
		"offset": offset,
	})
}

// ─────────────────────────── GET /api/entities/{id} ────────────────────

func (a *app) handleEntityDetail(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.read") {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	me := middleware.UserFrom(r.Context())

	e, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		log.Printf("entities find: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if e.Classification > me.ClearanceLevel {
		// Não vazar existência da entidade para quem não tem clearance.
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}
	if e.DeletedAt != nil {
		httpx.Error(w, http.StatusGone, "entidade excluída")
		return
	}
	httpx.OK(w, map[string]any{"entity": toPublicEntity(e)})
}

// ─────────────────────────── POST /api/entities ────────────────────────

type createEntityRequest struct {
	Kind           string                 `json:"kind"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Classification int                    `json:"classification"`
	Tags           []string               `json:"tags"`
	Person         *personAttrsJSON       `json:"person,omitempty"`
	Organization   *organizationAttrsJSON `json:"organization,omitempty"`
	Place          *placeAttrsJSON        `json:"place,omitempty"`
}

func (a *app) handleEntityCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.create") {
		return
	}
	me := middleware.UserFrom(r.Context())

	var req createEntityRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	kind := entities.Kind(strings.TrimSpace(req.Kind))
	if !kind.IsValid() {
		httpx.Error(w, http.StatusBadRequest, "kind inválido (person|organization|place)")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpx.Error(w, http.StatusBadRequest, "name obrigatório")
		return
	}
	if req.Classification == 0 {
		req.Classification = 1
	}
	if req.Classification < 1 || req.Classification > 4 {
		httpx.Error(w, http.StatusBadRequest, "classification deve estar entre 1 e 4")
		return
	}
	if req.Classification > me.ClearanceLevel {
		httpx.Error(w, http.StatusForbidden, "clearance insuficiente para esta classification")
		return
	}

	person, organization, place, err := decodeAttrs(kind, req.Person, req.Organization, req.Place)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	e, err := a.entities.Create(r.Context(), entities.NewEntity{
		Kind:           kind,
		Name:           req.Name,
		Description:    req.Description,
		Classification: req.Classification,
		Tags:           req.Tags,
		Person:         person,
		Organization:   organization,
		Place:          place,
	}, me.ID)
	if err != nil {
		switch {
		case errors.Is(err, entities.ErrInvalidKind):
			httpx.Error(w, http.StatusBadRequest, "kind inválido")
		case errors.Is(err, entities.ErrOrgNameDuplicate):
			httpx.Error(w, http.StatusConflict, "organização com este nome já cadastrada")
		case errors.Is(err, entities.ErrCPFDuplicate):
			httpx.Error(w, http.StatusConflict, "CPF já cadastrado")
		default:
			log.Printf("entities create: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao criar entidade")
		}
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := e.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.create",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(e.ID),
		ResourceClassification: &classPtr,
		After:                  toPublicEntity(e),
	})

	httpx.Created(w, map[string]any{"entity": toPublicEntity(e)})
}

// ─────────────────────────── PATCH /api/entities/{id} ──────────────────

type updateEntityRequest struct {
	Version        int                    `json:"version"`
	Name           *string                `json:"name"`
	Description    *string                `json:"description"`
	Classification *int                   `json:"classification"`
	Tags           *[]string              `json:"tags"`
	Person         *personAttrsJSON       `json:"person,omitempty"`
	Organization   *organizationAttrsJSON `json:"organization,omitempty"`
	Place          *placeAttrsJSON        `json:"place,omitempty"`
}

func (a *app) handleEntityUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.update") {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	me := middleware.UserFrom(r.Context())

	var req updateEntityRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	if req.Version <= 0 {
		httpx.Error(w, http.StatusBadRequest, "version obrigatório (>= 1)")
		return
	}
	if req.Classification != nil && (*req.Classification < 1 || *req.Classification > 4) {
		httpx.Error(w, http.StatusBadRequest, "classification deve estar entre 1 e 4")
		return
	}
	if req.Classification != nil && *req.Classification > me.ClearanceLevel {
		httpx.Error(w, http.StatusForbidden, "clearance insuficiente para esta classification")
		return
	}

	// Verifica clearance + visibilidade antes de prosseguir (não vazar 404).
	current, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		log.Printf("entities find pre-update: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if current.Classification > me.ClearanceLevel || current.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}

	person, organization, place, err := decodeAttrsForKind(current.Kind, req.Person, req.Organization, req.Place)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	before, after, err := a.entities.Update(r.Context(), id, req.Version, entities.Patch{
		Name:           trimPtr(req.Name),
		Description:    req.Description,
		Classification: req.Classification,
		Tags:           req.Tags,
		Person:         person,
		Organization:   organization,
		Place:          place,
	}, me.ID)
	if err != nil {
		switch {
		case errors.Is(err, entities.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		case errors.Is(err, entities.ErrVersionConflict):
			httpx.Error(w, http.StatusConflict, "versão desatualizada — recarregue e tente novamente")
		case errors.Is(err, entities.ErrAlreadyDeleted):
			httpx.Error(w, http.StatusGone, "entidade excluída")
		case errors.Is(err, entities.ErrOrgNameDuplicate):
			httpx.Error(w, http.StatusConflict, "organização com este nome já cadastrada")
		case errors.Is(err, entities.ErrCPFDuplicate):
			httpx.Error(w, http.StatusConflict, "CPF já cadastrado")
		default:
			log.Printf("entities update: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar")
		}
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := after.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.update",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(after.ID),
		ResourceClassification: &classPtr,
		Before:                 toPublicEntity(before),
		After:                  toPublicEntity(after),
	})

	httpx.OK(w, map[string]any{"entity": toPublicEntity(after)})
}

// ─────────────────────────── DELETE /api/entities/{id} ─────────────────

type deleteEntityRequest struct {
	Reason string `json:"reason"`
}

func (a *app) handleEntityDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.delete") {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	me := middleware.UserFrom(r.Context())

	current, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		log.Printf("entities find pre-delete: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if current.Classification > me.ClearanceLevel {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}
	if current.DeletedAt != nil {
		httpx.Error(w, http.StatusConflict, "entidade já está excluída")
		return
	}

	var req deleteEntityRequest
	_ = httpx.Decode(r, &req) // corpo opcional

	before, err := a.entities.SoftDelete(r.Context(), id, me.ID)
	if err != nil {
		switch {
		case errors.Is(err, entities.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		case errors.Is(err, entities.ErrAlreadyDeleted):
			httpx.Error(w, http.StatusConflict, "entidade já está excluída")
		default:
			log.Printf("entities delete: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao excluir")
		}
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := before.Classification
	reason := strings.TrimSpace(req.Reason)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.delete",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(before.ID),
		ResourceClassification: &classPtr,
		Before:                 toPublicEntity(before),
		Reason:                 reasonPtr,
	})

	httpx.NoContent(w)
}

// ─────────────────────────── POST /api/entities/{id}/restore ──────────

type restoreEntityRequest struct {
	Reason string `json:"reason"`
}

func (a *app) handleEntityRestore(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.restore") {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	me := middleware.UserFrom(r.Context())

	current, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return
		}
		log.Printf("entities find pre-restore: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return
	}
	if current.Classification > me.ClearanceLevel {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return
	}
	if current.DeletedAt == nil {
		httpx.Error(w, http.StatusConflict, "entidade não está excluída")
		return
	}

	var req restoreEntityRequest
	_ = httpx.Decode(r, &req) // corpo opcional

	before, after, err := a.entities.Restore(r.Context(), id, me.ID)
	if err != nil {
		switch {
		case errors.Is(err, entities.ErrNotFound):
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		case errors.Is(err, entities.ErrNotDeleted):
			httpx.Error(w, http.StatusConflict, "entidade não está excluída")
		case errors.Is(err, entities.ErrOrgNameDuplicate):
			httpx.Error(w, http.StatusConflict,
				"outra organização ativa assumiu este nome — renomeie a atual antes de restaurar")
		default:
			log.Printf("entities restore: %v", err)
			httpx.Error(w, http.StatusInternalServerError, "erro ao restaurar")
		}
		return
	}

	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := after.Classification
	reason := strings.TrimSpace(req.Reason)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.restore",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(after.ID),
		ResourceClassification: &classPtr,
		Before:                 toPublicEntity(before),
		After:                  toPublicEntity(after),
		Reason:                 reasonPtr,
	})

	httpx.OK(w, map[string]any{"entity": toPublicEntity(after)})
}

// ─────────────────────────── helpers ────────────────────────────

// decodeAttrs valida e converte os blocos de attrs do request para os
// equivalentes do pacote entities. Garante coerência com o kind escolhido.
func decodeAttrs(k entities.Kind, p *personAttrsJSON, o *organizationAttrsJSON, pl *placeAttrsJSON) (*entities.PersonAttrs, *entities.OrganizationAttrs, *entities.PlaceAttrs, error) {
	switch k {
	case entities.KindPerson:
		if o != nil || pl != nil {
			return nil, nil, nil, errors.New("envie apenas o bloco 'person' para kind=person")
		}
		return personFromJSON(p), nil, nil, nil
	case entities.KindOrganization:
		if p != nil || pl != nil {
			return nil, nil, nil, errors.New("envie apenas o bloco 'organization' para kind=organization")
		}
		return nil, orgFromJSON(o), nil, nil
	case entities.KindPlace:
		if p != nil || o != nil {
			return nil, nil, nil, errors.New("envie apenas o bloco 'place' para kind=place")
		}
		return nil, nil, placeFromJSON(pl), nil
	}
	return nil, nil, nil, errors.New("kind inválido")
}

// decodeAttrsForKind aceita nil (sem alteração) e nunca falha por ausência;
// só valida que o bloco enviado corresponde ao kind.
func decodeAttrsForKind(k entities.Kind, p *personAttrsJSON, o *organizationAttrsJSON, pl *placeAttrsJSON) (*entities.PersonAttrs, *entities.OrganizationAttrs, *entities.PlaceAttrs, error) {
	if p == nil && o == nil && pl == nil {
		return nil, nil, nil, nil
	}
	switch k {
	case entities.KindPerson:
		if o != nil || pl != nil {
			return nil, nil, nil, errors.New("attrs incompatíveis com kind person")
		}
		return personFromJSON(p), nil, nil, nil
	case entities.KindOrganization:
		if p != nil || pl != nil {
			return nil, nil, nil, errors.New("attrs incompatíveis com kind organization")
		}
		return nil, orgFromJSON(o), nil, nil
	case entities.KindPlace:
		if p != nil || o != nil {
			return nil, nil, nil, errors.New("attrs incompatíveis com kind place")
		}
		return nil, nil, placeFromJSON(pl), nil
	}
	return nil, nil, nil, errors.New("kind inválido")
}

func personFromJSON(p *personAttrsJSON) *entities.PersonAttrs {
	if p == nil {
		return nil
	}
	a := &entities.PersonAttrs{
		Aliases:    p.Aliases,
		Gender:     p.Gender,
		MotherName: p.MotherName,
		CPF:        p.CPF,
		OrcrimID:   p.OrcrimID,
	}
	if p.DateOfBirth != nil {
		if t, err := time.Parse("2006-01-02", *p.DateOfBirth); err == nil {
			a.DateOfBirth = &t
		}
	}
	return a
}

func orgFromJSON(o *organizationAttrsJSON) *entities.OrganizationAttrs {
	if o == nil {
		return nil
	}
	a := &entities.OrganizationAttrs{
		Aliases:   o.Aliases,
		LegalName: o.LegalName,
		TaxID:     o.TaxID,
	}
	if o.FoundedAt != nil {
		if t, err := time.Parse("2006-01-02", *o.FoundedAt); err == nil {
			a.FoundedAt = &t
		}
	}
	return a
}

func placeFromJSON(p *placeAttrsJSON) *entities.PlaceAttrs {
	if p == nil {
		return nil
	}
	return &entities.PlaceAttrs{
		Address:   p.Address,
		Country:   p.Country,
		Region:    p.Region,
		Latitude:  p.Latitude,
		Longitude: p.Longitude,
	}
}

func trimPtr(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	return &v
}
