import { ReactNode } from "react";

interface Props {
  label: string;
  value: ReactNode;
  sub?: string;
  icon?: ReactNode;
  accent?: string; // tailwind text color class
}

export function StatCard({ label, value, sub, icon, accent = "text-gray-900" }: Props) {
  return (
    <div className="panel-surface-soft rounded-[20px] p-4 flex items-start gap-3">
      {icon && (
        <div className="w-9 h-9 rounded-xl bg-white/6 border border-white/10 flex items-center justify-center flex-shrink-0 mt-0.5 shadow-[inset_0_1px_0_rgba(255,255,255,0.05)]">
          {icon}
        </div>
      )}
      <div className="min-w-0">
        <p className="text-[11px] uppercase tracking-[0.18em] text-slate-400 mb-1">{label}</p>
        <p className={`text-xl font-bold leading-none metric-glow ${accent}`}>{value}</p>
        {sub && <p className="text-xs text-slate-400/90 mt-1.5 leading-relaxed">{sub}</p>}
      </div>
    </div>
  );
}
