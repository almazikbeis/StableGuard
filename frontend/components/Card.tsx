import { ReactNode } from "react";

interface CardProps {
  title?: string;
  subtitle?: string;
  children: ReactNode;
  className?: string;
  action?: ReactNode;
}

export function Card({ title, subtitle, children, className = "", action }: CardProps) {
  return (
    <div className={`panel-surface neon-border hover-lift rounded-[22px] overflow-hidden ${className}`}>
      {(title || action) && (
        <div className="relative flex items-start justify-between px-5 py-4 border-b border-white/6">
          <div className="pointer-events-none absolute inset-x-5 top-0 h-px bg-gradient-to-r from-transparent via-white/25 to-transparent" />
          <div>
            {title && <h2 className="panel-title text-sm font-semibold uppercase tracking-[0.16em]">{title}</h2>}
            {subtitle && <p className="panel-subtitle text-xs mt-0.5">{subtitle}</p>}
          </div>
          {action && <div className="ml-4 flex-shrink-0">{action}</div>}
        </div>
      )}
      <div className="p-5">{children}</div>
    </div>
  );
}
