"use client";

import { useEffect, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { X, AlertTriangle, CheckCircle, Info, XCircle } from "lucide-react";
import { toast, ToastItem, ToastLevel } from "@/lib/toast";

const CONFIG: Record<
  ToastLevel,
  { bg: string; border: string; iconCls: string; Icon: React.ComponentType<{ size: number; className?: string }> }
> = {
  info:    { bg: "bg-blue-50",   border: "border-blue-200",   iconCls: "text-blue-500",   Icon: Info },
  success: { bg: "bg-green-50",  border: "border-green-200",  iconCls: "text-green-500",  Icon: CheckCircle },
  warning: { bg: "bg-amber-50",  border: "border-amber-200",  iconCls: "text-amber-500",  Icon: AlertTriangle },
  danger:  { bg: "bg-red-50",    border: "border-red-200",    iconCls: "text-red-500",    Icon: XCircle },
};

export function ToastContainer() {
  const [items, setItems] = useState<ToastItem[]>([]);

  useEffect(() => toast.subscribe(setItems), []);

  return (
    <div className="fixed top-4 right-4 z-[200] flex flex-col gap-2 pointer-events-none">
      <AnimatePresence>
        {items.map((item) => {
          const { bg, border, iconCls, Icon } = CONFIG[item.level];
          return (
            <motion.div
              key={item.id}
              initial={{ opacity: 0, x: 60, scale: 0.92 }}
              animate={{ opacity: 1, x: 0, scale: 1 }}
              exit={{ opacity: 0, x: 60, scale: 0.92 }}
              transition={{ type: "spring", stiffness: 320, damping: 28 }}
              className={`${bg} ${border} border rounded-xl shadow-lg shadow-black/5 p-3.5 min-w-[260px] max-w-[320px] pointer-events-auto`}
            >
              <div className="flex items-start gap-2.5">
                <Icon size={15} className={`${iconCls} mt-0.5 flex-shrink-0`} />
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-semibold text-gray-900 leading-snug">{item.title}</p>
                  {item.message && (
                    <p className="text-xs text-gray-500 mt-0.5 leading-relaxed">{item.message}</p>
                  )}
                </div>
                <button
                  onClick={() => toast.dismiss(item.id)}
                  className="text-gray-300 hover:text-gray-500 transition-colors flex-shrink-0 mt-0.5"
                >
                  <X size={13} />
                </button>
              </div>
            </motion.div>
          );
        })}
      </AnimatePresence>
    </div>
  );
}
