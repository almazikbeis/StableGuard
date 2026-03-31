"use client";

interface RiskGaugeProps {
  value: number; // 0–100
  size?: number;
}

function riskColor(v: number) {
  if (v < 30) return { stroke: "#16a34a", text: "#15803d", label: "Low" };
  if (v < 60) return { stroke: "#d97706", text: "#b45309", label: "Medium" };
  if (v < 80) return { stroke: "#f97316", text: "#ea580c", label: "High" };
  return { stroke: "#dc2626", text: "#b91c1c", label: "Critical" };
}

export function RiskGauge({ value, size = 180 }: RiskGaugeProps) {
  const clamp = Math.max(0, Math.min(100, value));
  const { stroke, text, label } = riskColor(clamp);

  const r = 72;
  const cx = size / 2;
  const cy = size / 2;
  const startAngle = -210; // degrees
  const totalArc = 240;
  const arcAngle = (clamp / 100) * totalArc;

  function polarToXY(angleDeg: number, radius: number) {
    const rad = ((angleDeg - 90) * Math.PI) / 180;
    return { x: cx + radius * Math.cos(rad), y: cy + radius * Math.sin(rad) };
  }

  function arc(startDeg: number, endDeg: number, r: number) {
    const s = polarToXY(startDeg, r);
    const e = polarToXY(endDeg, r);
    const large = endDeg - startDeg > 180 ? 1 : 0;
    return `M ${s.x} ${s.y} A ${r} ${r} 0 ${large} 1 ${e.x} ${e.y}`;
  }

  const trackStart = startAngle;
  const trackEnd = startAngle + totalArc;
  const fillEnd = startAngle + arcAngle;

  return (
    <div className="flex flex-col items-center">
      <svg width={size} height={size * 0.8} viewBox={`0 0 ${size} ${size}`}>
        {/* Track */}
        <path
          d={arc(trackStart, trackEnd, r)}
          fill="none"
          stroke="#f3f4f6"
          strokeWidth={10}
          strokeLinecap="round"
        />
        {/* Fill */}
        {clamp > 0 && (
          <path
            d={arc(trackStart, fillEnd, r)}
            fill="none"
            stroke={stroke}
            strokeWidth={10}
            strokeLinecap="round"
            style={{ transition: "all 0.6s ease" }}
          />
        )}
        {/* Value */}
        <text x={cx} y={cy + 8} textAnchor="middle" fill={text} fontSize={32} fontWeight={700}>
          {Math.round(clamp)}
        </text>
        <text x={cx} y={cy + 26} textAnchor="middle" fill="#9ca3af" fontSize={11}>
          / 100
        </text>
        {/* Min / Max labels */}
        <text x={cx - r + 4} y={cy + r - 4} textAnchor="middle" fill="#d1d5db" fontSize={9}>0</text>
        <text x={cx + r - 4} y={cy + r - 4} textAnchor="middle" fill="#d1d5db" fontSize={9}>100</text>
      </svg>
      <span
        className="text-xs font-semibold px-2.5 py-0.5 rounded-full mt-1"
        style={{ background: stroke + "18", color: text }}
      >
        {label} Risk
      </span>
    </div>
  );
}
