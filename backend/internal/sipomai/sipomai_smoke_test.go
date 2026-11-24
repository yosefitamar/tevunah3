// Smoke test contra o Postgres de dev: monta o pedido à IA com o catálogo
// real e ocorrências reais pendentes, e confere a validação da resposta com
// respostas fabricadas (sem chamar a API). Pula sem APP_DATABASE_URL.
package sipomai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	idb "github.com/belia/tevunah/backend/internal/db"
	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipom"
)

func TestSmoke_BuildAndParse(t *testing.T) {
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("APP_DATABASE_URL não definido")
	}
	db, err := idb.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	cat, err := sipom.Load(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	terms, err := sipom.NewTermRepo(db).Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repo := opsreport.New(db)

	find := func(code string) *opsreport.StoredOccurrence {
		var id string
		err := db.QueryRowContext(ctx, `
			SELECT id FROM app.ops_occurrences
			 WHERE deleted_at IS NULL AND $1 = ANY(sipom_pendencias) ORDER BY id LIMIT 1`, code).Scan(&id)
		if err != nil {
			return nil
		}
		so, err := repo.FindByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return so
	}

	t.Run("natureza", func(t *testing.T) {
		so := find(sipom.PendNatureza)
		if so == nil {
			t.Skip("sem ocorrência com a pendência")
		}
		req, ok := Build(cat, terms, so, nil)
		if !ok || !strings.Contains(req.Prompt, "# NATUREZA") {
			t.Fatalf("pedido sem a seção de natureza: ok=%v", ok)
		}
		nat := cat.Naturezas()[0]
		got := parse(t, req, fmt.Sprintf(`{"natureza":{"natureza_id":%d,"confianca":"alta","motivo":"x"},
			"bairro":null,"pessoas":[],"armas":[],"drogas":[],"veiculos":[]}`, nat.ID))
		if len(got) != 1 || !got[0].Applicable || got[0].Apply["natureza_id"] != nat.ID {
			t.Fatalf("sugestão de natureza: %+v", got)
		}
		got = parse(t, req, `{"natureza":{"natureza_id":999999,"confianca":"alta","motivo":"x"},
			"bairro":null,"pessoas":[],"armas":[],"drogas":[],"veiculos":[]}`)
		if len(got) != 1 || got[0].Applicable {
			t.Fatalf("id inexistente não pode ser aplicável: %+v", got)
		}
	})

	t.Run("orientacao", func(t *testing.T) {
		so := find(sipom.PendBairroCatalogo)
		if so == nil {
			t.Skip("sem ocorrência com a pendência")
		}
		other := -1
		guid := []Guidance{
			{ID: "g-bairro", Topic: "bairro", CidadeID: so.Sipom.CidadeID, Body: "Zona Rural vai como Centro", Author: "A", Date: "24/11/2026", Pending: true},
			{ID: "g-outra-cidade", Topic: "bairro", CidadeID: &other, Body: "não deve entrar"},
			{ID: "g-natureza", Topic: "natureza", Body: "não deve entrar sem a seção de natureza"},
			{ID: "g-geral", Topic: "geral", Body: "vale sempre"},
		}
		req, ok := Build(cat, terms, so, guid)
		if !ok {
			t.Fatal("sem pedido")
		}
		if !strings.Contains(req.Prompt, "Zona Rural vai como Centro") || !strings.Contains(req.Prompt, "vale sempre") ||
			strings.Contains(req.Prompt, "não deve entrar sem") == !strings.Contains(strings.Join(req.Sections, ","), "natureza") ||
			strings.Contains(req.Prompt, "não deve entrar\n") {
			t.Fatalf("orientações erradas no pedido:\n%s", req.Prompt[strings.Index(req.Prompt, "# ORIENTAÇÕES"):])
		}
		var ref string
		for k, g := range req.guidance {
			if g.ID == "g-bairro" {
				ref = k
			}
		}
		b := cat.Bairros(req.cidadeID)[0]
		got := parse(t, req, fmt.Sprintf(`{"natureza":null,"bairro":{"bairro_id":%d,"confianca":"alta","motivo":"x","orientacao":"%s"},
			"pessoas":[],"armas":[],"drogas":[],"veiculos":[]}`, b.ID, ref))
		var hit *Suggestion
		for i := range got {
			if got[i].Kind == KindBairro {
				hit = &got[i]
			}
		}
		if hit == nil || hit.Guidance == nil || hit.Guidance.ID != "g-bairro" || len(UsedGuidance(got)) != 1 {
			t.Fatalf("citação da orientação: %+v", got)
		}
	})

	t.Run("bairro", func(t *testing.T) {
		so := find(sipom.PendBairroCatalogo)
		if so == nil {
			t.Skip("sem ocorrência com a pendência")
		}
		req, ok := Build(cat, terms, so, nil)
		if !ok || req.cidadeID == 0 {
			t.Fatal("pedido sem a cidade")
		}
		b := cat.Bairros(req.cidadeID)[0]
		got := parse(t, req, fmt.Sprintf(`{"natureza":null,"bairro":{"bairro_id":%d,"confianca":"media","motivo":"x"},
			"pessoas":[],"armas":[],"drogas":[],"veiculos":[]}`, b.ID))
		if len(got) != 1 || !got[0].Applicable {
			t.Fatalf("bairro da cidade deve ser aplicável: %+v", got)
		}
		// Um bairro de outra cidade é recusado.
		other := 0
		for _, c := range cat.Cidades() {
			if c.ID != req.cidadeID && len(cat.Bairros(c.ID)) > 0 {
				other = cat.Bairros(c.ID)[0].ID
				break
			}
		}
		got = parse(t, req, fmt.Sprintf(`{"natureza":null,"bairro":{"bairro_id":%d,"confianca":"alta","motivo":"x"},
			"pessoas":[],"armas":[],"drogas":[],"veiculos":[]}`, other))
		if len(got) != 1 || got[0].Applicable {
			t.Fatalf("bairro de outra cidade não pode ser aplicável: %+v", got)
		}
	})

	t.Run("sexo", func(t *testing.T) {
		so := find(sipom.PendPessoaSexoConfirm)
		if so == nil {
			t.Skip("sem ocorrência com a pendência")
		}
		req, ok := Build(cat, terms, so, nil)
		if !ok || len(req.people) == 0 {
			t.Fatal("pedido sem envolvidos")
		}
		got := parse(t, req, `{"natureza":null,"bairro":null,"pessoas":[{"ref":"P1","sexo":"F","confianca":"alta","motivo":"a acusada"},
			{"ref":"P99","sexo":"M","confianca":"alta","motivo":"x"}],"armas":[],"drogas":[],"veiculos":[]}`)
		if len(got) != 1 || got[0].PersonID != req.people["P1"].id || got[0].Apply["sexo"] != sipom.SexoFeminino {
			t.Fatalf("sexo: %+v", got)
		}
	})

	t.Run("droga", func(t *testing.T) {
		so := find(sipom.PendDroga)
		if so == nil {
			t.Skip("sem ocorrência com a pendência")
		}
		req, ok := Build(cat, terms, so, nil)
		if !ok || len(req.drugs) == 0 {
			t.Fatal("pedido sem drogas")
		}
		d := cat.Drogas()[0]
		got := parse(t, req, fmt.Sprintf(`{"natureza":null,"bairro":null,"pessoas":[],"armas":[],
			"drogas":[{"item":%d,"droga_id":%d,"quantidade":12.6,"confianca":"alta","motivo":"x"}],"veiculos":[]}`, req.drugs[0], d.ID))
		if len(got) != 1 || !got[0].Applicable || got[0].Apply["quantidade"] != 13.0 {
			t.Fatalf("droga (quantidade arredondada): %+v", got)
		}
	})

	t.Run("veiculo", func(t *testing.T) {
		so := find(sipom.PendVeiculo)
		if so == nil {
			t.Skip("sem ocorrência com a pendência")
		}
		req, ok := Build(cat, terms, so, nil)
		if !ok || len(req.vehicles) == 0 {
			t.Fatal("pedido sem veículos")
		}
		for i, ask := range req.vehicles {
			if !ask.material {
				continue
			}
			// Marca/modelo fora dos candidatos não é aceita.
			got := parse(t, req, fmt.Sprintf(`{"natureza":null,"bairro":null,"pessoas":[],"armas":[],"drogas":[],
				"veiculos":[{"item":%d,"situacao":"apreendido","tipo_codigo":%d,"cor_codigo":%d,"marca_modelo_codigo":-1,
				"placa":null,"confianca":"alta","motivo":"x"}]}`, i, cat.VeiculoTipos()[0].ID, cat.VeiculoCores()[0].ID))
			if len(got) != 1 || got[0].Applicable || !strings.Contains(got[0].Value, "marca/modelo") {
				t.Fatalf("veículo sem marca/modelo válida: %+v", got)
			}
			return
		}
		t.Skip("veículo pendente só de placa")
	})
}

func parse(t *testing.T, req *Request, raw string) []Suggestion {
	t.Helper()
	if !json.Valid([]byte(raw)) {
		t.Fatalf("json inválido no teste: %s", raw)
	}
	got, err := req.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return got
}
