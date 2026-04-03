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
    <div className={`panel-surface neon-border rounded-[22px] overflow-hidden ${className}`}>
      {(title || action) && (
        <div className="flex items-start justify-between px-5 py-4 border-b border-white/6">
          <div>
            {title && <h2 className="panel-title text-sm font-semibold">{title}</h2>}
            {subtitle && <p className="panel-subtitle text-xs mt-0.5">{subtitle}</p>}
          </div>
          {action && <div className="ml-4 flex-shrink-0">{action}</div>}
        </div>
      )}
      <div className="p-5">{children}</div>
    </div>
  );
}
