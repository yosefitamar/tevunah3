package opsreport

// Ponta a ponta contra um SIPOM falso: ocorrência real do banco → payload →
// cliente assinado → servidor que confere a assinatura e as regras do manual
// ("API de integração SIPOM — Referência") → resultado gravado. Os dados não
// saem do processo. Pula sem banco ou sem ocorrência que caiba.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/belia/tevunah/backend/internal/sipom"
)

var reItemID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// fakeSipom confere o que o manual exige e responde como o SIPOM:
// "criado" na primeira vez, "sem_alteracao" para o mesmo conteúdo.
func fakeSipom(t *testing.T, secret string) *httptest.Server {
	seen := map[string][32]byte{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		ts := r.Header.Get("X-SIPOM-Timestamp")
		if r.Header.Get("X-SIPOM-Assinatura") != sipom.Sign(secret, ts, r.Method, r.URL.Path, body) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"erro":"assinatura","codigo":"assinatura_invalida"}`))
			return
		}
		var p sipom.Payload
		dec := json.NewDecoder(bytesReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&p); err != nil {
			t.Errorf("corpo fora do contrato: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		errs := checkContract(&p)
		for _, e := range errs {
			t.Errorf("regra do manual: %s", e)
		}
		if len(errs) > 0 {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"erro":"validacao","codigo":"validacao"}`))
			return
		}
		sum := sha256.Sum256(body)
		sit := sipom.SituacaoCriado
		if prev, ok := seen[p.Origem.ID]; ok {
			sit = sipom.SituacaoAtualizado
			if prev == sum {
				sit = sipom.SituacaoSemAlteracao
			}
		}
		seen[p.Origem.ID] = sum
		_ = json.NewEncoder(w).Encode(sipom.Resultado{Protocolo: "0b6f4f4e-4a49-4f0e-9a52-6d1b2f0a9c11",
			Situacao: sit, Identificador: "2026E2E00001", Avisos: []sipom.Aviso{}})
	}))
}

func checkContract(p *sipom.Payload) []string {
	var errs []string
	bad := func(cond bool, msg string) {
		if cond {
			errs = append(errs, msg)
		}
	}
	o := p.Ocorrencia
	_, err := time.Parse(time.RFC3339, o.DataHora)
	bad(err != nil, "data_hora ISO 8601 com fuso: "+o.DataHora)
	bad(len(p.Origem.ID) != 36, "origem.id uuid")
	bad(o.NaturezaFatoID <= 0 || o.UnidadeMilitarID <= 0 || o.OPMMilitarID <= 0, "ids do cabeçalho")
	bad(o.Endereco.CidadeID <= 0 || o.Endereco.BairroID <= 0, "cidade_id e bairro_id")
	bad(o.Endereco.Latitude == 0 || o.Endereco.Longitude == 0, "latitude e longitude")
	bad(len(o.NumeroOcorrencia) > 45, "numero_ocorrencia ≤ 45")
	bad(len(p.Composicao) > 30 || len(p.Procedimentos) > 20 || len(p.Materiais) > 100 || len(p.Fotos) > 10 || len(p.Envolvidos) > 50, "limites de itens")
	for _, c := range p.Composicao {
		bad(!reItemID.MatchString(c.ID) || c.Matricula == "" || c.PoliciamentoTipoID <= 0 || c.FuncaoID <= 0, "composição: "+c.ID)
	}
	for _, pr := range p.Procedimentos {
		bad(!reItemID.MatchString(pr.ID) || pr.ProcedimentoID <= 0 || pr.Numero == "" || len(pr.Ano) != 4, "procedimento: "+pr.ID)
		bad(pr.Reparticao == sipom.ReparticaoPoliciaCivil && pr.DelegaciaID <= 0, "procedimento sem delegacia: "+pr.ID)
	}
	for _, m := range p.Materiais {
		bad(!reItemID.MatchString(m.ID), "material sem id")
		switch m.TipoID {
		case sipom.MaterialArma:
			bad(m.ArmaTipoID == nil, "arma sem tipo")
		case sipom.MaterialDroga:
			bad(m.DrogaTipoID == nil || m.Quantidade == nil, "droga")
		case sipom.MaterialVeiculo:
			bad(m.Placa == "" || m.Situacao == nil || m.CodigoTipo == nil || m.CodigoCor == nil ||
				m.CodigoMarcaModelo == nil || m.AnoFabricacao == nil, "veículo")
		}
	}
	for _, f := range p.Fotos {
		bad(!reItemID.MatchString(f.ID) || f.Base64 == "" || len(f.Base64)*3/4 > sipom.MaxFotoBytes, "foto "+f.ID)
	}
	for _, e := range p.Envolvidos {
		bad(!reItemID.MatchString(e.ID) || e.VinculoID <= 0, "envolvido "+e.ID)
		if e.Pessoa != nil {
			n := utf8.RuneCountInString(e.Pessoa.Nome)
			bad(n < 3 || n > 125, "nome 3 a 125: "+e.Pessoa.Nome)
			bad(e.Pessoa.Sexo != 1 && e.Pessoa.Sexo != 2, "sexo 1 ou 2")
			bad(e.Pessoa.CPF != "" && len(e.Pessoa.CPF) != 11, "cpf")
			if e.Pessoa.Nascimento != "" {
				_, err := time.Parse("2006-01-02", e.Pessoa.Nascimento)
				bad(err != nil, "nascimento AAAA-MM-DD")
			}
		}
	}
	return errs
}

func TestSmoke_SipomEndToEnd(t *testing.T) {
	db, _ := smokeDB(t)
	ctx := context.Background()
	cat, err := sipom.Load(ctx, db)
	if err != nil {
		t.Skipf("catálogo do SIPOM não carregado: %v", err)
	}
	r := New(db)
	// Uma ocorrência com coordenada; com envolvido e droga, de preferência.
	rows, err := db.QueryContext(ctx, `
		SELECT o.id FROM app.ops_occurrences o
		 WHERE o.deleted_at IS NULL AND o.latitude IS NOT NULL AND o.sipom_bairro_id IS NOT NULL
		 ORDER BY EXISTS (SELECT 1 FROM app.ops_occurrence_people p WHERE p.occurrence_id = o.id) DESC,
		          EXISTS (SELECT 1 FROM app.ops_occurrence_drugs d WHERE d.occurrence_id = o.id) DESC,
		          o.occurred_on DESC LIMIT 300`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()

	var so *StoredOccurrence
	for _, id := range ids {
		if _, _, err := r.RecomputeSipom(ctx, cat, id); err != nil {
			t.Fatal(err)
		}
		cand, err := r.FindByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if !SipomBlocked(cand.Sipom.Pending) {
			so = cand
			break
		}
	}
	if so == nil {
		t.Skip("nenhuma ocorrência pronta para o envio no banco")
	}

	shrinkCalls := 0
	src := PayloadSources{
		Photo:    func(string) ([]byte, string) { return nil, "" },
		OccPhoto: func(Photo) []byte { return tinyJPEG(t) },
		Shrink:   func(b []byte) ([]byte, error) { shrinkCalls++; return sipom.ShrinkJPEG(b) },
	}
	build := func() []byte {
		p, err := BuildSipomPayload(cat, so, src)
		if err != nil {
			t.Fatalf("payload de %s: %v", so.ID, err)
		}
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	body := build()
	if string(build()) != string(body) {
		t.Fatalf("o mesmo conteúdo precisa dar o mesmo corpo (sem_alteracao no reenvio)")
	}

	srv := fakeSipom(t, "segredo-e2e")
	defer srv.Close()
	c := &sipom.Client{BaseURL: srv.URL, Key: "cli_e2e", Secret: "segredo-e2e", HTTP: srv.Client(),
		Now: time.Now, Sleep: func(time.Duration) {}}
	res, err := c.Receber(ctx, body)
	if err != nil {
		t.Fatalf("envio: %v", err)
	}
	if res.Situacao != sipom.SituacaoCriado {
		t.Errorf("primeiro envio: %s", res.Situacao)
	}
	if res, err = c.Receber(ctx, body); err != nil || res.Situacao != sipom.SituacaoSemAlteracao {
		t.Errorf("reenvio igual: %v %+v", err, res)
	}
	if shrinkCalls != 0 {
		t.Errorf("foto pequena não deveria ser reduzida")
	}
	t.Logf("ocorrência %s: %d bytes, %d envolvidos, %d policiais, %d armas/veículos, %d drogas, %d fotos",
		so.ID, len(body), len(so.People), len(so.Officers), len(so.Weapons)+len(so.Vehicles), len(so.Drugs), len(so.Photos))
}

// tinyJPEG é uma imagem JPEG 2×2 para as fotos do teste.
func tinyJPEG(t *testing.T) []byte {
	t.Helper()
	img := imageRGBA(2, 2)
	b, err := encodeJPEG(img)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

func imageRGBA(w, h int) image.Image { return image.NewRGBA(image.Rect(0, 0, w, h)) }

func encodeJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, nil)
	return buf.Bytes(), err
}
