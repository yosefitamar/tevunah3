# SIPOM — Materiais e Procedimento (levantamento da fase 2)

Levantado em 06/10/2026 direto na tela do SIPOM (ficha da ocorrência,
`/ocorrencias/ocorrencias-exibir/...`), lendo os formulários dos modais
**Material** e **Procedimento** e as listas que eles carregam. Complementa
`sipom-integracao.md` (fase 1) com o que falta para a fase 2.

Nada foi gravado no SIPOM durante o levantamento: só leitura dos formulários,
das listas e de três ocorrências já cadastradas.

## 1. Modal "Material"

Formulário `POST /ocorrencias/ocorrencias-material`
(`application/x-www-form-urlencoded`), um material por envio. Campos fixos:
`csrf_token_name`, `ocorrencia_id`, `material_tipo`. Os demais campos
aparecem conforme o tipo escolhido.

Os `value` dos selects são tokens cifrados que mudam a cada carregamento (como
o vínculo da pessoa, já observado na fase 1). Para a integração valem os ids
das tabelas de referência, que já estão no schema `sipom` do Tevunah
(migrações 00051/00052) — exceto **drogas** e **marcas de celular**, que não
vieram em nenhum dump e estão listadas aqui.

### Tipos (`material_tipo`) — `sipom.material_tipos`

| id | Tipo |
|---|---|
| 1 | Arma |
| 2 | Munição |
| 3 | Droga |
| 4 | Veículo |
| 5 | Celular |
| 6 | Dinheiro |
| 7 | Outros |

### Arma

| Campo | Tipo | Obrig. | Origem dos valores |
|---|---|---|---|
| `arma_tipo` | select | sim | `sipom.arma_tipos` (9): Pistola, Revolver, Fuzil, Simulacro, Branca, Rifle, Espingarda, Artesanal, Carabina |
| `arma_marca` | select | sim | `sipom.arma_marcas` (38; inclui "Suprimida" para marca raspada) |
| `arma_calibre` | select | sim | `sipom.arma_calibres` (27; inclui "Outros") |
| `arma_numero` | texto | não | numeração da arma |
| `arma_quantidade` | número | sim | |
| `arma_descricao` | texto | não | |

Exibição na aba: `Tipo · Marca · Calibre · Numeração · Quantidade · Descrição`.

### Munição

| Campo | Tipo | Obrig. | Origem dos valores |
|---|---|---|---|
| `municao` | select | sim | mesma lista de calibres (`sipom.arma_calibres`) |
| `municao_quantidade` | número | sim | |

### Droga

| Campo | Tipo | Obrig. | Valores |
|---|---|---|---|
| `droga` | select | sim | lista abaixo (não há tabela no dump) |
| `droga_quantidade` | número | sim | na unidade da droga escolhida |

Lista de drogas como o SIPOM apresenta — o nome já carrega a unidade de
medida, e a quantidade é informada nessa unidade:

| Droga | Unidade |
|---|---|
| Chá de Ayahuasca | mililitros (ml) |
| Cocaína | gramas (g) |
| Crack | gramas (g) |
| Ecstasy/MDMA | comprimido |
| Fentanil | miligramas (mg) |
| Haxixe | gramas (g) |
| Heroína | gramas (g) |
| Lsd | dose |
| Maconha | gramas (g) |
| Quetamina | comprimido |
| Skank | gramas (g) |
| Solvente | mililitros (ml) |

Exibição na aba: `Nome: Cocaína · Quantidade: 10 gramas (g)`.

### Veículo

| Campo | Tipo | Obrig. | Origem dos valores |
|---|---|---|---|
| `situacao` | select | sim | 1 = Apreendido, 2 = Recuperado |
| `placa` | texto | não | ao digitar, `GET /ocorrencias/ocorrencias-consulta-placa?placa=` preenche tipo, marca/modelo, cor e anos e trava os selects |
| `codigo_veiculo_tipo` | select | sim | `GET /ajax/veiculos-all-tipos` → `sipom.veiculo_tipos` (22; código DENATRAN) |
| `codigo_marca_modelo` | select com busca | sim | `GET /ajax/veiculos-busca-marcas-modelos?q=` → `sipom.marcasmodelos` (40.733; `codigo` DENATRAN, `desc` "HONDA/CG 160 FAN") |
| `codigo_cor` | select | sim | `GET /ajax/veiculos-all-cores` → `sipom.veiculo_cores` (16) |
| `veiculo_tipo`, `marca_modelo`, `veiculo_cor` | ocultos | — | texto das opções escolhidas, enviados junto com os códigos |
| `ano_fabricacao`, `ano_modelo` | número | não | |

Exibição na aba: `FWB-7B19 - I/HONDA CB 350 - PRATA - MOTOCLICLETA - 2018/2018 - Apreendido`.

### Celular

| Campo | Tipo | Obrig. | Valores |
|---|---|---|---|
| `marca_celular` | select | sim | Acer, Alcatel, Apple, ASUS, BlackBerry, BLU, HTC, Huawei, Lenovo, LG, Motorola, Nokia, OnePlus, Outros, Realme, Samsung, Sony, TCL, Vivo, Xiaomi, ZTE (21; não há tabela no dump) |
| `modelo_celular` | texto | sim | |
| `imei_celular` | número | não | |

### Dinheiro

| Campo | Tipo | Obrig. |
|---|---|---|
| `dinheiro_quantidade` | texto (valor em R$) | sim |

Exibição na aba: `R$ 37`.

### Outros

| Campo | Tipo | Obrig. |
|---|---|---|
| `outros_descricao` | texto | sim |
| `outros_quantidade` | número | sim |

## 2. Modal "Procedimento"

Formulário `POST /ocorrencias/ocorrencias-procedimento`. Campos ocultos:
`csrf_token_name`, `ocorrencia_id`, `servidor_id`, `opm_tipo`.

| Campo | Tipo | Obrig. | Origem dos valores |
|---|---|---|---|
| `procedimento` | select | sim | `sipom.procedimentos`: 1 = Inquérito Policial (IP), 2 = TCO, 3 = Boletim de Ocorrência (BO), 4 = Ato Infracional (o 5 = Processo existe na tabela, mas não aparece no formulário) |
| `reparticao` | select | sim | 1 = Polícia Militar, 2 = Polícia Civil, 3 = Polícia Federal |
| `procedimento_numero` | texto | sim | |
| `procedimento_ano` | texto | sim | |

Campos que dependem da repartição:

| Repartição | Campos |
|---|---|
| Polícia Militar | `opm` (select, 312 opções — as companhias, agrupadas por CRPM e batalhão; valor = id da companhia), `matricula_encarregado`, `nome_encarregado` (preenchido pela consulta de matrícula) |
| Polícia Civil | `procedimento_delegacia` (select, 289 — `delegacias` do dump de 05/10), `procedimento_delegado` (select, 542 — `delegados` do dump; não obrigatório) |
| Polícia Federal | `dpf` (delegacia, texto), `delf` (delegado, texto) |

Exibição na aba: `Repartição: Polícia Civil · Procedimento: Inquerito Policial - IP · Número/Ano: 674 / 2026 · Delegacia: 110-DELEGACIA DO 10. DISTRITO POLICIAL · Delegado(a): MARCO AURELIO EHMKE PIZZOLATTI`.

## 3. O que o relatório operacional traz, e o que falta para preencher

| Aba do SIPOM | O relatório traz | Para casar com o SIPOM |
|---|---|---|
| Arma | tipo, marca, modelo, calibre, numeração (texto livre: "REVOLVER", "TAURUS", "38") | de-para de tipo, marca e calibre para as listas; o que não casar vira pendência com escolha do analista |
| Munição | aparece dentro de "objetos apreendidos", em texto | sem extração estruturada hoje; entra como "Outros" ou o analista cadastra |
| Droga | descrição e gramas (e pacotes) | de-para do nome para a lista de 12; quantidade só em gramas — drogas em comprimido/dose/ml precisam do analista |
| Veículo | tipo, marca, modelo, placa, cor | tipo e cor pelas listas; marca/modelo pela tabela `marcasmodelos` (busca por texto); situação (apreendido/recuperado) não vem no relatório |
| Celular | só em "objetos apreendidos", em texto | sem extração estruturada hoje |
| Dinheiro | só em "objetos apreendidos", em texto | sem extração estruturada hoje |
| Procedimento | delegacia (sigla), delegado (nome), tipo (BO/IP/TCO), número com prefixo da delegacia | ver `sipom-integracao.md`, seção de procedimento: prefixo do número → código da delegacia; nome do delegado por aproximação; repartição = Polícia Civil |

## 4. No contrato do POST único (implementado, versão "2")

Dois blocos novos no payload da fase 1, com os ids das tabelas de referência
(nunca os tokens da tela). `procedimento` vai `null` quando o relatório não
trouxe procedimento — mas aí a ocorrência está pendente e não é enviada.
`delegado_id` pode ir `null`: o SIPOM não o exige. `materiais` leva um item
por arma, droga e veículo; munição, celular e dinheiro ainda não.

```json
"procedimento": {
  "reparticao": 2,
  "procedimento_id": 1, "procedimento": "Inquerito Policial - IP",
  "numero": "7653", "ano": "2026",
  "delegacia_id": 3, "delegacia": "939-CENTRAL DE PROCEDIMENTOS DIGITAIS",
  "delegado_id": 361, "delegado": "ITALO RENNO ALVES FEITOSA"
},
"materiais": [
  { "tipo_id": 1, "tipo": "Arma",
    "arma_tipo_id": 2, "arma_tipo": "Revolver", "arma_marca_id": 27, "arma_marca": "Taurus",
    "arma_calibre_id": 10, "arma_calibre": ".38", "numero": "GI63237", "descricao": "RT 85", "quantidade": 1 },
  { "tipo_id": 3, "tipo": "Droga", "droga": "Maconha", "unidade": "gramas (g)", "quantidade": 582 },
  { "tipo_id": 4, "tipo": "Veículo", "situacao": 1, "placa": "ABC1D23",
    "veiculo_tipo_codigo": 4, "veiculo_tipo": "MOTOCLICLETA",
    "marca_modelo_codigo": 2892, "marca_modelo": "HONDA/CG 160 FAN", "cor_codigo": 11, "cor": "PRETA" }
]
```

Para drogas não há tabela de ids no SIPOM: vai o nome e a unidade exatamente
como a lista da tela mostra.

## 5. Como o Tevunah resolve, e o que fica para o analista

| Campo | Automático | Pendência |
|---|---|---|
| Tipo do procedimento | BO, IP, TCO, AI pelo texto | `procedimento_tipo` (bloqueia) |
| Número/ano | `nnn-nnnn/aaaa` ou `nnnn/aaaa` | `procedimento_numero` (bloqueia; corrige pelo CORRIGIR) |
| Delegacia | código que abre o número do procedimento → catálogo; senão termo aprendido para o texto ("DMC"); senão "N° DP" → distrito | `delegacia` (bloqueia) |
| Delegado | termo aprendido; senão o nome abreviado contra o catálogo (palavras na ordem, inicial casa qualquer nome); um candidato é aceito, vários viram sugestão | `delegado` (aviso: não é obrigatório) |
| Arma | tipo por sinônimos ("REVOLVER", "FACA" → Branca, "ARTESANAL"), marca pelo nome ou primeira palavra do catálogo, calibre pelos dígitos (".38" = "38", "9MM" = "9mm") | `arma` (bloqueia) |
| Droga | nome por sinônimos; quantidade só quando o relatório traz gramas e a droga é medida em gramas | `droga` (bloqueia) |
| Veículo | tipo e cor por sinônimos (cor sem a vogal final: PRETO = PRETA), marca/modelo pela descrição exata "MARCA/MODELO" ou busca por palavras com um resultado só; situação recuperado quando a natureza é de recuperação | `veiculo` (bloqueia), com candidatos de marca/modelo |

Toda escolha do analista na ficha vira termo aprendido (`app.sipom_term_map`:
campo + texto do relatório → id) e vale para as próximas ocorrências com o
mesmo texto; o acervo é recalculado na hora.
