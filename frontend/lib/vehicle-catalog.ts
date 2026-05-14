// Catálogo de marcas e modelos de veículos pra evitar typo na digitação.
// Lista curada (não exaustiva) das principais marcas no Brasil + modelos
// canônicos por marca. Variações dentro do mesmo modelo são consolidadas
// (ex.: "Gol Flex Power" vira só "GOL"; "Onix Plus" vira só "ONIX").
//
// O Combobox aceita valor livre — o catálogo é só pra autocompletar e padronizar.
// Manter as listas em uppercase facilita o lookup e a renderização.

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
// Para marcas raras ou modelos antigos, o usuário digita livre.
export const VEHICLE_MODELS: Record<string, string[]> = {
  AUDI: ["A1", "A3", "A4", "A5", "A6", "A7", "A8", "Q3", "Q5", "Q7", "Q8", "RS3", "RS6", "S3", "TT"],
  BMW: ["115I", "118I", "120I", "320I", "330I", "M3", "M4", "X1", "X3", "X4", "X5", "X6", "X7", "Z4"],
  BYD: ["DOLPHIN", "HAN", "SEAL", "SONG", "TAN", "YUAN PLUS"],
  "CAOA CHERY": ["ARRIZO 5", "ARRIZO 6", "ARRIZO 8", "TIGGO 2", "TIGGO 3X", "TIGGO 5X", "TIGGO 7", "TIGGO 8"],
  CHEVROLET: [
    "AGILE", "ASTRA", "BLAZER", "CAMARO", "CAPTIVA", "CELTA", "CLASSIC",
    "COBALT", "CORSA", "CRUZE", "EQUINOX", "MERIVA", "MONTANA", "MONZA",
    "ONIX", "OPALA", "PRISMA", "S10", "SILVERADO", "SONIC", "SPIN",
    "TRACKER", "TRAILBLAZER", "VECTRA", "ZAFIRA",
  ],
  CITROËN: ["AIRCROSS", "BASALT", "C3", "C4", "C4 CACTUS", "C4 LOUNGE", "C5 AIRCROSS", "JUMPER", "JUMPY"],
  FIAT: [
    "500", "ARGO", "BRAVO", "CRONOS", "DOBLÒ", "DUCATO", "FASTBACK",
    "FIORINO", "FREEMONT", "GRAND SIENA", "IDEA", "LINEA", "MAREA",
    "MOBI", "PALIO", "PUNTO", "SIENA", "STILO", "STRADA", "TIPO",
    "TORO", "UNO", "PULSE",
  ],
  FORD: [
    "BRONCO", "ECOSPORT", "EDGE", "ESCAPE", "F-150", "F-250", "FIESTA",
    "FOCUS", "FUSION", "KA", "KUGA", "MAVERICK", "MUSTANG", "RANGER",
    "TERRITORY", "TRANSIT",
  ],
  HONDA: ["ACCORD", "CITY", "CIVIC", "CR-V", "FIT", "HR-V", "WR-V", "ZR-V"],
  HYUNDAI: [
    "AZERA", "CRETA", "ELANTRA", "GENESIS", "HB20", "HB20S", "HB20X",
    "I30", "IX35", "KONA", "SANTA FE", "SONATA", "TUCSON", "VERACRUZ",
  ],
  IVECO: ["DAILY", "TECTOR", "STRALIS", "EUROCARGO"],
  JAC: ["E-JS1", "E-JS4", "T40", "T60", "T80"],
  JEEP: ["CHEROKEE", "COMMANDER", "COMPASS", "GLADIATOR", "GRAND CHEROKEE", "RENEGADE", "WRANGLER"],
  KIA: ["BONGO", "CARNIVAL", "CERATO", "CITATO", "K3", "MOHAVE", "OPTIMA", "PICANTO", "RIO", "SELTOS", "SORENTO", "SOUL", "SPORTAGE", "STONIC"],
  "LAND ROVER": ["DEFENDER", "DISCOVERY", "DISCOVERY SPORT", "EVOQUE", "RANGE ROVER", "RANGE ROVER SPORT", "VELAR"],
  "MERCEDES-BENZ": ["A-CLASS", "AMG GT", "C-CLASS", "CLA", "CLS", "E-CLASS", "G-CLASS", "GLA", "GLB", "GLC", "GLE", "GLS", "S-CLASS", "SPRINTER"],
  MITSUBISHI: ["ASX", "ECLIPSE CROSS", "L200", "LANCER", "OUTLANDER", "PAJERO", "PAJERO SPORT"],
  NISSAN: ["FRONTIER", "KICKS", "LEAF", "MARCH", "SENTRA", "VERSA", "X-TRAIL"],
  PEUGEOT: ["2008", "208", "3008", "308", "408", "5008", "PARTNER"],
  RAM: ["1500", "2500", "CLASSIC", "RAMPAGE"],
  RENAULT: [
    "CAPTUR", "CLIO", "DUSTER", "FLUENCE", "KANGOO", "KARDIAN", "KWID",
    "LOGAN", "MASTER", "MEGANE", "OROCH", "SANDERO", "SCENIC", "STEPWAY",
    "SYMBOL",
  ],
  SCANIA: ["G-SERIES", "P-SERIES", "R-SERIES", "S-SERIES"],
  SUZUKI: ["JIMNY", "S-CROSS", "SWIFT", "VITARA"],
  TOYOTA: [
    "BANDEIRANTE", "COROLLA", "COROLLA CROSS", "ETIOS", "HILUX", "HILUX SW4",
    "PRIUS", "RAV4", "YARIS",
  ],
  VOLKSWAGEN: [
    "AMAROK", "BORA", "CROSSFOX", "FOX", "FUSCA", "GOL", "GOLF", "JETTA",
    "KOMBI", "NIVUS", "PARATI", "PASSAT", "POLO", "SANTANA", "SAVEIRO",
    "SPACEFOX", "T-CROSS", "TAOS", "TIGUAN", "TOUAREG", "UP", "VIRTUS",
    "VOYAGE",
  ],
  VOLVO: ["C40", "EX30", "EX90", "S60", "S90", "V60", "XC40", "XC60", "XC90"],
};

/**
 * Devolve a lista de modelos para a marca informada. Se a marca não estiver
 * no catálogo (digitação livre), devolve array vazio — o Combobox apenas
 * libera digitação sem sugestões.
 */
export function modelsForBrand(brand: string): string[] {
  if (!brand) return [];
  const key = brand.trim().toUpperCase();
  return VEHICLE_MODELS[key] ?? [];
}
