package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/belia/tevunah/backend/internal/audit"
	"github.com/belia/tevunah/backend/internal/geocode"
	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipom"
)

// Coordenada das ocorrências do relatório operacional. O endereço do PDF é
// localizado pelo geocodificador da agência (Nominatim próprio — sem custo
// por consulta, e nenhum endereço vai a terceiros); o que ele não acha vira
// pendência, que o analista resolve informando o ponto na ficha.

// locate procura a coordenada do local do fato (o da ocorrência; sem ele, o
// da abordagem — o mesmo local que vai ao SIPOM). Devolve Geo vazio quando o
// endereço não foi localizado; erro só para falha do serviço.
func (a *app) locate(ctx context.Context, o *opsreport.Occurrence) (opsreport.Geo, error) {
	in := opsreport.SipomInput(o)
	street, number := sipom.SplitAddress(in.Address)
	res, err := a.geocoder.Locate(ctx, geocode.Query{
		Street: street, Number: number, Neighborhood: in.Neighborhood, City: in.City,
	})
	if err != nil || res == nil {
		return opsreport.Geo{}, err
	}
	return opsreport.Geo{Lat: &res.Lat, Lng: &res.Lng, Precision: res.Precision, Source: opsreport.GeoAuto}, nil
}

func geoAuditSnapshot(g opsreport.Geo) map[string]any {
	return map[string]any{
		"latitude": g.Lat, "longitude": g.Lng, "geo_precision": g.Precision, "geo_source": g.Source,
	}
}

// PUT /api/ops-occurrences/{id}/geo
//
//	{"latitude": -3.73, "longitude": -38.52}   o analista informa o ponto
//	{"reset": true}                            volta ao geocodificador
//
// O ponto do analista vale sobre o automático e não é refeito quando o
// endereço é corrigido. A ocorrência é recalculada na hora (a pendência de
// coordenada entra ou sai).
func (a *app) handleOpsOccurrenceGeo(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "opsreport.update") {
		return
	}
	id := r.PathValue("id")
	var req struct {
		Latitude  *float64 `json:"latitude"`
		Longitude *float64 `json:"longitude"`
		Reset     bool     `json:"reset"`
	}
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "corpo inválido")
		return
	}
	before, err := a.opsReports.FindByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
		return
	}

	var geo opsreport.Geo
	switch {
	case req.Reset:
		if !a.geocoder.Enabled() {
			httpx.Error(w, http.StatusServiceUnavailable, "geocodificador não configurado (GEOCODER_URL)")
			return
		}
		if geo, err = a.locate(r.Context(), &before.Occurrence); err != nil {
			log.Printf("opsreport geo: %v", err)
			httpx.Error(w, http.StatusBadGateway, "geocodificador indisponível — tente de novo ou informe o ponto")
			return
		}
	case req.Latitude != nil && req.Longitude != nil:
		if *req.Latitude < -90 || *req.Latitude > 90 || *req.Longitude < -180 || *req.Longitude > 180 {
			httpx.Error(w, http.StatusBadRequest, "coordenada fora do intervalo válido")
			return
		}
		geo = opsreport.Geo{Lat: req.Latitude, Lng: req.Longitude, Source: opsreport.GeoManual}
	default:
		httpx.Error(w, http.StatusBadRequest, "informe latitude e longitude, ou reset")
		return
	}

	if err := a.opsReports.SetGeo(r.Context(), id, geo); err != nil {
		if errors.Is(err, opsreport.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "ocorrência não encontrada")
			return
		}
		log.Printf("opsreport geo set: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao gravar")
		return
	}
	a.sipomRecomputeOne(r.Context(), id)

	aid, sid, ip, ua := a.actorInfo(r)
	_ = a.audit.Log(r.Context(), audit.Entry{
		ActorUserID: aid, ActorSessionID: sid, ActorIP: ip, ActorUserAgent: ua,
		Action:       "opsreport.geo.set",
		ResourceType: audit.Ptr("ops_occurrence"),
		ResourceID:   &id,
		Before:       geoAuditSnapshot(before.Geo),
		After:        geoAuditSnapshot(geo),
	})
	a.respondOpsOccurrence(w, r, id)
}
