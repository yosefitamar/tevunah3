package main

import (
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/belia/tevunah/backend/internal/httpx"
	"github.com/belia/tevunah/backend/internal/middleware"
	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/personpdf"
)

// Os PDFs de consulta têm dezenas de KiB; o teto só barra upload abusivo.
const personPDFMaxBytes = 10 << 20

type personPDFResponse struct {
	Report *personpdf.Report `json:"report"`
	// Existing é a pessoa ativa que já tem o CPF, se o usuário pode vê-la.
	Existing *publicEntity `json:"existing,omitempty"`
	// CPFBlocked: o CPF pertence a uma pessoa excluída ou acima do clearance
	// do usuário; criar outra pessoa com ele vai falhar.
	CPFBlocked bool `json:"cpf_blocked"`
}

// POST /api/entities/persons/parse-pdf (multipart, campo "file")
//
// Lê um PDF de consulta (RELATÓRIO DE CPF ou CONSULTA INTEGRADA) anexado no
// cadastro de pessoa e devolve os dados para preencher o formulário. Não
// grava nada: o PDF é descartado ao fim da requisição, e a criação da pessoa
// segue pela rota de entidades, com as próprias permissões e auditoria.
func (a *app) handlePersonPDFParse(w http.ResponseWriter, r *http.Request) {
	if !a.requirePerm(w, r, "entity.list") {
		return
	}
	me := middleware.UserFrom(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, personPDFMaxBytes+1<<20)
	if err := r.ParseMultipartForm(personPDFMaxBytes); err != nil {
		httpx.Error(w, http.StatusBadRequest, "upload inválido ou maior que 10 MiB")
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "arquivo ausente (campo 'file')")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, personPDFMaxBytes+1))
	if err != nil || len(data) > personPDFMaxBytes {
		httpx.Error(w, http.StatusBadRequest, "arquivo maior que 10 MiB")
		return
	}
	if http.DetectContentType(data) != "application/pdf" {
		httpx.Error(w, http.StatusBadRequest, "envie o arquivo em PDF")
		return
	}

	pages, err := opsreport.Extract(r.Context(), data)
	if err != nil {
		if errors.Is(err, opsreport.ErrNoText) {
			httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		log.Printf("person-pdf extract: %v", err)
		httpx.Error(w, http.StatusUnprocessableEntity, "não foi possível ler o PDF")
		return
	}
	rep, err := personpdf.Parse(pages)
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	out := personPDFResponse{Report: rep}
	found, err := a.entities.FindPersonByCPF(r.Context(), rep.CPF, me.ClearanceLevel)
	if err != nil {
		log.Printf("person-pdf lookup: %v", err)
		httpx.Error(w, http.StatusInternalServerError, "erro ao procurar o CPF no cadastro")
		return
	}
	if found.Entity != nil {
		pe := toPublicEntity(found.Entity)
		out.Existing = &pe
	}
	out.CPFBlocked = found.Blocked
	httpx.OK(w, out)
}
