"use client";

import { motion } from "framer-motion";

interface RiskGaugeProps {
  value: number; // 0–100
  size?: number;
}

function riskColor(v: number) {
  if (v < 30) return { stroke: "#4fe3ff", text: "#b3f4ff", label: "Low" };
  if (v < 60) return { stroke: "#ffd166", text: "#ffe4a3", label: "Medium" };
  if (v < 80) return { stroke: "#ff9d4d", text: "#ffc48f", label: "High" };
  return { stroke: "#ff6b9d", text: "#ffb2c9", label: "Critical" };
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
  const endPoint = polarToXY(fillEnd, r);
  const circumference = 2 * Math.PI * r;

  return (
    <div className="terminal-ring relative flex flex-col items-center rounded-full px-4 pt-4">
      <div
        className="pointer-events-none absolute inset-3 rounded-full blur-3xl"
        style={{ background: `radial-gradient(circle, ${stroke}22 0%, transparent 68%)` }}
      />
      <svg width={size} height={size * 0.8} viewBox={`0 0 ${size} ${size}`}>
        <defs>
          <linearGradient id="risk-track" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="rgba(79,227,255,0.24)" />
            <stop offset="50%" stopColor="rgba(255,122,26,0.2)" />
            <stop offset="100%" stopColor="rgba(255,107,157,0.24)" />
          </linearGradient>
          <filter id="risk-glow">
            <feGaussianBlur stdDeviation="5" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        {/* Track */}
        <path
          d={arc(trackStart, trackEnd, r)}
          fill="none"
          stroke="url(#risk-track)"
          strokeWidth={12}
          strokeLinecap="round"
          opacity={0.35}
        />
        {/* Fill */}
        {clamp > 0 && (
          <motion.path
            d={arc(trackStart, fillEnd, r)}
            fill="none"
            stroke={stroke}
            strokeWidth={12}
            strokeLinecap="round"
            filter="url(#risk-glow)"
            initial={{ pathLength: 0 }}
            animate={{ pathLength: 1 }}
            transition={{ duration: 0.8, ease: "easeOut" }}
          />
        )}

        <circle cx={endPoint.x} cy={endPoint.y} r={6} fill={stroke} filter="url(#risk-glow)" />
        <circle cx={endPoint.x} cy={endPoint.y} r={11} fill="none" stroke={stroke} strokeOpacity={0.22} strokeWidth={1.5}>
          <animate attributeName="r" values="8;14;8" dur="1.8s" repeatCount="indefinite" />
          <animate attributeName="opacity" values="0.65;0.1;0.65" dur="1.8s" repeatCount="indefinite" />
        </circle>

        <circle
          cx={cx}
          cy={cy}
          r={r - 18}
          fill="none"
          stroke="rgba(255,255,255,0.06)"
          strokeDasharray={`${circumference / 42} ${circumference / 72}`}
        />
        {/* Value */}
        <text x={cx} y={cy + 6} textAnchor="middle" fill={text} fontSize={34} fontWeight={700} className="font-mono-data">
          {Math.round(clamp)}
        </text>
        <text x={cx} y={cy - 12} textAnchor="middle" fill="#6f86ab" fontSize={10} letterSpacing="2">
          LIVE RISK
        </text>
        <text x={cx} y={cy + 24} textAnchor="middle" fill="#93a9c8" fontSize={11}>
          /100 SCORE
        </text>
        {/* Min / Max labels */}
        <text x={cx - r + 4} y={cy + r - 4} textAnchor="middle" fill="#7b90b2" fontSize={9}>0</text>
        <text x={cx + r - 4} y={cy + r - 4} textAnchor="middle" fill="#7b90b2" fontSize={9}>100</text>
      </svg>
      <span
        className="data-chip mt-1 rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.2em]"
        style={{ color: text }}
      >
        {label} Risk
      </span>
    </div>
  );
}
