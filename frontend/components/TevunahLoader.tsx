// Loader animado do Tevunah — combina o glifo de rede (camada estática + fluxo
// + breath no núcleo) com a digitação das letras hebraicas תבונה e um cursor
// piscante que percorre as posições conforme cada letra entra. Herda a cor do
// container via currentColor. Keyframes ficam em frontend/app/globals.css
// (.tv-edge, .tv-core, .tv-letter-1..5, .tv-cursor).
export default function TevunahLoader({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 400 200"
      fill="none"
      role="img"
      aria-label="Tevunah"
    >
      <title>Tevunah</title>

      {/* ───── Ícone de rede ───── */}
      <g transform="translate(172,24) scale(0.875)">
        {/* base estática */}
        <g stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" fill="none" opacity="0.18">
          <line x1="14" y1="20" x2="32" y2="32" />
          <line x1="50" y1="20" x2="32" y2="32" />
          <line x1="14" y1="44" x2="32" y2="32" />
          <line x1="50" y1="44" x2="32" y2="32" />
          <line x1="32" y1="12" x2="32" y2="32" />
          <line x1="14" y1="20" x2="32" y2="12" />
          <line x1="50" y1="20" x2="32" y2="12" />
          <line x1="14" y1="20" x2="14" y2="44" />
          <line x1="50" y1="20" x2="50" y2="44" />
        </g>
        {/* camada de fluxo */}
        <g stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" fill="none">
          <line className="tv-edge" x1="14" y1="20" x2="32" y2="32" />
          <line className="tv-edge" x1="50" y1="20" x2="32" y2="32" />
          <line className="tv-edge" x1="14" y1="44" x2="32" y2="32" />
          <line className="tv-edge" x1="50" y1="44" x2="32" y2="32" />
          <line className="tv-edge" x1="32" y1="12" x2="32" y2="32" />
          <line className="tv-edge" x1="14" y1="20" x2="32" y2="12" />
          <line className="tv-edge" x1="50" y1="20" x2="32" y2="12" />
          <line className="tv-edge" x1="14" y1="20" x2="14" y2="44" />
          <line className="tv-edge" x1="50" y1="20" x2="50" y2="44" />
        </g>
        {/* nós */}
        <g fill="currentColor">
          <circle cx="14" cy="20" r="2.6" />
          <circle cx="50" cy="20" r="2.6" />
          <circle cx="14" cy="44" r="2.6" />
          <circle cx="50" cy="44" r="2.6" />
          <circle cx="32" cy="12" r="2.6" />
          <circle className="tv-core" cx="32" cy="32" r="4.5" />
        </g>
      </g>

      {/* ───── Letras hebraicas (תבונה — direita p/ esquerda) ───── */}
      <g>
        <path
          className="tv-letter tv-letter-5"
          d="M535 722V0H205V586ZM1221 569V0H891V603Q891 766 849.5 824.0Q808 882 688 882H186V1120H686Q837 1120 938.0 1090.0Q1039 1060 1102.5 992.5Q1166 925 1193.5 823.0Q1221 721 1221 569Z"
          transform="translate(122.26,150) scale(0.0273,-0.0273)"
          fill="currentColor"
        />
        <path
          className="tv-letter tv-letter-4"
          d="M411 238V684Q411 817 370.0 849.5Q329 882 264 882H111V1120H264Q552 1120 647.0 1006.0Q742 892 742 706V0H88V238Z"
          transform="translate(160.73,150) scale(0.0273,-0.0273)"
          fill="currentColor"
        />
        <path
          className="tv-letter tv-letter-3"
          d="M517 1120V0H186V1120Z"
          transform="translate(186.11,150) scale(0.0273,-0.0273)"
          fill="currentColor"
        />
        <path
          className="tv-letter tv-letter-2"
          d="M680 238V602Q680 659 676.5 696.5Q673 734 660.5 772.0Q648 810 625.5 832.0Q603 854 563.5 868.0Q524 882 467 882H88V1120H467Q597 1120 691.5 1097.0Q786 1074 847.5 1031.0Q909 988 945.5 918.5Q982 849 996.5 766.0Q1011 683 1011 569V238H1161V0H88V238Z"
          transform="translate(205.33,150) scale(0.0273,-0.0273)"
          fill="currentColor"
        />
        <path
          className="tv-letter tv-letter-1"
          d="M1213 602V0H882V603Q882 779 832 832Q786 882 691 882H550V420Q550 167 461 74Q382 -9 232.0 -9.0Q82 -9 20 12V237Q118 224 130 224Q180 224 199.5 263.5Q219 303 219 420V882H24V1120H691Q1043 1120 1148 926Q1213 806 1213 602Z"
          transform="translate(239.48,150) scale(0.0273,-0.0273)"
          fill="currentColor"
        />
      </g>

      {/* Cursor piscante que percorre as posições */}
      <rect className="tv-cursor" x="0" y="112" width="3" height="42" fill="currentColor" />
    </svg>
  );
}
