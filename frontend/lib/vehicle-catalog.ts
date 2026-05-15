// Catálogo de marcas e modelos de veículos pra evitar typo na digitação.
// Lista curada (não exaustiva) das principais marcas no Brasil + modelos
// canônicos por marca. Variações dentro do mesmo modelo são consolidadas
// (ex.: "Gol Flex Power" vira só "GOL"; "Onix Plus" vira só "ONIX").
//
// O Combobox aceita valor livre — o catálogo é só pra autocompletar e padronizar.
// Manter as listas em uppercase facilita o lookup e a renderização.
//
// Marcas: ordem alfabética (lookup rápido). Modelos: ordenados por
// popularidade no mercado brasileiro — os mais vendidos no topo do select.

export const VEHICLE_BRANDS: string[] = [
  "AUDI",
  "BMW",
  "BYD",
  "CAOA CHERY",
  "CHEVROLET",
  "CITROËN",
  "FIAT",
  "FORD",
  "HONDA",
  "HYUNDAI",
  "IVECO",
  "JAC",
  "JEEP",
  "KIA",
  "LAND ROVER",
  "MERCEDES-BENZ",
  "MITSUBISHI",
  "NISSAN",
  "PEUGEOT",
  "RAM",
  "RENAULT",
  "SCANIA",
  "SUZUKI",
  "TOYOTA",
  "VOLKSWAGEN",
  "VOLVO",
];

// Modelos por marca em formato canônico (sem suffixes de versão/motorização).
// Ordenados por popularidade — mais vendidos primeiro. Para marcas raras ou
// modelos antigos, o usuário digita livre.
export const VEHICLE_MODELS: Record<string, string[]> = {
  AUDI: ["A3", "Q3", "A4", "Q5", "A5", "Q7", "Q8", "A6", "A1", "A7", "A8", "TT", "RS3", "RS6", "S3"],
  BMW: ["320I", "X1", "118I", "120I", "X3", "X5", "330I", "X4", "X6", "115I", "X7", "M3", "M4", "Z4"],
  BYD: ["DOLPHIN", "SONG", "YUAN PLUS", "SEAL", "HAN", "TAN"],
  "CAOA CHERY": ["TIGGO 5X", "TIGGO 7", "TIGGO 8", "TIGGO 3X", "TIGGO 2", "ARRIZO 6", "ARRIZO 5", "ARRIZO 8"],
  CHEVROLET: [
    "ONIX", "TRACKER", "MONTANA", "S10", "SPIN", "CRUZE", "EQUINOX", "PRISMA",
    "COBALT", "CELTA", "CORSA", "AGILE", "CAMARO", "TRAILBLAZER", "BLAZER",
    "CAPTIVA", "SILVERADO", "SONIC", "ASTRA", "VECTRA", "MERIVA", "ZAFIRA",
    "MONZA", "OPALA", "CLASSIC",
  ],
  CITROËN: ["C3", "BASALT", "C4 CACTUS", "C4 LOUNGE", "C4", "C5 AIRCROSS", "AIRCROSS", "JUMPER", "JUMPY"],
  FIAT: [
    "STRADA", "ARGO", "MOBI", "PULSE", "TORO", "FASTBACK", "CRONOS", "FIORINO",
    "UNO", "PALIO", "SIENA", "GRAND SIENA", "DOBLÒ", "DUCATO", "PUNTO", "IDEA",
    "LINEA", "500", "BRAVO", "STILO", "TIPO", "FREEMONT", "MAREA",
  ],
  FORD: [
    "RANGER", "TERRITORY", "MAVERICK", "BRONCO", "MUSTANG", "KA", "ECOSPORT",
    "FIESTA", "FOCUS", "F-150", "F-250", "FUSION", "EDGE", "ESCAPE", "KUGA",
    "TRANSIT",
  ],
  HONDA: ["HR-V", "CITY", "CIVIC", "WR-V", "CR-V", "FIT", "ZR-V", "ACCORD"],
  HYUNDAI: [
    "HB20", "CRETA", "HB20S", "HB20X", "TUCSON", "IX35", "I30", "KONA",
    "SANTA FE", "ELANTRA", "AZERA", "SONATA", "VERACRUZ", "GENESIS",
  ],
  IVECO: ["DAILY", "TECTOR", "STRALIS", "EUROCARGO"],
  JAC: ["T40", "T60", "T80", "E-JS1", "E-JS4"],
  JEEP: ["COMPASS", "RENEGADE", "COMMANDER", "GRAND CHEROKEE", "WRANGLER", "CHEROKEE", "GLADIATOR"],
  KIA: ["SPORTAGE", "SELTOS", "STONIC", "SORENTO", "CARNIVAL", "CERATO", "PICANTO", "SOUL", "RIO", "BONGO", "MOHAVE", "OPTIMA", "K3", "CITATO"],
  "LAND ROVER": ["DISCOVERY SPORT", "EVOQUE", "DEFENDER", "RANGE ROVER SPORT", "DISCOVERY", "VELAR", "RANGE ROVER"],
  "MERCEDES-BENZ": ["C-CLASS", "GLA", "GLC", "A-CLASS", "CLA", "GLE", "GLB", "E-CLASS", "SPRINTER", "GLS", "S-CLASS", "CLS", "G-CLASS", "AMG GT"],
  MITSUBISHI: ["L200", "PAJERO SPORT", "ECLIPSE CROSS", "OUTLANDER", "ASX", "PAJERO", "LANCER"],
  NISSAN: ["KICKS", "FRONTIER", "VERSA", "SENTRA", "MARCH", "X-TRAIL", "LEAF"],
  PEUGEOT: ["208", "2008", "3008", "408", "308", "5008", "PARTNER"],
  RAM: ["RAMPAGE", "1500", "2500", "CLASSIC"],
  RENAULT: [
    "KWID", "DUSTER", "KARDIAN", "OROCH", "STEPWAY", "SANDERO", "LOGAN",
    "CAPTUR", "MASTER", "KANGOO", "CLIO", "FLUENCE", "MEGANE", "SCENIC",
    "SYMBOL",
  ],
  SCANIA: ["R-SERIES", "P-SERIES", "S-SERIES", "G-SERIES"],
  SUZUKI: ["JIMNY", "VITARA", "S-CROSS", "SWIFT"],
  TOYOTA: [
    "COROLLA", "COROLLA CROSS", "HILUX", "YARIS", "HILUX SW4", "RAV4", "ETIOS",
    "PRIUS", "BANDEIRANTE",
  ],
  VOLKSWAGEN: [
    "POLO", "NIVUS", "T-CROSS", "VIRTUS", "GOL", "SAVEIRO", "AMAROK", "TAOS",
    "JETTA", "VOYAGE", "UP", "GOLF", "FOX", "TIGUAN", "PASSAT", "FUSCA",
    "KOMBI", "BORA", "CROSSFOX", "PARATI", "SANTANA", "SPACEFOX", "TOUAREG",
  ],
  VOLVO: ["XC60", "XC40", "XC90", "EX30", "C40", "S60", "V60", "S90", "EX90"],
};

// ─────────────────────────── Motos ───────────────────────────
// Catálogo curado das principais marcas de moto no Brasil + modelos
// canônicos. Mesma filosofia do catálogo de carros: o Combobox aceita valor
// livre, isto é só pra autocompletar e padronizar. Modelos por popularidade.

export const MOTORCYCLE_BRANDS: string[] = [
  "BMW",
  "DAFRA",
  "DUCATI",
  "HAOJUE",
  "HARLEY-DAVIDSON",
  "HONDA",
  "KAWASAKI",
  "KTM",
  "ROYAL ENFIELD",
  "SHINERAY",
  "SUZUKI",
  "TRAXX",
  "TRIUMPH",
  "YAMAHA",
];

export const MOTORCYCLE_MODELS: Record<string, string[]> = {
  BMW: ["G 310 GS", "G 310 R", "F 850 GS", "R 1250 GS", "F 750 GS", "F 900 R", "S 1000 RR", "S 1000 R"],
  DAFRA: ["APACHE", "NH", "CITYCOM", "NEXT", "HORIZON", "RIVA", "SUPER 100"],
  DUCATI: ["MONSTER", "PANIGALE", "MULTISTRADA", "SCRAMBLER", "DIAVEL", "STREETFIGHTER"],
  HAOJUE: ["DK 160", "DK 150", "NK 150", "MASTER RIDE", "CHOPPER ROAD", "NEX"],
  "HARLEY-DAVIDSON": ["IRON 883", "FORTY-EIGHT", "FAT BOY", "SPORTSTER S", "STREET GLIDE", "ROAD KING", "PAN AMERICA"],
  HONDA: [
    "CG 160 FAN", "CG 160 TITAN", "CG 160 START", "BIZ", "POP 110", "PCX",
    "XRE 300", "BROS 160", "XRE 190", "CB 300", "ELITE 125", "ADV", "HORNET",
    "CB 500", "CB 650R", "CBR 650R", "SAHARA 300", "AFRICA TWIN",
  ],
  KAWASAKI: ["NINJA 400", "Z400", "NINJA 300", "Z650", "NINJA 650", "Z900", "VERSYS 650", "VERSYS-X 300", "NINJA ZX-10R"],
  KTM: ["DUKE 390", "DUKE 200", "ADVENTURE 390", "DUKE 790", "RC 390", "ADVENTURE 890"],
  "ROYAL ENFIELD": ["METEOR 350", "CLASSIC 350", "HUNTER 350", "HIMALAYAN", "INTERCEPTOR 650", "CONTINENTAL GT 650"],
  SHINERAY: ["JET", "PHOENIX", "WORKER", "XY 50Q"],
  SUZUKI: ["GIXXER", "BURGMAN", "V-STROM", "GSX-S", "GSX-R", "YES", "BANDIT", "INTRUDER"],
  TRAXX: ["WORK 125", "JH 50", "STAR 50"],
  TRIUMPH: ["STREET TRIPLE", "TRIDENT 660", "TIGER 900", "SPEED TRIPLE", "BONNEVILLE", "SCRAMBLER", "TIGER 1200"],
  YAMAHA: [
    "FACTOR 150", "FAZER 150", "NMAX", "FAZER 250", "FZ25", "XTZ 150 CROSSER",
    "NEO", "MT-03", "XTZ 250 LANDER", "YBR", "R3", "R15", "MT-07", "MT-09",
    "XTZ 250 TÉNÉRÉ",
  ],
};

/**
 * Marcas disponíveis para a categoria de veículo (carro ou moto).
 */
export function brandsForCategory(category: "car" | "motorcycle"): string[] {
  return category === "motorcycle" ? MOTORCYCLE_BRANDS : VEHICLE_BRANDS;
}

/**
 * Devolve a lista de modelos para a marca informada dentro da categoria. Se a
 * marca não estiver no catálogo (digitação livre), devolve array vazio — o
 * Combobox apenas libera digitação sem sugestões.
 */
export function modelsForBrand(
  brand: string,
  category: "car" | "motorcycle" = "car",
): string[] {
  if (!brand) return [];
  const key = brand.trim().toUpperCase();
  const table = category === "motorcycle" ? MOTORCYCLE_MODELS : VEHICLE_MODELS;
  return table[key] ?? [];
}

// Cores no padrão dos DETRANs brasileiros — ordenadas pela frequência real
// na frota nacional (branca/prata/preta dominam ~90% dos emplacamentos).
export const VEHICLE_COLORS: string[] = [
  "BRANCA",
  "PRATA",
  "PRETA",
  "CINZA",
  "VERMELHA",
  "AZUL",
  "VERDE",
  "MARROM",
  "BEGE",
  "AMARELA",
  "DOURADA",
  "LARANJA",
  "GRENÁ",
  "ROXA",
  "ROSA",
];

// ─────────────────────────── Placa ───────────────────────────

// Placa brasileira válida em dois layouts, ambos com 7 caracteres:
//  - Antigo  : LLL NNNN  → 3 letras + 4 dígitos        (ex.: ABC1234)
//  - Mercosul: LLL NLNN  → 3 letras, dígito, letra, 2 dígitos (ex.: ABC1D23)
// A 5ª posição é o discriminante: dígito = antigo, letra = Mercosul. O regex
// abaixo cobre os dois: [0-9A-Z] na posição 5.
const PLATE_REGEX = /^[A-Z]{3}[0-9][0-9A-Z][0-9]{2}$/;

/**
 * Normaliza entrada de placa: uppercase, remove tudo que não é alfanumérico
 * e limita a 7 caracteres. Usado no onChange do input.
 */
export function normalizePlateInput(raw: string): string {
  return raw.toUpperCase().replace(/[^A-Z0-9]/g, "").slice(0, 7);
}

/**
 * Valida se a placa (já normalizada ou não) está em um dos formatos
 * brasileiros — antigo ou Mercosul.
 */
export function isValidPlate(plate: string): boolean {
  return PLATE_REGEX.test(normalizePlateInput(plate));
}
