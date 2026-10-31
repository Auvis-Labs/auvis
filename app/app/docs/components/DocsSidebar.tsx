"use client";

import React, { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { DocSection } from "@/lib/docs";
import { Search, ChevronRight, BookOpen, Layers } from "lucide-react";

interface DocsSidebarProps {
  sections: DocSection[];
  onNavigate?: () => void;
}

export function DocsSidebar({ sections, onNavigate }: DocsSidebarProps) {
  const pathname = usePathname();
  const [filter, setFilter] = useState("");

  const filteredSections = sections
    .map((section) => ({
      ...section,
      items: section.items.filter((item) =>
        item.title.toLowerCase().includes(filter.toLowerCase())
      ),
    }))
    .filter((section) => section.items.length > 0);

  return (
    <aside className="w-full flex flex-col h-full">
      {/* Search Filter Input */}
      <div className="relative mb-6">
        <Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
        <input
          type="text"
          placeholder="Filter documentation..."
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="w-full rounded-lg border border-slate-200 bg-slate-50/70 py-2 pl-9 pr-3 text-xs text-slate-800 placeholder-slate-400 transition-colors focus:border-[#7B62FC] focus:bg-white focus:outline-hidden"
        />
        {filter && (
          <button
            onClick={() => setFilter("")}
            className="absolute right-2.5 top-2.5 text-xs text-slate-400 hover:text-slate-600"
          >
            ✕
          </button>
        )}
      </div>

      {/* Navigation Sections */}
      <nav className="flex-1 space-y-7 overflow-y-auto pr-2 pb-10 text-xs">
        {filteredSections.map((section) => (
          <div key={section.title}>
            <h4 className="mb-2.5 flex items-center gap-1.5 font-mono text-[11px] font-bold uppercase tracking-wider text-slate-500">
              <Layers className="h-3.5 w-3.5 text-[#7B62FC]" />
              <span>{section.title}</span>
            </h4>
            <ul className="space-y-1 border-l border-slate-100 ml-1.5 pl-2.5">
              {section.items.map((item) => {
                const href = `/docs/${item.slug}`;
                const isActive = pathname === href || (pathname === "/docs" && item.slug === "overview");

                return (
                  <li key={item.slug}>
                    <Link
                      href={href}
                      onClick={onNavigate}
                      className={`group flex items-center justify-between rounded-md px-2 py-1.5 transition-all ${
                        isActive
                          ? "bg-[#7B62FC]/10 font-semibold text-[#7B62FC]"
                          : "text-slate-600 hover:bg-slate-50 hover:text-slate-900"
                      }`}
                    >
                      <span className="truncate">{item.title}</span>
                      {isActive && <span className="h-1.5 w-1.5 rounded-full bg-[#7B62FC]" />}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>
    </aside>
  );
}

