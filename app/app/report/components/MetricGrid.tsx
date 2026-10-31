import React from "react";

interface MetricCardProps {
  label: string;
  value: string;
  sub?: string;
  change?: string;
}

export function MetricCard({ label, value, sub, change }: MetricCardProps) {
  return (
    <div className="flex flex-col justify-between rounded-xl border border-slate-200/80 bg-gradient-to-b from-white to-slate-50/50 p-5 shadow-xs transition-all hover:border-[#7B62FC]/40">
      <div className="flex items-center justify-between text-xs font-medium uppercase tracking-wider text-slate-500">
        <span>{label}</span>
        {change && (
          <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold text-emerald-600">
            {change}
          </span>
        )}
      </div>
      <div className="my-2.5 font-mono text-2xl font-bold tracking-tight text-slate-900 sm:text-3xl">
        {value}
      </div>
      {sub && <div className="text-xs text-slate-500 leading-normal">{sub}</div>}
    </div>
  );
}

export function MetricGrid({ children }: { children: React.ReactNode }) {
  return (
    <div className="my-8 grid grid-cols-1 gap-4 sm:grid-cols-3">
      {children}
    </div>
  );
}

