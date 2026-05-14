package main

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
)

// publicAddress é o envelope JSON enviado ao frontend.
type publicAddress struct {
	ID           string    `json:"id"`
	Label        *string   `json:"label,omitempty"`
	CEP          *string   `json:"cep,omitempty"`
	Street       *string   `json:"street,omitempty"`
	Number       *string   `json:"number,omitempty"`
	Complement   *string   `json:"complement,omitempty"`
	Neighborhood *string   `json:"neighborhood,omitempty"`
	City         *string   `json:"city,omitempty"`
	State        *string   `json:"state,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	CreatedBy    string    `json:"created_by"`
	UpdatedAt    time.Time `json:"updated_at"`
	UpdatedBy    string    `json:"updated_by"`
}

func toPublicAddress(a *entities.PersonAddress) publicAddress {
	return publicAddress{
		ID:           a.ID,
		Label:        a.Label,
		CEP:          a.CEP,
		Street:       a.Street,
		Number:       a.Number,
		Complement:   a.Complement,
		Neighborhood: a.Neighborhood,
		City:         a.City,
		State:        a.State,
		CreatedAt:    a.CreatedAt,
		CreatedBy:    a.CreatedBy,
		UpdatedAt:    a.UpdatedAt,
		UpdatedBy:    a.UpdatedBy,
	}
}

// addressRequest é o corpo dos POST/PATCH. Strings vazias viram NULL no DB.
type addressRequest struct {
	Label        string `json:"label"`
	CEP          string `json:"cep"`
	Street       string `json:"street"`
	Number       string `json:"number"`
	Complement   string `json:"complement"`
	Neighborhood string `json:"neighborhood"`
	City         string `json:"city"`
	State        string `json:"state"`
}

func (req addressRequest) toDomain() entities.NewPersonAddress {
	return entities.NewPersonAddress{
		Label:        strings.TrimSpace(req.Label),
		CEP:          req.CEP,
		Street:       strings.TrimSpace(req.Street),
		Number:       strings.TrimSpace(req.Number),
		Complement:   strings.TrimSpace(req.Complement),
		Neighborhood: strings.TrimSpace(req.Neighborhood),
		City:         strings.TrimSpace(req.City),
		State:        strings.TrimSpace(req.State),
	}
}

// guardPersonAccess valida que o caller pode ver/operar essa pessoa.
// Retorna a entidade ou escreve resposta de erro e devolve nil.
func (a *app) guardPersonAccess(w http.ResponseWriter, r *http.Request, id string) *entities.Entity {
	me := middleware.UserFrom(r.Context())
	e, err := a.entities.FindByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, entities.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
			return nil
		}
		httpx.Error(w, http.StatusInternalServerError, "erro ao buscar")
		return nil
	}
	if e.Kind != entities.KindPerson {
		httpx.Error(w, http.StatusBadRequest, "endereços são exclusivos do kind person")
		return nil
	}
	if e.Classification > me.ClearanceLevel || e.DeletedAt != nil {
		httpx.Error(w, http.StatusNotFound, "entidade não encontrada")
		return nil
	}
	return e
}

// ─────────────────────── GET /api/entities/{id}/addresses ───────────────

func (a *app) handlePersonAddressList(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.read") {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	if a.guardPersonAccess(w, r, id) == nil {
		return
	}
	addrs, err := a.entities.ListAddresses(r.Context(), id)
	if err != nil {
		log.Printf("addresses list: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao listar endereços")
		return
	}
	out := make([]publicAddress, 0, len(addrs))
	for i := range addrs {
		out = append(out, toPublicAddress(&addrs[i]))
	}
	httpx.OK(w, map[string]any{"items": out})
}

// ─────────────────────── POST /api/entities/{id}/addresses ──────────────

func (a *app) handlePersonAddressCreate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.update") {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpx.Error(w, http.StatusBadRequest, "id obrigatório")
		return
	}
	e := a.guardPersonAccess(w, r, id)
	if e == nil {
		return
	}
	var req addressRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	me := middleware.UserFrom(r.Context())
	addr, err := a.entities.CreateAddress(r.Context(), id, req.toDomain(), me.ID)
	if err != nil {
		log.Printf("address create: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao criar endereço")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := e.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.address.add",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(id),
		ResourceClassification: &classPtr,
		After:                  toPublicAddress(addr),
	})
	httpx.Created(w, map[string]any{"address": toPublicAddress(addr)})
}

// ─────────────────────── PATCH /api/entities/{id}/addresses/{aid} ───────

func (a *app) handlePersonAddressUpdate(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.update") {
		return
	}
	id := r.PathValue("id")
	addrID := r.PathValue("aid")
	if id == "" || addrID == "" {
		httpx.Error(w, http.StatusBadRequest, "id e aid obrigatórios")
		return
	}
	e := a.guardPersonAccess(w, r, id)
	if e == nil {
		return
	}
	var req addressRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	me := middleware.UserFrom(r.Context())
	addr, err := a.entities.UpdateAddress(r.Context(), id, addrID, req.toDomain(), me.ID)
	if err != nil {
		if errors.Is(err, entities.ErrAddressNotFound) {
			httpx.Error(w, http.StatusNotFound, "endereço não encontrado")
			return
		}
		log.Printf("address update: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao atualizar endereço")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := e.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.address.update",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(id),
		ResourceClassification: &classPtr,
		After:                  toPublicAddress(addr),
	})
	httpx.OK(w, map[string]any{"address": toPublicAddress(addr)})
}

// ─────────────────────── DELETE /api/entities/{id}/addresses/{aid} ──────

func (a *app) handlePersonAddressDelete(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.update") {
		return
	}
	id := r.PathValue("id")
	addrID := r.PathValue("aid")
	if id == "" || addrID == "" {
		httpx.Error(w, http.StatusBadRequest, "id e aid obrigatórios")
		return
	}
	e := a.guardPersonAccess(w, r, id)
	if e == nil {
		return
	}
	me := middleware.UserFrom(r.Context())
	before, err := a.entities.DeleteAddress(r.Context(), id, addrID, me.ID)
	if err != nil {
		if errors.Is(err, entities.ErrAddressNotFound) {
			httpx.Error(w, http.StatusNotFound, "endereço não encontrado")
			return
		}
		log.Printf("address delete: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao remover endereço")
		return
	}
	aid, sid, ip, ua := a.actorInfo(r)
	classPtr := e.Classification
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID:            aid,
		ActorSessionID:         sid,
		ActorIP:                ip,
		ActorUserAgent:         ua,
		Action:                 "entity.address.remove",
		ResourceType:           audit.Ptr("entity"),
		ResourceID:             audit.Ptr(id),
		ResourceClassification: &classPtr,
		Before:                 toPublicAddress(before),
	})
	httpx.OK(w, map[string]any{"ok": true})
}
