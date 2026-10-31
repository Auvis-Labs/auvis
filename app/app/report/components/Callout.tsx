import React from "react";
import { Info, AlertTriangle, Lightbulb, ShieldAlert } from "lucide-react";

interface CalloutProps {
  type?: "info" | "warning" | "tip" | "danger";
  title?: string;
  children: React.ReactNode;
}

export function Callout({ type = "info", title, children }: CalloutProps) {
  const styles = {
    info: {
      border: "border-blue-500/30",
      bg: "bg-blue-500/5",
      text: "text-blue-900 dark:text-blue-200",
      icon: <Info className="h-5 w-5 text-blue-600 shrink-0 mt-0.5" />,
      defaultTitle: "Note",
    },
    warning: {
      border: "border-amber-500/30",
      bg: "bg-amber-500/5",
      text: "text-amber-900 dark:text-amber-200",
      icon: <AlertTriangle className="h-5 w-5 text-amber-600 shrink-0 mt-0.5" />,
      defaultTitle: "Warning",
    },
    tip: {
      border: "border-[#7B62FC]/30",
      bg: "bg-[#7B62FC]/5",
      text: "text-purple-950 dark:text-purple-200",
      icon: <Lightbulb className="h-5 w-5 text-[#7B62FC] shrink-0 mt-0.5" />,
      defaultTitle: "Pro Tip",
    },
    danger: {
      border: "border-rose-500/30",
      bg: "bg-rose-500/5",
      text: "text-rose-950 dark:text-rose-200",
      icon: <ShieldAlert className="h-5 w-5 text-rose-600 shrink-0 mt-0.5" />,
      defaultTitle: "Critical Alert",
    },
  }[type];

  return (
    <div className={`my-6 flex gap-3.5 rounded-xl border p-4.5 ${styles.border} ${styles.bg}`}>
      {styles.icon}
      <div className="flex-1 text-sm leading-relaxed text-slate-700">
        <strong className={`block mb-1 font-semibold ${styles.text}`}>
          {title || styles.defaultTitle}
        </strong>
        <div>{children}</div>
      </div>
    </div>
  );
}

