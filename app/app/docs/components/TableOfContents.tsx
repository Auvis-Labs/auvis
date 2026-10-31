"use client";

import React, { useEffect, useState } from "react";
import { TOCItem } from "@/lib/docs";
import { AlignLeft } from "lucide-react";

interface TableOfContentsProps {
  toc: TOCItem[];
  slug: string;
}

export function TableOfContents({ toc, slug }: TableOfContentsProps) {
  const [activeId, setActiveId] = useState<string>("");

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveId(entry.target.id);
          }
        }
      },
      { rootMargin: "0% 0% -70% 0%" }
    );

    toc.forEach((item) => {
      const el = document.getElementById(item.id);
      if (el) observer.observe(el);
    });

    return () => observer.disconnect();
  }, [toc]);

  if (!toc || toc.length === 0) {
    return null;
  }

  return (
    <div className="w-full text-xs">
      <div className="mb-3 flex items-center gap-1.5 font-mono text-[11px] font-bold uppercase tracking-wider text-slate-500">
        <AlignLeft className="h-3.5 w-3.5 text-[#7B62FC]" />
        <span>On This Page</span>
      </div>

      <ul className="space-y-2 border-l border-slate-100 pl-3">
        {toc.map((item) => {
          const isActive = activeId === item.id;
          return (
            <li
              key={item.id}
              className={`${item.level === 3 ? "pl-3 text-slate-500" : ""}`}
            >
              <a
                href={`#${item.id}`}
                className={`block transition-colors hover:text-[#7B62FC] ${
                  isActive ? "font-semibold text-[#7B62FC]" : "text-slate-600"
                }`}
              >
                {item.text}
              </a>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

