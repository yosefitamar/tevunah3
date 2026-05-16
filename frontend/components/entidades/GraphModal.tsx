"use client";

// GraphModal — visualização full-screen do subgrafo de vínculos de uma entidade.
// Cada nó é um cartão 3:4 com foto + nome + vulgo + ORCRIM. Vínculos
// pessoa↔pessoa têm peso visual maior. Clicar em ABRIR DOSSIÊ abre um
// EntidadeDrawer sobre o modal — o grafo continua intacto no fundo.
//
// Carrega N=2 saltos por padrão (ajustável 1/2/3). Layout dagre top-down.

import { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import dynamic from "next/dynamic";
import dagre from "dagre";
import {
  Background,
  BackgroundVariant,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
  type ReactFlowInstance,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Bike, Building2, Car, MapPin, Network, RotateCw, UserCircle2, X } from "lucide-react";
import {
  CLASSIFICATION_LABEL,
  CLASSIFICATION_PILL,
  ENTITY_KIND_LABEL,
  RELATION_LABEL,
  vehicleListLabel,
  type EntityGraph,
  type EntityKind,
  type GraphEdge,
  type GraphNode,
} from "@/lib/entities-types";
import { getEntityGraph, photoURL, type GraphDepth } from "@/lib/entities-api";

// EntidadeDrawer importa GraphModal — quebramos o ciclo via dynamic, que
// carrega o drawer em um chunk separado on-demand.
const EntidadeDrawer = dynamic(() => import("./EntidadeDrawer"), {
  ssr: false,
});

type Props = {
  entityId: string;
  entityName: string;
  onClose: () => void;
  // Mantido por compat — não usamos mais nesta versão porque o dossiê abre
  // sobre o próprio modal em vez de delegar pro parent.
  onOpenEntity?: (id: string) => void;
};

// Cartão 3:4 (140×186) — foto domina o topo, com 3 linhas de metadado abaixo.
const NODE_W = 150;
const NODE_H = 200;
// Nó compacto p/ propriedades (veículo/endereço) promovidas. Ícone grande no
// topo e 1–2 linhas de identificação abaixo.
const ASSET_W = 110;
const ASSET_H = 96;

// Tipos de propriedade que respondem aos toggles do header. Person/org são
// sempre renderizados como cartão grande.
type AssetKind = "vehicle" | "place";
type AssetToggles = Record<AssetKind, boolean>;

export default function GraphModal({ entityId, entityName, onClose }: Props) {
  const [centerId, setCenterId] = useState(entityId);
  const [centerName, setCenterName] = useState(entityName);
  const [depth, setDepth] = useState<GraphDepth>(2);
  const [graph, setGraph] = useState<EntityGraph | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  // Toggles de promoção de propriedades a nó. OFF (default) = propriedade
  // sempre vira pill no cartão da pessoa-dona; ON = compartilhadas (2+ donos
  // visíveis) viram nó compacto, as exclusivas continuam pill.
  const [assetToggles, setAssetToggles] = useState<AssetToggles>({
    vehicle: false,
    place: false,
  });
  // Instância capturada via onInit pra disparar fitView() em runtime quando
  // os toggles mudam (a prop fitView só atua no mount).
  const [rfInstance, setRfInstance] = useState<ReactFlowInstance | null>(null);
  // dossierId aberto = drawer sobreposto. Não fecha o modal — quando o
  // usuário fecha o dossiê, volta direto pro grafo no mesmo estado.
  const [dossierId, setDossierId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    getEntityGraph(centerId, depth)
      .then((g) => {
        if (cancelled) return;
        setGraph(g);
        const center = g.nodes.find((n) => n.id === g.center_id);
        if (center) setCenterName(center.name);
      })
      .catch((e: { message?: string }) => {
        if (cancelled) return;
        setError(e?.message ?? "Erro ao carregar grafo");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [centerId, depth]);

  // ESC fecha em camadas: dossier → painel lateral → modal.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      if (dossierId) {
        // O drawer trata ESC sozinho — não interferimos aqui pra evitar
        // fechar duas camadas com uma tecla.
        return;
      }
      if (selectedId) setSelectedId(null);
      else onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [selectedId, dossierId, onClose]);

  const { rfNodes, rfEdges, pillsByOwner } = useMemo(
    () => buildFlow(graph, centerId, assetToggles, setSelectedId),
    [graph, centerId, assetToggles],
  );

  const onNodeClick = useCallback(
    (_: unknown, node: Node) => setSelectedId(node.id),
    [],
  );

  // Se o nó selecionado sumiu do grafo (recentragem ou mudança de profundidade
  // o removeu por completo), limpa a seleção. Importante: assets demovidos a
  // pill *continuam* em graph.nodes — só não estão em rfNodes — e o usuário
  // pode ter selecionado um deles clicando na pill; nesse caso preservamos.
  useEffect(() => {
    if (!selectedId) return;
    if (graph && !graph.nodes.some((n) => n.id === selectedId)) {
      setSelectedId(null);
    }
  }, [graph, selectedId]);

  // Recentra o viewport ao alternar VEÍCULOS/ENDEREÇOS — o conjunto de nós
  // muda de tamanho/quantidade e o fit anterior fica desalinhado. RAF dá
  // tempo do ReactFlow aplicar os novos rfNodes antes do cálculo do bounds.
  useEffect(() => {
    if (!rfInstance) return;
    const raf = requestAnimationFrame(() => {
      rfInstance.fitView({ padding: 0.2, maxZoom: 1.1, duration: 280 });
    });
    return () => cancelAnimationFrame(raf);
  }, [assetToggles, rfInstance]);

  const selectedNode = useMemo(() => {
    if (!graph || !selectedId) return null;
    return graph.nodes.find((n) => n.id === selectedId) ?? null;
  }, [graph, selectedId]);

  if (typeof document === "undefined") return null;

  return createPortal(
    // stopPropagation: o GraphModal é portado pra document.body, mas o evento
    // sintético do React ainda sobe pela árvore — sem isso, cliques no modal
    // chegariam ao .drawer-backdrop do dossiê original e o fechariam.
    <div
      className="graph-modal"
      role="dialog"
      aria-modal="true"
      onClick={(e) => e.stopPropagation()}
    >
      <div className="graph-modal__header">
        <div className="graph-modal__title">
          <Network size={14} strokeWidth={1.8} />
          <span>GRAFO · {centerName.toUpperCase()}</span>
          {graph?.truncated && (
            <span className="graph-modal__trunc">
              · TRUNCADO ({graph.nodes.length} NÓS)
            </span>
          )}
        </div>
        <div className="graph-modal__ctrls">
          <span className="graph-modal__label">CAMADAS</span>
          <button
            type="button"
            className={`quick-chip${assetToggles.vehicle ? " quick-chip--on" : ""}`}
            onClick={() =>
              setAssetToggles((t) => ({ ...t, vehicle: !t.vehicle }))
            }
            aria-pressed={assetToggles.vehicle}
            title="Mostra veículos como nó no grafo (default: pill no cartão da pessoa)"
          >
            <Car size={11} strokeWidth={1.8} />
            <span>VEÍCULOS</span>
          </button>
          <button
            type="button"
            className={`quick-chip${assetToggles.place ? " quick-chip--on" : ""}`}
            onClick={() => setAssetToggles((t) => ({ ...t, place: !t.place }))}
            aria-pressed={assetToggles.place}
            title="Mostra endereços como nó no grafo (default: pill no cartão da pessoa)"
          >
            <MapPin size={11} strokeWidth={1.8} />
            <span>ENDEREÇOS</span>
          </button>
          <span className="graph-modal__ctrls-sep" aria-hidden="true" />
          <span className="graph-modal__label">PROFUNDIDADE</span>
          {([1, 2, 3] as GraphDepth[]).map((d) => (
            <button
              key={d}
              type="button"
              className={`quick-chip${depth === d ? " quick-chip--on" : ""}`}
              onClick={() => setDepth(d)}
              aria-pressed={depth === d}
            >
              {d}
            </button>
          ))}
          <button
            type="button"
            className="action-btn"
            onClick={onClose}
            aria-label="Fechar"
          >
            <X size={14} />
          </button>
        </div>
      </div>

      <div className="graph-modal__body">
        <div className="graph-modal__canvas">
          {loading && (
            <div className="graph-modal__state muted">// CARREGANDO REDE…</div>
          )}
          {error && (
            <div className="graph-modal__state banner banner-error">
              ⚠ {error}
            </div>
          )}
          {!loading && !error && rfNodes.length > 0 && (
            <ReactFlow
              nodes={rfNodes}
              edges={rfEdges}
              nodeTypes={nodeTypes}
              onInit={setRfInstance}
              onNodeClick={onNodeClick}
              onPaneClick={() => setSelectedId(null)}
              fitView
              fitViewOptions={{ padding: 0.2, maxZoom: 1.1 }}
              proOptions={{ hideAttribution: true }}
              nodesDraggable
              nodesConnectable={false}
              elementsSelectable
              minZoom={0.2}
              maxZoom={2}
            >
              <Background
                color="var(--line)"
                gap={28}
                size={1}
                variant={BackgroundVariant.Dots}
              />
              <Controls
                position="bottom-right"
                showInteractive={false}
                className="graph-modal__controls"
              />
            </ReactFlow>
          )}
        </div>

        {selectedNode && (
          <aside className="graph-modal__side">
            <div className="graph-modal__side-hd">
              <span>NÓ SELECIONADO</span>
              <button
                type="button"
                className="action-btn"
                onClick={() => setSelectedId(null)}
                aria-label="Fechar painel"
              >
                <X size={12} />
              </button>
            </div>
            <SidePanel
              node={selectedNode}
              isCenter={selectedNode.id === centerId}
              pills={pillsByOwner.get(selectedNode.id) ?? []}
              onRecenter={() => {
                setCenterId(selectedNode.id);
                setSelectedId(null);
              }}
              onOpenDossier={() => setDossierId(selectedNode.id)}
            />
          </aside>
        )}
      </div>

      {/* Dossier overlay — fica sobre o grafo, preservando o estado abaixo.
          Fechar o drawer mantém o grafo intacto exatamente como estava. */}
      {dossierId && (
        <div className="graph-modal__dossier-layer">
          <EntidadeDrawer
            entityId={dossierId}
            onClose={() => setDossierId(null)}
            onChanged={() => {
              // Edições no dossiê sobreposto podem ter alterado vínculos —
              // recarrega o grafo pra refletir o estado atual.
              getEntityGraph(centerId, depth)
                .then((g) => setGraph(g))
                .catch(() => {});
            }}
            onOpenEntity={(id) => setDossierId(id)}
          />
        </div>
      )}
    </div>,
    document.body,
  );
}

// ───────────────────────── Painel lateral ─────────────────────────

function SidePanel({
  node,
  isCenter,
  pills,
  onRecenter,
  onOpenDossier,
}: {
  node: GraphNode;
  isCenter: boolean;
  pills: AssetPill[];
  onRecenter: () => void;
  onOpenDossier: () => void;
}) {
  const label =
    node.kind === "vehicle" ? vehicleListLabel(node.vehicle, node.name) : node.name;
  return (
    <div className="graph-modal__side-bd">
      <div className="graph-side__kind">{ENTITY_KIND_LABEL[node.kind]}</div>
      <div className="graph-side__name">{label.toUpperCase()}</div>
      {node.alias && (
        <div className="graph-side__alias">VULGO: {node.alias.toUpperCase()}</div>
      )}
      {node.orcrim_alias && (
        <div className="graph-side__orcrim">
          ORCRIM: {node.orcrim_alias.toUpperCase()}
        </div>
      )}
      <div className="graph-side__row">
        <span className={"pill " + CLASSIFICATION_PILL[node.classification]}>
          {CLASSIFICATION_LABEL[node.classification]}
        </span>
        {isCenter && <span className="graph-side__hint">· CENTRO ATUAL</span>}
      </div>

      {pills.length > 0 && (
        <div className="graph-side__assets">
          <div className="graph-side__assets-hd">
            PROPRIEDADES ({pills.length})
          </div>
          <ul className="graph-side__assets-list">
            {pills.map((p) => (
              <li
                key={p.id}
                className={`graph-side__asset graph-side__asset--${p.kind}`}
              >
                <PillIcon kind={p.kind} vehicleCategory={p.vehicleCategory} />
                <span>{p.label.toUpperCase()}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="graph-side__actions">
        <button
          type="button"
          className="btn"
          onClick={onRecenter}
          disabled={isCenter}
        >
          <RotateCw size={12} strokeWidth={1.8} />
          <span>RECENTRAR GRAFO</span>
        </button>
        <button type="button" className="btn btn--primary" onClick={onOpenDossier}>
          ABRIR DOSSIÊ
        </button>
      </div>
    </div>
  );
}

// ───────────────────────── Node component ─────────────────────────

type AssetPill = {
  id: string;
  kind: AssetKind;
  label: string;        // texto curto (placa OU primeira linha)
  vehicleCategory?: "car" | "motorcycle";
};

type EntityNodeData = {
  kind: EntityKind;
  name: string;
  alias?: string;
  orcrim?: string;
  photoUrl?: string;
  vehicleCategory?: "car" | "motorcycle";
  isCenter: boolean;
  pills: AssetPill[];   // assets demovidos exibidos no rodapé do cartão
  onPillClick?: (assetID: string) => void;
};

type CompactNodeData = {
  kind: AssetKind;
  label: string;
  sublabel?: string;
  vehicleCategory?: "car" | "motorcycle";
  isCenter: boolean;
  ownerCount: number;   // ≥2 quando compartilhado entre múltiplos donos
};

function PlaceholderIcon({ kind, vehicleCategory }: {
  kind: EntityKind;
  vehicleCategory?: "car" | "motorcycle";
}) {
  if (kind === "vehicle") {
    return vehicleCategory === "motorcycle" ? (
      <Bike size={28} strokeWidth={1.2} />
    ) : (
      <Car size={28} strokeWidth={1.2} />
    );
  }
  if (kind === "organization") return <Building2 size={28} strokeWidth={1.2} />;
  if (kind === "place") return <MapPin size={28} strokeWidth={1.2} />;
  return <UserCircle2 size={28} strokeWidth={1.2} />;
}

function PillIcon({ kind, vehicleCategory }: {
  kind: AssetKind;
  vehicleCategory?: "car" | "motorcycle";
}) {
  if (kind === "vehicle") {
    return vehicleCategory === "motorcycle" ? (
      <Bike size={10} strokeWidth={2} />
    ) : (
      <Car size={10} strokeWidth={2} />
    );
  }
  return <MapPin size={10} strokeWidth={2} />;
}

const MAX_VISIBLE_PILLS = 3;

function EntityNode({ data }: NodeProps) {
  const d = data as unknown as EntityNodeData;
  const visiblePills = d.pills.slice(0, MAX_VISIBLE_PILLS);
  const extraPills = d.pills.length - visiblePills.length;
  return (
    <div
      className={`graph-node graph-node--${d.kind}${d.isCenter ? " graph-node--center" : ""}`}
    >
      {/* Handles laterais: usados exclusivamente por sibling/half_sibling.
          IDs explícitos pra evitar que ReactFlow as escolha como default em
          edges sem sourceHandle/targetHandle. */}
      <Handle type="target" position={Position.Left} id="lt" className="graph-node__handle" />
      <Handle type="source" position={Position.Left} id="ls" className="graph-node__handle" />
      <Handle type="target" position={Position.Right} id="rt" className="graph-node__handle" />
      <Handle type="source" position={Position.Right} id="rs" className="graph-node__handle" />
      <Handle type="target" position={Position.Top} id="tt" className="graph-node__handle" />
      <div className="graph-node__photo">
        {d.photoUrl ? (
          <img src={d.photoUrl} alt="" loading="lazy" draggable={false} />
        ) : (
          <div className="graph-node__photo-empty">
            <PlaceholderIcon kind={d.kind} vehicleCategory={d.vehicleCategory} />
          </div>
        )}
      </div>
      <div className="graph-node__meta">
        <div className="graph-node__name" title={d.name}>
          {d.name.toUpperCase()}
        </div>
        {d.alias && (
          <div className="graph-node__alias" title={d.alias}>
            VULGO {d.alias.toUpperCase()}
          </div>
        )}
        {d.orcrim && (
          <div className="graph-node__orcrim" title={d.orcrim}>
            ORCRIM {d.orcrim.toUpperCase()}
          </div>
        )}
        {d.pills.length > 0 && (
          <div className="graph-node__pills">
            {visiblePills.map((p) => (
              <button
                key={p.id}
                type="button"
                className={`graph-pill graph-pill--${p.kind}`}
                title={p.label}
                // stopPropagation: o clique não pode "vazar" pro ReactFlow,
                // senão selecionaria o cartão da pessoa em vez do asset.
                onClick={(e) => {
                  e.stopPropagation();
                  d.onPillClick?.(p.id);
                }}
                onMouseDown={(e) => e.stopPropagation()}
              >
                <PillIcon kind={p.kind} vehicleCategory={p.vehicleCategory} />
                <span className="graph-pill__txt">{p.label.toUpperCase()}</span>
              </button>
            ))}
            {extraPills > 0 && (
              <span className="graph-pill graph-pill--more" title={`Mais ${extraPills}`}>
                +{extraPills}
              </span>
            )}
          </div>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} id="bs" className="graph-node__handle" />
    </div>
  );
}

function CompactAssetNode({ data }: NodeProps) {
  const d = data as unknown as CompactNodeData;
  const shared = d.ownerCount >= 2;
  return (
    <div
      className={`graph-asset graph-asset--${d.kind}${d.isCenter ? " graph-asset--center" : ""}${shared ? " graph-asset--shared" : ""}`}
    >
      {shared && (
        <span className="graph-asset__badge" title={`${d.ownerCount} donos vinculados`}>
          {d.ownerCount}×
        </span>
      )}
      <Handle type="target" position={Position.Top} id="tt" className="graph-node__handle" />
      <div className="graph-asset__icon">
        {d.kind === "vehicle" ? (
          d.vehicleCategory === "motorcycle" ? (
            <Bike size={22} strokeWidth={1.4} />
          ) : (
            <Car size={22} strokeWidth={1.4} />
          )
        ) : (
          <MapPin size={22} strokeWidth={1.4} />
        )}
      </div>
      <div className="graph-asset__meta">
        <div className="graph-asset__label" title={d.label}>
          {d.label.toUpperCase()}
        </div>
        {d.sublabel && (
          <div className="graph-asset__sublabel" title={d.sublabel}>
            {d.sublabel.toUpperCase()}
          </div>
        )}
      </div>
      <Handle type="source" position={Position.Bottom} id="bs" className="graph-node__handle" />
    </div>
  );
}

const nodeTypes = { entity: EntityNode, asset: CompactAssetNode };

// ───────────────────────── Layout (dagre) ─────────────────────────

// Rótulo curto pra pill/nó compacto:
//   - veículo: placa, ou marca+modelo se não houver placa
//   - endereço: nome livre (primeira parte antes de vírgula)
function shortAssetLabel(n: GraphNode): string {
  if (n.kind === "vehicle") {
    const plate = n.vehicle?.plate?.trim();
    if (plate) return plate;
    const brand = n.vehicle?.brand?.trim();
    const model = n.vehicle?.model?.trim();
    const head = [brand, model].filter(Boolean).join(" ");
    return head || n.name;
  }
  return n.name.split(",")[0]?.trim() || n.name;
}

function compactSublabel(n: GraphNode): string | undefined {
  if (n.kind === "vehicle") {
    const plate = n.vehicle?.plate?.trim();
    if (!plate) return undefined;
    const brand = n.vehicle?.brand?.trim();
    const model = n.vehicle?.model?.trim();
    const head = [brand, model].filter(Boolean).join(" ");
    return head || undefined;
  }
  return undefined;
}

function buildFlow(
  graph: EntityGraph | null,
  centerId: string,
  assetToggles: AssetToggles,
  onPillClick: (assetID: string) => void,
): { rfNodes: Node[]; rfEdges: Edge[]; pillsByOwner: Map<string, AssetPill[]> } {
  if (!graph) return { rfNodes: [], rfEdges: [], pillsByOwner: new Map() };

  // Mapa id→kind pra estilizar as edges com base no par de pontas.
  const kindByID = new Map<string, EntityKind>();
  const nodeByID = new Map<string, GraphNode>();
  for (const n of graph.nodes) {
    kindByID.set(n.id, n.kind);
    nodeByID.set(n.id, n);
  }

  // ─── Decisão de promoção/demoção dos assets ────────────────────────
  //
  // Para cada vehicle/place (≠ centro):
  //   - levanta donos = vizinhos cujo kind ∈ {person, organization}
  //   - se toggle do tipo está ON E há ≥2 donos → vira NÓ COMPACTO
  //   - senão, se há ≥1 dono → vira PILL no cartão do primeiro dono
  //     (preferindo o centro, depois pessoas, depois orgs)
  //   - senão (nenhum dono visível) → some
  //
  // Edges que terminam em assets demovidos são descartadas; nas que viram
  // nó compacto, mantemos a aresta normalmente.
  const ownersByAsset = new Map<string, string[]>();
  for (const e of graph.edges) {
    const fk = kindByID.get(e.from);
    const tk = kindByID.get(e.to);
    const fromIsAsset = fk === "vehicle" || fk === "place";
    const toIsAsset = tk === "vehicle" || tk === "place";
    if (fromIsAsset && (tk === "person" || tk === "organization")) {
      ownersByAsset.set(e.from, [...(ownersByAsset.get(e.from) ?? []), e.to]);
    } else if (toIsAsset && (fk === "person" || fk === "organization")) {
      ownersByAsset.set(e.to, [...(ownersByAsset.get(e.to) ?? []), e.from]);
    }
  }

  const demotedAssetIDs = new Set<string>();   // viram pill
  const droppedAssetIDs = new Set<string>();   // somem
  const pillOwnerOf = new Map<string, string>(); // assetID → personID que recebe a pill
  const ownerCountOf = new Map<string, number>(); // assetID → nº de donos visíveis

  for (const n of graph.nodes) {
    if (n.kind !== "vehicle" && n.kind !== "place") continue;
    if (n.id === centerId) continue;
    const owners = ownersByAsset.get(n.id) ?? [];
    const uniqueOwners = Array.from(new Set(owners));
    ownerCountOf.set(n.id, uniqueOwners.length);
    const toggleOn = assetToggles[n.kind];
    // Toggle ON → asset SEMPRE vira nó (compartilhamento só vira badge).
    if (toggleOn) continue;
    // Toggle OFF e sem dono → some (nenhum cartão pra hospedar a pill).
    if (uniqueOwners.length === 0) {
      droppedAssetIDs.add(n.id);
      continue;
    }
    // Toggle OFF: vira pill. Escolhe um dono pra hospedar.
    // Preferência: centro > primeira pessoa > primeira org.
    let owner: string | undefined;
    if (uniqueOwners.includes(centerId)) owner = centerId;
    if (!owner) owner = uniqueOwners.find((id) => kindByID.get(id) === "person");
    if (!owner) owner = uniqueOwners[0];
    if (owner) {
      demotedAssetIDs.add(n.id);
      pillOwnerOf.set(n.id, owner);
    } else {
      droppedAssetIDs.add(n.id);
    }
  }

  // Coleta pills por dono pra renderizar no cartão e no painel lateral.
  const pillsByOwner = new Map<string, AssetPill[]>();
  for (const [assetID, ownerID] of pillOwnerOf) {
    const a = nodeByID.get(assetID);
    if (!a) continue;
    const pill: AssetPill = {
      id: assetID,
      kind: a.kind as AssetKind,
      label: shortAssetLabel(a),
      vehicleCategory: a.vehicle?.category,
    };
    pillsByOwner.set(ownerID, [...(pillsByOwner.get(ownerID) ?? []), pill]);
  }

  // Conjunto final de nós que vão pro grafo.
  const survivingNodes = graph.nodes.filter(
    (n) => !demotedAssetIDs.has(n.id) && !droppedAssetIDs.has(n.id),
  );
  const isSurviving = new Set(survivingNodes.map((n) => n.id));
  const survivingEdges = graph.edges.filter(
    (e) => isSurviving.has(e.from) && isSurviving.has(e.to),
  );

  // Mapa de dimensões por nó: pessoas/orgs/centro = NODE_*, compactos = ASSET_*.
  const sizeByID = new Map<string, { w: number; h: number }>();
  for (const n of survivingNodes) {
    const isCompact =
      (n.kind === "vehicle" || n.kind === "place") && n.id !== centerId;
    sizeByID.set(
      n.id,
      isCompact
        ? { w: ASSET_W, h: ASSET_H }
        : { w: NODE_W, h: NODE_H },
    );
  }

  const positions = layoutWithDagre(
    survivingNodes,
    survivingEdges,
    kindByID,
    centerId,
    sizeByID,
  );

  const rfNodes: Node[] = survivingNodes.map((n) => {
    const isCompact =
      (n.kind === "vehicle" || n.kind === "place") && n.id !== centerId;
    if (isCompact) {
      return {
        id: n.id,
        type: "asset",
        position: positions[n.id] ?? { x: 0, y: 0 },
        data: {
          kind: n.kind as AssetKind,
          label: shortAssetLabel(n),
          sublabel: compactSublabel(n),
          vehicleCategory: n.vehicle?.category,
          isCenter: false,
          ownerCount: ownerCountOf.get(n.id) ?? 0,
        } satisfies CompactNodeData,
        width: ASSET_W,
        height: ASSET_H,
      };
    }
    const displayName =
      n.kind === "vehicle" ? vehicleListLabel(n.vehicle, n.name) : n.name;
    return {
      id: n.id,
      type: "entity",
      position: positions[n.id] ?? { x: 0, y: 0 },
      data: {
        kind: n.kind,
        name: displayName,
        alias: n.alias,
        orcrim: n.orcrim_alias,
        photoUrl: n.has_photo ? photoURL(n.id, n.version) : undefined,
        vehicleCategory: n.vehicle?.category,
        isCenter: n.id === centerId,
        pills: pillsByOwner.get(n.id) ?? [],
        onPillClick,
      } satisfies EntityNodeData,
      width: NODE_W,
      height: NODE_H,
    };
  });

  const rfEdges: Edge[] = survivingEdges.map((e: GraphEdge) => {
    const fromKind = kindByID.get(e.from);
    const toKind = kindByID.get(e.to);
    const fromPos = positions[e.from];
    const toPos = positions[e.to];
    const isPersonToPerson = fromKind === "person" && toKind === "person";
    const isParental =
      e.relation_type === "father_of" || e.relation_type === "mother_of";
    const isSibling =
      e.relation_type === "sibling" || e.relation_type === "half_sibling";
    const isHalfSibling = e.relation_type === "half_sibling";
    // Camadas visuais (do mais forte ao mais fraco):
    //   1. parental         → accent, 1.8px, label opaco e bold
    //   2. sibling/spouse   → accent, 1.5px, label colorido
    //   3. half_sibling     → accent-dim com dash leve, 1.2px
    //   4. demais p2p       → accent-dim, 1.2px
    //   5. patrimonial      → line-2, 1px, opacidade baixa
    const stroke = isParental || isSibling
      ? "var(--accent)"
      : isPersonToPerson
        ? "var(--accent-dim)"
        : "var(--line-2)";
    const strokeWidth = isParental
      ? 1.8
      : isSibling
        ? 1.5
        : isPersonToPerson
          ? 1.2
          : 1;
    const opacity = isParental
      ? 1
      : isSibling
        ? isHalfSibling
          ? 0.75
          : 0.95
        : isPersonToPerson
          ? 0.85
          : 0.5;
    const labelFill = isParental
      ? "var(--accent)"
      : isSibling
        ? "var(--accent)"
        : isPersonToPerson
          ? "var(--fg-1)"
          : "var(--fg-3)";
    // half_sibling ganha um dash sutil pra diferenciar visualmente do sibling.
    const strokeDasharray = isHalfSibling ? "6 4" : undefined;
    // Relações entre nós no mesmo rank (sibling, half_sibling, spouse)
    // roteiam pelas laterais — sai do lado direito do nó à esquerda e
    // entra pelo lado esquerdo do nó à direita. Demais relações continuam
    // top/bottom (ids "tt"/"bs").
    const isSpouse = e.relation_type === "spouse";
    const useSideHandles = isSibling || isSpouse;
    let sourceHandle: string = "bs";
    let targetHandle: string = "tt";
    if (useSideHandles && fromPos && toPos) {
      if (fromPos.x <= toPos.x) {
        sourceHandle = "rs";
        targetHandle = "lt";
      } else {
        sourceHandle = "ls";
        targetHandle = "rt";
      }
    }
    return {
      id: e.id,
      source: e.from,
      target: e.to,
      sourceHandle,
      targetHandle,
      label: RELATION_LABEL[e.relation_type],
      labelStyle: {
        fontSize: isParental ? 10 : isSibling ? 9.5 : 9.5,
        letterSpacing: "0.16em",
        fill: labelFill,
        fontFamily: "inherit",
        fontWeight: isParental || isSibling ? 600 : 400,
      },
      labelBgStyle: { fill: "var(--bg-0)" },
      labelBgPadding: [4, 2],
      labelBgBorderRadius: 2,
      style: { stroke, strokeWidth, opacity, strokeDasharray },
      markerEnd: {
        type: MarkerType.ArrowClosed,
        color: stroke,
        width: 14,
        height: 14,
      },
    };
  });

  return { rfNodes, rfEdges, pillsByOwner };
}

// layoutWithDagre roda o algoritmo de camadas top-down do dagre. Atribui peso
// maior em edges pessoa↔pessoa pra que o layout privilegie a clareza dessas
// relações no centro do diagrama (caminhos mais retos, menos cruzamentos).
function layoutWithDagre(
  nodes: GraphNode[],
  edges: GraphEdge[],
  kindByID: Map<string, EntityKind>,
  centerId: string,
  sizeByID: Map<string, { w: number; h: number }>,
): Record<string, { x: number; y: number }> {
  const g = new dagre.graphlib.Graph();
  g.setGraph({ rankdir: "TB", nodesep: 120, ranksep: 110, marginx: 24, marginy: 24 });
  g.setDefaultEdgeLabel(() => ({}));

  for (const n of nodes) {
    const s = sizeByID.get(n.id) ?? { w: NODE_W, h: NODE_H };
    g.setNode(n.id, { width: s.w, height: s.h });
  }
  // Pesos do dagre influenciam ranks e roteamento:
  //   - parental    → minlen=1, weight 10 (define hierarquia vertical)
  //   - spouse      → minlen=1, weight 8  (camada seguinte, mas colado)
  //   - outros p2p  → minlen=1, weight 4
  //   - patrimonial → minlen=1, weight 1
  //   - sibling     → fora do dagre (post-processado abaixo pra ficar lateral)
  for (const e of edges) {
    const from = kindByID.get(e.from);
    const to = kindByID.get(e.to);
    const isP2P = from === "person" && to === "person";
    const isParental =
      e.relation_type === "father_of" || e.relation_type === "mother_of";
    const isSibling =
      e.relation_type === "sibling" || e.relation_type === "half_sibling";
    const isSpouse = e.relation_type === "spouse";
    if (isSibling) continue;
    if (isParental) {
      g.setEdge(e.from, e.to, { weight: 10, minlen: 1 });
    } else if (isSpouse) {
      g.setEdge(e.from, e.to, { weight: 8, minlen: 1 });
    } else if (isP2P) {
      g.setEdge(e.from, e.to, { weight: 4, minlen: 1 });
    } else {
      g.setEdge(e.from, e.to, { weight: 1, minlen: 1 });
    }
  }

  dagre.layout(g);

  // Helpers locais — todos os cálculos pós-dagre operam sobre o tamanho real
  // do nó (cartão grande vs. asset compacto).
  const sizeOf = (id: string) =>
    sizeByID.get(id) ?? { w: NODE_W, h: NODE_H };

  const out: Record<string, { x: number; y: number }> = {};
  for (const n of nodes) {
    const p = g.node(n.id);
    if (!p) continue;
    const s = sizeOf(n.id);
    out[n.id] = { x: p.x - s.w / 2, y: p.y - s.h / 2 };
  }

  // ─── Layout familiar: irmãos | alvo+esposa+filhos ───────────────────
  //
  // Da esquerda pra direita:
  //   1. IRMÃOS/PARENTES (coluna vertical à esquerda, mesma altura do âncora)
  //   2. ÂNCORA (centro do grafo)
  //   3. ESPOSA (mesmo rank do âncora, imediatamente à direita)
  //   4. FILHOS (linha abaixo, centralizados sob o par âncora+esposa)
  //   5. PROPRIEDADES (veículos, lugares) numa faixa abaixo dos filhos —
  //      evita sobreposição quando esposa/filhos têm carros próprios.
  //
  // Sub-clusters: a esposa pode ter sua própria subárvore (veículo, irmão
  // dela, etc.); cada irmão pode ter descendentes. Translamos cada sub-
  // cluster em bloco preservando o arranjo interno do dagre.

  const anchor = out[centerId];
  if (!anchor) return out;

  const adjacency = new Map<string, string[]>();
  for (const e of edges) {
    adjacency.set(e.from, [...(adjacency.get(e.from) ?? []), e.to]);
    adjacency.set(e.to, [...(adjacency.get(e.to) ?? []), e.from]);
  }

  // Mapa de vizinhos diretos do âncora por tipo de relação.
  let spouseID: string | null = null;
  const childIDs: string[] = [];
  const siblingIDs: string[] = [];
  for (const e of edges) {
    const isFrom = e.from === centerId;
    const isTo = e.to === centerId;
    if (!isFrom && !isTo) continue;
    const other = isFrom ? e.to : e.from;
    const t = e.relation_type;
    if (t === "spouse" && !spouseID) {
      spouseID = other;
    } else if ((t === "father_of" || t === "mother_of") && isFrom) {
      // âncora → filho (âncora é pai/mãe).
      if (!childIDs.includes(other)) childIDs.push(other);
    } else if (t === "sibling" || t === "half_sibling" || t === "relative") {
      if (!siblingIDs.includes(other)) siblingIDs.push(other);
    }
  }

  // Reclamantes principais — nós "âncora" dos sub-clusters. Cada sub-cluster
  // é a BFS a partir do reclamante, atravessando edges mas parando ao bater
  // em qualquer reclamante (incluindo o centro), evitando vazamento entre
  // subárvores.
  const claimants = new Set<string>([centerId]);
  if (spouseID) claimants.add(spouseID);
  for (const id of childIDs) claimants.add(id);
  for (const id of siblingIDs) claimants.add(id);

  function subCluster(rootID: string): string[] {
    const result: string[] = [rootID];
    const visited = new Set<string>([rootID]);
    const queue = [rootID];
    while (queue.length > 0) {
      const id = queue.shift()!;
      for (const next of adjacency.get(id) ?? []) {
        if (visited.has(next)) continue;
        if (claimants.has(next) && next !== rootID) continue; // não invade outro reclamante
        visited.add(next);
        result.push(next);
        queue.push(next);
      }
    }
    return result;
  }

  function translate(ids: string[], dx: number, dy: number) {
    for (const id of ids) {
      const p = out[id];
      if (!p) continue;
      p.x += dx;
      p.y += dy;
    }
  }

  const SPOUSE_GAP = 80;   // folga horizontal entre âncora e esposa
  const SIB_GAP_H = 140;   // folga horizontal entre coluna de irmãos e âncora
  const SIB_GAP_V = 30;    // folga vertical entre irmãos empilhados
  const CHILD_GAP = 50;    // folga horizontal entre filhos
  const CHILD_GAP_V = 90;  // distância vertical entre âncora e linha dos filhos
  const ASSET_GAP_V = 70;  // distância vertical entre linha dos filhos e faixa de propriedades
  const ASSET_GAP_H = 40;  // folga horizontal entre propriedades do mesmo dono

  const anchorSize = sizeOf(centerId);

  // 1. Esposa → mesma altura do âncora, à direita. Move o sub-cluster
  //    da esposa.
  if (spouseID && out[spouseID]) {
    const targetX = anchor.x + anchorSize.w + SPOUSE_GAP;
    const targetY = anchor.y;
    const s = out[spouseID];
    const dx = targetX - s.x;
    const dy = targetY - s.y;
    translate(subCluster(spouseID), dx, dy);
  }

  // 2. Filhos → linha abaixo, centralizados sob (âncora + esposa).
  const childRowY = anchor.y + anchorSize.h + CHILD_GAP_V;
  if (childIDs.length > 0) {
    const sortedChildren = [...childIDs]
      .filter((id) => out[id])
      .sort((a, b) => out[a].x - out[b].x);
    const spouseSize = spouseID ? sizeOf(spouseID) : null;
    const coupleMidX = spouseID && out[spouseID] && spouseSize
      ? (anchor.x + out[spouseID].x + spouseSize.w) / 2
      : anchor.x + anchorSize.w / 2;
    const childWidths = sortedChildren.map((id) => sizeOf(id).w);
    const total =
      childWidths.reduce((sum, w) => sum + w, 0) +
      Math.max(0, sortedChildren.length - 1) * CHILD_GAP;
    let cursor = coupleMidX - total / 2;
    for (const childID of sortedChildren) {
      const c = out[childID];
      if (!c) continue;
      const w = sizeOf(childID).w;
      const dx = cursor - c.x;
      const dy = childRowY - c.y;
      translate(subCluster(childID), dx, dy);
      cursor += w + CHILD_GAP;
    }
  }

  // 3. Irmãos → coluna vertical à ESQUERDA do âncora (lado oposto da
  //    esposa). Cada irmão com sua subárvore.
  if (siblingIDs.length > 0) {
    let stackY = anchor.y;
    siblingIDs.forEach((sibID) => {
      const s = out[sibID];
      if (!s) return;
      const sibSize = sizeOf(sibID);
      const targetX = anchor.x - SIB_GAP_H - sibSize.w;
      const targetY = stackY;
      const dx = targetX - s.x;
      const dy = targetY - s.y;
      translate(subCluster(sibID), dx, dy);
      stackY += sibSize.h + SIB_GAP_V;
    });
  }

  // 4. Propriedades (veículos, lugares) → faixa abaixo dos filhos. Cada
  //    propriedade é ancorada na coluna do seu dono. Evita sobreposição
  //    com filhos quando esposa/irmãos têm carros próprios.
  //
  // Aqui só entram assets *promovidos* (i.e. presentes em `nodes`); os
  // demovidos já viraram pill antes do dagre. Largura usada é a real do
  // asset (ASSET_W), não NODE_W.
  const family = new Set<string>(
    [centerId, spouseID, ...childIDs, ...siblingIDs].filter((id): id is string => !!id),
  );
  // Altura da faixa de assets baseada na maior altura possível na linha
  // dos filhos (cartão grande). Se não há filhos, usa altura do âncora.
  const childRowH = childIDs.length > 0 ? NODE_H : anchorSize.h;
  const assetRowY = childRowY + childRowH + ASSET_GAP_V;
  const assetsByOwner = new Map<string, string[]>();
  for (const n of nodes) {
    if (family.has(n.id)) continue;
    if (n.kind !== "vehicle" && n.kind !== "place") continue;
    if (!out[n.id]) continue;
    // Dono: vizinho na família (preferindo o âncora; senão o primeiro membro).
    let owner: string | null = null;
    for (const next of adjacency.get(n.id) ?? []) {
      if (family.has(next)) {
        if (next === centerId) {
          owner = next;
          break;
        }
        if (!owner) owner = next;
      }
    }
    if (!owner) continue;
    assetsByOwner.set(owner, [...(assetsByOwner.get(owner) ?? []), n.id]);
  }
  for (const [ownerID, assetIDs] of assetsByOwner) {
    const ownerPos = out[ownerID];
    if (!ownerPos) continue;
    const ownerSize = sizeOf(ownerID);
    const widths = assetIDs.map((id) => sizeOf(id).w);
    const total =
      widths.reduce((sum, w) => sum + w, 0) +
      Math.max(0, assetIDs.length - 1) * ASSET_GAP_H;
    let cursor = ownerPos.x + ownerSize.w / 2 - total / 2;
    for (const aID of assetIDs) {
      const a = out[aID];
      if (!a) continue;
      const w = sizeOf(aID).w;
      a.x = cursor;
      a.y = assetRowY;
      cursor += w + ASSET_GAP_H;
    }
  }

  return out;
}
