package opsreport

import (
	"context"

	"github.com/belia/tevunah/backend/internal/sipom"
)

// AreaPending é uma ocorrência da fila que ainda pede a área da unidade
// militar, com o lugar como a tradução o lê.
type AreaPending struct {
	ID                 string
	CIOPS              string
	City, Neighborhood string // grafia do relatório
	CidadeID, BairroID *int   // catálogo do SIPOM
}

// AreaPendingList devolve as ocorrências não enviadas com pendência de área
// (sem área, ou mais de uma possível) — a lista de lugares que a tela de
// referências oferece para resolver de uma vez.
func (r *Repo) AreaPendingList(ctx context.Context) ([]AreaPending, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.id, o.ciops_record, o.place_address, o.place_neighborhood, o.place_city,
		       o.approach_address, o.approach_neighborhood, o.approach_city,
		       o.sipom_cidade_id, o.sipom_bairro_id
		  FROM app.ops_occurrences o
		 WHERE o.deleted_at IS NULL AND o.sipom_sent_at IS NULL AND o.sipom_excluida_em IS NULL
		   AND o.sipom_pendencias && $1
		 ORDER BY o.occurred_on`, []string{sipom.PendArea, sipom.PendAreaAmbig})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AreaPending{}
	for rows.Next() {
		var x AreaPending
		var o Occurrence
		if err := rows.Scan(&x.ID, &x.CIOPS, &o.PlaceAddress, &o.PlaceNeighborhood, &o.PlaceCity,
			&o.ApproachAddress, &o.ApproachNeighborhood, &o.ApproachCity, &x.CidadeID, &x.BairroID); err != nil {
			return nil, err
		}
		in := SipomInput(&o)
		x.City, x.Neighborhood = in.City, in.Neighborhood
		out = append(out, x)
	}
	return out, rows.Err()
}
