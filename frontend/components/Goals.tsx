"use client";

import { useCallback, useEffect, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Target, Plus, Trash2, CheckCircle2, Loader2, Trophy, X } from "lucide-react";
import { api, Goal } from "@/lib/api";
import { toast } from "@/lib/toast";

const GOAL_TYPES = [
  { key: "monthly_yield",  label: "Monthly yield",   placeholder: "e.g. 500",  prefix: "$", suffix: "/mo" },
  { key: "total_earned",   label: "Total earned",    placeholder: "e.g. 5000", prefix: "$", suffix: "" },
  { key: "custom",         label: "Custom",          placeholder: "e.g. 1000", prefix: "$", suffix: "" },
];

function ProgressBar({ value, target }: { value: number; target: number }) {
  const pct = Math.min((value / target) * 100, 100);
  const color = pct >= 100 ? "bg-green-400" : pct >= 60 ? "bg-orange-400" : "bg-blue-400";
  return (
    <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden mt-2">
      <motion.div
        initial={{ width: 0 }}
        animate={{ width: `${pct}%` }}
        transition={{ duration: 0.8, ease: "easeOut" }}
        className={`h-full rounded-full ${color}`}
      />
    </div>
  );
}

export function Goals() {
  const [goals, setGoals]         = useState<Goal[]>([]);
  const [totalEarned, setTotal]   = useState(0);
  const [loading, setLoading]     = useState(true);
  const [creating, setCreating]   = useState(false);
  const [showForm, setShowForm]   = useState(false);

  // Form state
  const [formName, setFormName]   = useState("");
  const [formType, setFormType]   = useState("monthly_yield");
  const [formTarget, setFormTarget] = useState("");

  const load = useCallback(async () => {
    try {
      const data = await api.goals();
      setGoals(data.goals ?? []);
      setTotal(data.total_earned ?? 0);
    } catch {
      // backend not running — show empty state
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  async function createGoal() {
    if (!formName || !formTarget) return;
    setCreating(true);
    try {
      await api.createGoal(formName, formType, parseFloat(formTarget));
      setShowForm(false);
      setFormName(""); setFormType("monthly_yield"); setFormTarget("");
      toast.show("success", "Goal created!", formName);
      await load();
    } catch (e) {
      toast.show("danger", "Failed to create goal", String(e));
    } finally {
      setCreating(false);
    }
  }

  async function deleteGoal(id: number) {
    try {
      await api.deleteGoal(id);
      setGoals(prev => prev.filter(g => g.id !== id));
    } catch (e) {
      toast.show("danger", "Failed", String(e));
    }
  }

  const getProgress = (g: Goal) => {
    if (g.goal_type === "monthly_yield") return totalEarned;
    return g.progress;
  };

  const typeInfo = (key: string) => GOAL_TYPES.find(t => t.key === key) ?? GOAL_TYPES[0];

  if (loading) {
    return (
      <div className="bg-white rounded-xl border border-gray-200 p-4 flex items-center justify-center h-24">
        <Loader2 size={16} className="animate-spin text-gray-400" />
      </div>
    );
  }

  return (
    <div className="bg-white rounded-xl border border-gray-200 overflow-hidden">
      {/* Header */}
      <div className="px-4 pt-4 pb-3 border-b border-gray-100 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Target size={14} className="text-purple-500" />
          <span className="text-sm font-semibold text-gray-900">Financial Goals</span>
          <span className="text-xs text-gray-400">Track your yield objectives</span>
        </div>
        <button
          onClick={() => setShowForm(!showForm)}
          className="flex items-center gap-1 text-xs font-semibold text-orange-600 bg-orange-50 hover:bg-orange-100 border border-orange-200 px-2.5 py-1.5 rounded-lg transition-colors"
        >
          <Plus size={11} /> Add goal
        </button>
      </div>

      <div className="p-4 space-y-3">
        {/* Create form */}
        <AnimatePresence>
          {showForm && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: "auto" }}
              exit={{ opacity: 0, height: 0 }}
              className="overflow-hidden"
            >
              <div className="bg-gray-50 border border-gray-200 rounded-xl p-3 space-y-2.5 mb-1">
                <div className="flex items-center justify-between">
                  <p className="text-xs font-semibold text-gray-600">New goal</p>
                  <button onClick={() => setShowForm(false)} className="text-gray-400 hover:text-gray-600">
                    <X size={13} />
                  </button>
                </div>
                <input
                  value={formName}
                  onChange={e => setFormName(e.target.value)}
                  placeholder="Goal name (e.g. $500/month income)"
                  className="w-full text-sm bg-white border border-gray-200 rounded-lg px-3 py-2 focus:outline-none focus:ring-2 focus:ring-orange-200 focus:border-orange-300 transition-all"
                />
                <div className="flex gap-2">
                  <select
                    value={formType}
                    onChange={e => setFormType(e.target.value)}
                    className="flex-1 text-xs bg-white border border-gray-200 rounded-lg px-2 py-2 focus:outline-none text-gray-700"
                  >
                    {GOAL_TYPES.map(t => <option key={t.key} value={t.key}>{t.label}</option>)}
                  </select>
                  <div className="flex-1 relative">
                    <span className="absolute left-3 top-1/2 -translate-y-1/2 text-xs text-gray-400">$</span>
                    <input
                      type="number"
                      value={formTarget}
                      onChange={e => setFormTarget(e.target.value)}
                      placeholder={typeInfo(formType).placeholder}
                      className="w-full text-sm bg-white border border-gray-200 rounded-lg pl-6 pr-3 py-2 focus:outline-none focus:ring-2 focus:ring-orange-200 focus:border-orange-300 transition-all"
                    />
                  </div>
                </div>
                <button
                  onClick={createGoal}
                  disabled={creating || !formName || !formTarget}
                  className="w-full flex items-center justify-center gap-2 bg-orange-500 hover:bg-orange-600 disabled:opacity-40 text-white text-xs font-bold py-2 rounded-lg transition-all"
                >
                  {creating ? <Loader2 size={12} className="animate-spin" /> : <Plus size={12} />}
                  {creating ? "Creating…" : "Create goal"}
                </button>
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* Goal list */}
        {goals.length === 0 ? (
          <div className="text-center py-6">
            <Trophy size={28} className="text-gray-200 mx-auto mb-2" />
            <p className="text-sm text-gray-400">No goals yet</p>
            <p className="text-xs text-gray-300">Create your first financial goal to track progress</p>
          </div>
        ) : (
          <div className="space-y-2.5">
            {goals.map(g => {
              const progress = getProgress(g);
              const pct = Math.min((progress / g.target) * 100, 100);
              const ti = typeInfo(g.goal_type);
              const done = pct >= 100;
              return (
                <motion.div
                  key={g.id}
                  initial={{ opacity: 0, y: 8 }}
                  animate={{ opacity: 1, y: 0 }}
                  className={`rounded-xl border p-3 ${done ? "bg-green-50 border-green-200" : "bg-gray-50 border-gray-100"}`}
                >
                  <div className="flex items-start justify-between gap-2 mb-1">
                    <div className="flex items-center gap-2 flex-1 min-w-0">
                      {done
                        ? <CheckCircle2 size={14} className="text-green-500 flex-shrink-0" />
                        : <Target size={13} className="text-purple-400 flex-shrink-0 mt-0.5" />
                      }
                      <p className="text-sm font-medium text-gray-900 truncate">{g.name}</p>
                    </div>
                    <button onClick={() => deleteGoal(g.id)} className="text-gray-300 hover:text-red-400 flex-shrink-0 transition-colors">
                      <Trash2 size={12} />
                    </button>
                  </div>

                  <div className="flex items-center justify-between text-xs text-gray-500 mt-1">
                    <span>
                      <span className="font-mono font-semibold text-gray-800">{ti.prefix}{progress.toFixed(2)}</span>
                      {" / "}
                      {ti.prefix}{g.target.toLocaleString()}{ti.suffix}
                    </span>
                    <span className={`font-semibold ${done ? "text-green-600" : pct >= 60 ? "text-orange-500" : "text-blue-500"}`}>
                      {pct.toFixed(0)}%
                    </span>
                  </div>

                  <ProgressBar value={progress} target={g.target} />

                  {done && (
                    <p className="text-[10px] text-green-600 font-semibold mt-1.5">🎉 Goal achieved!</p>
                  )}
                </motion.div>
              );
            })}
          </div>
        )}

        {/* Total earned */}
        {totalEarned > 0 && (
          <div className="bg-green-50 border border-green-100 rounded-xl px-3 py-2 flex items-center justify-between">
            <span className="text-xs text-green-700">Total yield earned</span>
            <span className="font-mono font-bold text-green-800 text-sm">${totalEarned.toFixed(4)}</span>
          </div>
        )}
      </div>
    </div>
  );
}
