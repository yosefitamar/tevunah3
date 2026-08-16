"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { listIncidentLocations, type PlaceFacet } from "./incidents-api";

export type IncidentLocationsState = {
  /** Municípios com ocorrência registrada, ordenados por volume. */
  cities: PlaceFacet[];
  /** Bairros por município (ordenados por volume). */
  neighborhoodsOf: (city: string) => string[];
  /** Todos os bairros, para quando nenhum município está selecionado. */
  allNeighborhoods: string[];
  reload: () => void;
};

/**
 * Carrega os municípios/bairros presentes no acervo de ocorrências. Serve
 * tanto para o recorte territorial do mapa quanto para o autocompletar de
 * bairro no cadastro — em ambos os casos o que interessa é o que já existe,
 * não um cadastro universal de bairros.
 */
export function useIncidentLocations(): IncidentLocationsState {
  const [cities, setCities] = useState<PlaceFacet[]>([]);
  const [neighborhoods, setNeighborhoods] = useState<PlaceFacet[]>([]);

  const reload = useCallback(() => {
    listIncidentLocations()
      .then((r) => {
        setCities([...r.cities].sort((a, b) => b.count - a.count));
        setNeighborhoods(r.neighborhoods);
      })
      // Silencioso: sem sugestões a tela segue utilizável (campo é livre).
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  const byCity = useMemo(() => {
    const m = new Map<string, PlaceFacet[]>();
    for (const f of neighborhoods) {
      const arr = m.get(f.city);
      if (arr) arr.push(f);
      else m.set(f.city, [f]);
    }
    for (const arr of m.values()) arr.sort((a, b) => b.count - a.count);
    return m;
  }, [neighborhoods]);

  const allNeighborhoods = useMemo(() => {
    const seen = new Set<string>();
    return [...neighborhoods]
      .sort((a, b) => b.count - a.count)
      .map((f) => f.neighborhood ?? "")
      .filter((n) => n !== "" && !seen.has(n) && (seen.add(n), true));
  }, [neighborhoods]);

  const neighborhoodsOf = useCallback(
    (city: string) =>
      city
        ? (byCity.get(city) ?? []).map((f) => f.neighborhood ?? "")
        : allNeighborhoods,
    [byCity, allNeighborhoods],
  );

  return { cities, neighborhoodsOf, allNeighborhoods, reload };
}
