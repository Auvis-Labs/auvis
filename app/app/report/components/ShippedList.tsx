import React from "react";
import { Sparkles, Wrench, Gauge, BookOpen } from "lucide-react";

interface ShippedItemProps {
  type: "feat" | "fix" | "perf" | "docs";
  title: string;
  children: React.ReactNode;
}

export function ShippedItem({ type, title, children }: ShippedItemProps) {
  const badgeConfig = {
    feat: {
      label: "FEATURE",
      color: "bg-[#7B62FC]/10 text-[#7B62FC] border-[#7B62FC]/20",
      icon: <Sparkles className="h-3.5 w-3.5" />,
    },
    perf: {
      label: "PERF",
      color: "bg-cyan-500/10 text-cyan-700 border-cyan-500/20",
      icon: <Gauge className="h-3.5 w-3.5" />,
    },
    fix: {
      label: "FIX",
      color: "bg-amber-500/10 text-amber-700 border-amber-500/20",
      icon: <Wrench className="h-3.5 w-3.5" />,
    },
    docs: {
      label: "DOCS",
      color: "bg-emerald-500/10 text-emerald-700 border-emerald-500/20",
      icon: <BookOpen className="h-3.5 w-3.5" />,
    },
  }[type];

  return (
    <div className="flex flex-col gap-2 rounded-xl border border-slate-200/80 bg-white p-4.5 transition-all hover:border-slate-300">
      <div className="flex flex-wrap items-center gap-2.5">
        <span
          className={`inline-flex items-center gap-1 rounded-md border px-2 py-0.5 font-mono text-[11px] font-bold uppercase tracking-wider ${badgeConfig.color}`}
        >
          {badgeConfig.icon}
          {badgeConfig.label}
        </span>
        <h4 className="text-sm font-semibold text-slate-900 tracking-tight">{title}</h4>
      </div>
      <div className="text-xs sm:text-sm text-slate-600 leading-relaxed pl-0.5">
        {children}
      </div>
    </div>
  );
}

export function ShippedList({ children }: { children: React.ReactNode }) {
  return (
    <div className="my-6 flex flex-col gap-3">
      {children}
    </div>
  );
}

