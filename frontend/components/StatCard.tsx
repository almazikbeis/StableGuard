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
    <div className="bg-white rounded-xl border border-gray-200 p-4 flex items-start gap-3">
      {icon && (
        <div className="w-8 h-8 rounded-lg bg-gray-50 flex items-center justify-center flex-shrink-0 mt-0.5">
          {icon}
        </div>
      )}
      <div className="min-w-0">
        <p className="text-xs text-gray-500 mb-0.5">{label}</p>
        <p className={`text-xl font-bold leading-none ${accent}`}>{value}</p>
        {sub && <p className="text-xs text-gray-400 mt-1">{sub}</p>}
      </div>
    </div>
  );
}
