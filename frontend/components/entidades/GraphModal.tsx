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
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Network, RotateCw, UserCircle2, X } from "lucide-react";
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

export default function GraphModal({ entityId, entityName, onClose }: Props) {
  const [centerId, setCenterId] = useState(entityId);
  const [centerName, setCenterName] = useState(entityName);
  const [depth, setDepth] = useState<GraphDepth>(2);
  const [graph, setGraph] = useState<EntityGraph | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
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

  const { rfNodes, rfEdges } = useMemo(
    () => buildFlow(graph, centerId),
    [graph, centerId],
  );

  const onNodeClick = useCallback(
    (_: unknown, node: Node) => setSelectedId(node.id),
    [],
  );

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
  onRecenter,
  onOpenDossier,
}: {
  node: GraphNode;
  isCenter: boolean;
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

type EntityNodeData = {
  kind: EntityKind;
  name: string;
  alias?: string;
  orcrim?: string;
  photoUrl?: string;
  isCenter: boolean;
};

function EntityNode({ data }: NodeProps) {
  const d = data as unknown as EntityNodeData;
  return (
    <div
      className={`graph-node graph-node--${d.kind}${d.isCenter ? " graph-node--center" : ""}`}
    >
      <Handle type="target" position={Position.Top} className="graph-node__handle" />
      <div className="graph-node__photo">
        {d.photoUrl ? (
          <img src={d.photoUrl} alt="" loading="lazy" draggable={false} />
        ) : (
          <div className="graph-node__photo-empty">
            <UserCircle2 size={28} strokeWidth={1.2} />
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
      </div>
      <Handle type="source" position={Position.Bottom} className="graph-node__handle" />
    </div>
  );
}

const nodeTypes = { entity: EntityNode };

// ───────────────────────── Layout (dagre) ─────────────────────────

function buildFlow(
  graph: EntityGraph | null,
  centerId: string,
): { rfNodes: Node[]; rfEdges: Edge[] } {
  if (!graph) return { rfNodes: [], rfEdges: [] };

  // Mapa id→kind pra estilizar as edges com base no par de pontas.
  const kindByID = new Map<string, EntityKind>();
  for (const n of graph.nodes) kindByID.set(n.id, n.kind);

  const positions = layoutWithDagre(graph.nodes, graph.edges, kindByID);

  const rfNodes: Node[] = graph.nodes.map((n) => {
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
        isCenter: n.id === centerId,
      } satisfies EntityNodeData,
      width: NODE_W,
      height: NODE_H,
    };
  });

  const rfEdges: Edge[] = graph.edges.map((e: GraphEdge) => {
    const fromKind = kindByID.get(e.from);
    const toKind = kindByID.get(e.to);
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
    return {
      id: e.id,
      source: e.from,
      target: e.to,
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

  return { rfNodes, rfEdges };
}

// layoutWithDagre roda o algoritmo de camadas top-down do dagre. Atribui peso
// maior em edges pessoa↔pessoa pra que o layout privilegie a clareza dessas
// relações no centro do diagrama (caminhos mais retos, menos cruzamentos).
function layoutWithDagre(
  nodes: GraphNode[],
  edges: GraphEdge[],
  kindByID: Map<string, EntityKind>,
): Record<string, { x: number; y: number }> {
  const g = new dagre.graphlib.Graph();
  g.setGraph({ rankdir: "TB", nodesep: 60, ranksep: 90, marginx: 24, marginy: 24 });
  g.setDefaultEdgeLabel(() => ({}));

  for (const n of nodes) g.setNode(n.id, { width: NODE_W, height: NODE_H });
  // Pesos do dagre influenciam ranks e roteamento: parental domina (peso alto +
  // minlen=1 garantido), pessoa-pessoa intermediário, demais leves.
  // Patrimonial usa minlen mais relaxado pra deixar a hierarquia familiar
  // mais "limpa" no topo do diagrama.
  for (const e of edges) {
    const from = kindByID.get(e.from);
    const to = kindByID.get(e.to);
    const isP2P = from === "person" && to === "person";
    const isParental =
      e.relation_type === "father_of" || e.relation_type === "mother_of";
    const isSibling =
      e.relation_type === "sibling" || e.relation_type === "half_sibling";
    // Irmandade é visual (peers no mesmo rank): pular o dagre evita que ela
    // estique ranks ou empurre os irmãos pra fora do alinhamento abaixo dos
    // pais. Pais cuidam de colocar irmãos na mesma camada naturalmente.
    if (isSibling) continue;
    if (isParental) {
      g.setEdge(e.from, e.to, { weight: 10, minlen: 1 });
    } else if (isP2P) {
      g.setEdge(e.from, e.to, { weight: 4, minlen: 1 });
    } else {
      g.setEdge(e.from, e.to, { weight: 1, minlen: 1 });
    }
  }

  dagre.layout(g);

  const out: Record<string, { x: number; y: number }> = {};
  for (const n of nodes) {
    const p = g.node(n.id);
    if (!p) continue;
    out[n.id] = { x: p.x - NODE_W / 2, y: p.y - NODE_H / 2 };
  }
  return out;
}
