package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/belia/tevunah/backend/internal/audit"
	idb "github.com/belia/tevunah/backend/internal/db"
	"github.com/belia/tevunah/backend/internal/entities"
	"github.com/belia/tevunah/backend/internal/geocode"
	"github.com/belia/tevunah/backend/internal/intel"
	"github.com/belia/tevunah/backend/internal/opsreport"
	"github.com/belia/tevunah/backend/internal/sipom"
	"github.com/belia/tevunah/backend/internal/users"
)

// Mesmo teto do upload pela tela (o relatório diário tem < 2 MiB).
const opsImportMaxBytes = 20 << 20

// runOpsImport importa em lote os PDFs do Relatório Diário de Ocorrências —
// o acervo de meses anteriores, que pela tela entraria um arquivo por vez.
// Faz o mesmo que a importação da tela: recorta o batalhão, marca a
// participação da inteligência, traduz para o SIPOM, localiza o endereço,
// aplica o vínculo automático com os dossiês, guarda o PDF e registra na
// auditoria, em nome do usuário informado.
//
//	admin ops-import -user <email> [-dry-run] [-after <ficha>] <pasta|arquivo.pdf>...
//
// É seguro rodar de novo: PDF já importado é pulado (pelo hash), e ocorrência
// cuja ficha CIOPS já existe não é duplicada. -after descarta a ficha
// informada e as anteriores a ela: as que já foram lançadas no SIPOM à mão.
func runOpsImport(args []string) {
	fs := flag.NewFlagSet("ops-import", flag.ExitOnError)
	email := fs.String("user", "", "e-mail do usuário em nome de quem a importação é registrada")
	dry := fs.Bool("dry-run", false, "só lê e mostra o que seria importado, sem gravar")
	after := fs.String("after", "", "ficha CIOPS de corte: ela e as anteriores não são importadas")
	_ = fs.Parse(args)
	if *email == "" || fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "uso: admin ops-import -user <email> [-dry-run] [-after <ficha>] <pasta|arquivo.pdf>...")
		os.Exit(2)
	}
	files, err := opsImportFiles(fs.Args())
	if err != nil {
		log.Fatal(err)
	}
	if len(files) == 0 {
		log.Fatal("nenhum PDF encontrado")
	}

	ctx := context.Background()
	db := openDB()
	defer db.Close()

	me, err := users.New(db).FindByEmail(ctx, *email)
	if err != nil {
		log.Fatalf("usuário %s: %v", *email, err)
	}
	if !me.IsActive() {
		log.Fatalf("usuário %s não está ativo", *email)
	}

	unit := idb.Env("OPS_REPORT_UNIT", "2º BPRAIO")
	repo := opsreport.New(db)
	geo := newGeocoder(db)
	repo.GeoRequired = geo.Enabled()
	ents := entities.New(db)
	// Auditoria pela mesma conexão que o servidor usa (o papel que só escreve
	// no audit); sem ela configurada, vale a da aplicação.
	auditDB := db
	if dsn := idb.Env("AUDIT_DATABASE_URL", ""); dsn != "" {
		if auditDB, err = idb.Open(dsn); err != nil {
			log.Fatalf("db de auditoria: %v", err)
		}
		defer auditDB.Close()
	}
	auditLog := audit.New(auditDB)

	cat, err := sipom.Load(ctx, db)
	if err != nil {
		log.Fatalf("catálogo do SIPOM: %v", err)
	}
	matcher, err := intel.New(db).Matcher(ctx)
	if err != nil {
		log.Fatalf("termos de inteligência: %v", err)
	}
	refs := opsreport.SipomRefs{GeoRequired: geo.Enabled()}
	if refs.Natures, err = sipom.NewMapRepo(db).Load(ctx); err != nil {
		log.Fatalf("de-para de naturezas: %v", err)
	}
	if refs.Areas, err = sipom.NewAreaRuleRepo(db).Load(ctx); err != nil {
		log.Fatalf("referência de áreas: %v", err)
	}
	if refs.Terms, err = sipom.NewTermRepo(db).Load(ctx); err != nil {
		log.Fatalf("termos aprendidos: %v", err)
	}

	photoDir := strings.TrimSpace(os.Getenv("PHOTO_DIR"))
	if photoDir == "" {
		photoDir = "/var/lib/tevunah/photos"
	}
	pdfDir := filepath.Join(photoDir, opsreport.ReportsSubdir)
	if !*dry {
		if err := os.MkdirAll(pdfDir, 0o750); err != nil {
			log.Fatalf("preparar %s: %v", pdfDir, err)
		}
	}

	fmt.Printf("Unidade: %s · usuário: %s · geocodificador: %s · %d arquivo(s)%s\n\n",
		unit, me.Email, map[bool]string{true: "ligado", false: "desligado"}[geo.Enabled()],
		len(files), map[bool]string{true: " · SIMULAÇÃO (nada é gravado)", false: ""}[*dry])

	cut := strings.ToUpper(strings.TrimSpace(*after))
	if cut != "" {
		if _, ok := opsreport.CIOPSNotAfter(cut, cut); !ok {
			log.Fatalf("ficha de corte inválida: %q", *after)
		}
		fmt.Printf("Corte: fichas até %s (inclusive) não são importadas\n\n", cut)
	}

	var tot struct{ files, skippedFiles, failed, occ, dupOcc, cutOcc, linked, located, unlocated, intel int }
	geoDown := false
	for _, path := range files {
		name := filepath.Base(path)
		fail := func(format string, a ...any) {
			tot.failed++
			fmt.Printf("✗ %-44s %s\n", name, fmt.Sprintf(format, a...))
		}

		data, err := os.ReadFile(path)
		if err != nil {
			fail("não foi possível ler: %v", err)
			continue
		}
		if len(data) > opsImportMaxBytes {
			fail("maior que 20 MiB")
			continue
		}
		sum := sha256.Sum256(data)
		sha := hex.EncodeToString(sum[:])
		if prev, err := repo.FindReportBySHA(ctx, sha); err == nil && prev != nil {
			tot.skippedFiles++
			fmt.Printf("= %-44s já importado\n", name)
			continue
		}

		pages, err := opsreport.Extract(ctx, data)
		if err != nil {
			fail("%v", err)
			continue
		}
		rep := opsreport.Parse(pages)
		mine := rep.ForUnit(unit)
		if cut != "" {
			kept := mine[:0]
			for _, o := range mine {
				before, ok := opsreport.CIOPSNotAfter(o.CIOPS, cut)
				if !ok {
					// Sem ficha (ou em outro formato) não há como saber: entra, e
					// a fila mostra a ficha para o analista conferir.
					fmt.Printf("⚠ %-44s ficha %q não se compara com o corte — importada\n", name, o.CIOPS)
				}
				if before {
					tot.cutOcc++
					continue
				}
				kept = append(kept, o)
			}
			if len(kept) < len(mine) {
				fmt.Printf("- %-44s %d ocorrência(s) até a ficha de corte, descartada(s)\n", name, len(mine)-len(kept))
				if len(kept) == 0 {
					tot.skippedFiles++
					continue
				}
			}
			mine = kept
		}
		if len(mine) == 0 {
			tot.skippedFiles++
			fmt.Printf("= %-44s %d ocorrência(s) no PDF, nenhuma do %s\n", name, len(rep.Occurrences), unit)
			continue
		}

		fichas := make([]string, 0, len(mine))
		for _, o := range mine {
			if o.CIOPS != "" {
				fichas = append(fichas, o.CIOPS)
			}
		}
		existing, err := repo.ExistingCIOPS(ctx, fichas)
		if err != nil {
			log.Fatalf("consultar o acervo: %v", err)
		}

		var links []opsreport.PersonLink
		located, unlocated, intelMarked := 0, 0, 0
		for i := range mine {
			o := &mine[i]
			_, dup := existing[o.CIOPS]
			if !dup && geo.Enabled() && !geoDown {
				g, err := opsImportLocate(ctx, geo, cat, o)
				if err != nil {
					// Sem o geocodificador o lote segue: as ocorrências entram com
					// a pendência de coordenada e `ops-geocode` as refaz depois.
					geoDown = true
					fmt.Printf("⚠ geocodificador indisponível (%v) — o restante entra sem coordenada\n", err)
				} else {
					o.Geo = g
				}
			}
			o.IntelMatched = matcher.Match(o.IntelTexts()...)
			opsreport.TranslateSipom(cat, refs, o)
			if dup {
				continue
			}
			if o.Geo.Lat != nil {
				located++
			} else {
				unlocated++
			}
			if len(o.IntelMatched) > 0 {
				intelMarked++
			}
			for k, p := range o.People {
				auto, err := opsImportAutoLink(ctx, ents, repo, p, me.ClearanceLevel)
				if err != nil {
					log.Printf("candidatos de %q: %v", p.Name, err)
					continue
				}
				if auto != "" {
					links = append(links, opsreport.PersonLink{Occ: i, Person: k, EntityID: auto})
				}
			}
		}
		newCount := len(mine) - len(existing)

		if *dry {
			tot.files++
			tot.occ += newCount
			tot.dupOcc += len(existing)
			tot.linked += len(links)
			tot.located += located
			tot.unlocated += unlocated
			tot.intel += intelMarked
			fmt.Printf("✓ %-44s %s · %d do PDF, %d da unidade, %d nova(s), %d já no acervo · %d vínculo(s) · %d com coordenada\n",
				name, rep.ReportDate.Format("02/01/2006"), len(rep.Occurrences), len(mine), newCount, len(existing), len(links), located)
			continue
		}

		// O PDF original fica guardado para conferência. Nome = hash: o mesmo
		// arquivo nunca é gravado duas vezes.
		dst := filepath.Join(pdfDir, sha+".pdf")
		if err := os.WriteFile(dst, data, 0o640); err != nil {
			log.Fatalf("guardar o PDF: %v", err)
		}
		res, err := repo.Import(ctx, opsreport.ImportInput{
			FileSHA256: sha, FileName: name, FileSize: len(data),
			ReportDate: rep.ReportDate, Total: len(rep.Occurrences),
			Occurrences: mine, Links: links, CreatedBy: me.ID,
		})
		if errors.Is(err, opsreport.ErrAlreadyImported) {
			tot.skippedFiles++
			fmt.Printf("= %-44s já importado\n", name)
			continue
		}
		if err != nil {
			_ = os.Remove(dst)
			fail("erro ao importar: %v", err)
			continue
		}

		ids := make([]string, 0, len(res.Imported))
		for i := range mine {
			if id, ok := res.Imported[i]; ok {
				ids = append(ids, id)
			}
		}
		if err := auditLog.Log(ctx, audit.Entry{
			ActorUserID:  &me.ID,
			Action:       "opsreport.import",
			ResourceType: audit.Ptr("ops_report"),
			ResourceID:   &res.ReportID,
			Reason:       audit.Ptr("importação em lote (cli admin ops-import)"),
			After: map[string]any{
				"file_sha256":    sha,
				"file_name":      name,
				"report_date":    rep.ReportDate.Format("2006-01-02"),
				"total":          len(rep.Occurrences),
				"unit_total":     len(mine),
				"imported":       len(res.Imported),
				"skipped_ciops":  res.Skipped,
				"auto_linked":    res.AutoLinked,
				"intel_marked":   intelMarked,
				"occurrence_ids": ids,
			},
		}); err != nil {
			log.Printf("aviso: falha ao gravar a auditoria de %s: %v", name, err)
		}
		// Os vínculos automáticos só existem depois de gravar: refaz a tradução
		// das ocorrências novas para o aviso de pessoa sem dossiê sair certo.
		for _, id := range ids {
			if _, _, err := repo.RecomputeSipom(ctx, cat, id); err != nil {
				log.Printf("recálculo do SIPOM de %s: %v", id, err)
			}
		}

		tot.files++
		tot.occ += len(res.Imported)
		tot.dupOcc += len(res.Skipped)
		tot.linked += res.AutoLinked
		tot.located += located
		tot.unlocated += unlocated
		tot.intel += intelMarked
		fmt.Printf("✓ %-44s %s · %d do PDF, %d da unidade, %d importada(s), %d já no acervo · %d vínculo(s) · %d com coordenada\n",
			name, rep.ReportDate.Format("02/01/2006"), len(rep.Occurrences), len(mine),
			len(res.Imported), len(res.Skipped), res.AutoLinked, located)
	}

	verb := "importado(s)"
	if *dry {
		verb = "seriam importado(s)"
	}
	fmt.Printf("\nArquivos: %d %s, %d pulado(s), %d com erro\n", tot.files, verb, tot.skippedFiles, tot.failed)
	fmt.Printf("Ocorrências: %d nova(s), %d já no acervo · %d vínculo(s) automático(s) · %d com inteligência\n",
		tot.occ, tot.dupOcc, tot.linked, tot.intel)
	if cut != "" {
		fmt.Printf("Corte: %d ocorrência(s) até %s descartada(s)\n", tot.cutOcc, cut)
	}
	if geo.Enabled() {
		fmt.Printf("Coordenada: %d localizada(s), %d sem coordenada (pendência)\n", tot.located, tot.unlocated)
	}
	if !*dry && tot.occ > 0 {
		ready, pending, err := repo.RecomputeSipom(ctx, cat, "")
		if err != nil {
			log.Fatalf("recálculo: %v", err)
		}
		fmt.Printf("SIPOM (acervo todo): %d ocorrência(s) prontas para envio, %d com pendência\n", ready, pending)
	}
	if tot.failed > 0 {
		os.Exit(1)
	}
}

// opsImportFiles expande os argumentos em PDFs, em ordem de nome (as pastas
// do acervo nomeiam por data, então é a ordem cronológica).
func opsImportFiles(args []string) ([]string, error) {
	var out []string
	for _, a := range args {
		st, err := os.Stat(a)
		if err != nil {
			return nil, err
		}
		if !st.IsDir() {
			out = append(out, a)
			continue
		}
		err = filepath.WalkDir(a, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".pdf") {
				out = append(out, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

// opsImportLocate procura a coordenada do local do fato; Geo vazio quando o
// endereço não foi localizado, erro só para falha do serviço.
func opsImportLocate(ctx context.Context, geo *geocode.Geocoder, cat *sipom.Catalog, o *opsreport.Occurrence) (opsreport.Geo, error) {
	in := opsreport.SipomInput(o)
	street, number := sipom.SplitAddress(in.Address)
	res, err := geo.Locate(ctx, geocode.Query{
		Street: street, Number: number, City: in.City,
		Neighborhood:    cat.CanonNeighborhood(in.City, in.Neighborhood),
		AltNeighborhood: in.Neighborhood,
	})
	if err != nil || res == nil {
		return opsreport.Geo{}, err
	}
	return opsreport.Geo{Lat: &res.Lat, Lng: &res.Lng, Precision: res.Precision, Source: opsreport.GeoAuto}, nil
}

// opsImportAutoLink devolve o dossiê a vincular sozinho à pessoa, ou "" —
// dentro do clearance do usuário da importação.
func opsImportAutoLink(ctx context.Context, ents *entities.Repo, repo *opsreport.Repo, p opsreport.Person, clearance int) (string, error) {
	res, err := ents.FindPersonDuplicates(ctx, entities.DuplicatesQuery{
		Name: p.Name, MotherName: p.MotherName, MaxClearance: clearance,
	})
	if err != nil {
		return "", err
	}
	cands := make([]opsreport.AutoLinkCandidate, 0, len(res.Matches))
	for _, d := range res.Matches {
		c := opsreport.AutoLinkCandidate{ID: d.ID}
		for _, f := range d.MatchedFields {
			if f == "mother_name" {
				c.MotherMatched = true
			}
		}
		cands = append(cands, c)
	}
	auto, _, err := opsreport.DecideAutoLink(p, cands, func(id string) (bool, error) {
		return repo.PersonDeceased(ctx, id)
	})
	return auto, err
}
