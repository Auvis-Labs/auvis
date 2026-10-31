import React from "react";
import Link from "next/link";
import { getAllDocs, getDocsSections } from "@/lib/docs";
import { ArrowLeft, ArrowRight } from "lucide-react";

interface DocsPagerProps {
  currentSlug: string;
}

export function DocsPager({ currentSlug }: DocsPagerProps) {
  const sections = getDocsSections();
  const flatDocs = sections.flatMap((s) => s.items);

  const currentIndex = flatDocs.findIndex((d) => d.slug === currentSlug);

  if (currentIndex === -1) {
    return null;
  }

  const prevDoc = currentIndex > 0 ? flatDocs[currentIndex - 1] : null;
  const nextDoc = currentIndex < flatDocs.length - 1 ? flatDocs[currentIndex + 1] : null;

  return (
    <div className="mt-14 pt-8 border-t border-slate-200 grid grid-cols-1 sm:grid-cols-2 gap-4">
      {prevDoc ? (
        <Link
          href={`/docs/${prevDoc.slug}`}
          className="group flex flex-col justify-between rounded-xl border border-slate-200 p-4 transition-all hover:border-[#7B62FC]/50 hover:bg-purple-50/10"
        >
          <span className="text-xs font-mono font-semibold text-slate-400 flex items-center gap-1 mb-1">
            <ArrowLeft className="h-3 w-3" /> Previous
          </span>
          <span className="text-sm font-semibold text-slate-900 group-hover:text-[#7B62FC] transition-colors line-clamp-1">
            {prevDoc.title}
          </span>
        </Link>
      ) : (
        <div className="hidden sm:block" />
      )}

      {nextDoc ? (
        <Link
          href={`/docs/${nextDoc.slug}`}
          className="group flex flex-col justify-between rounded-xl border border-slate-200 p-4 text-right transition-all hover:border-[#7B62FC]/50 hover:bg-purple-50/10"
        >
          <span className="text-xs font-mono font-semibold text-slate-400 flex items-center justify-end gap-1 mb-1">
            Next <ArrowRight className="h-3 w-3" />
          </span>
          <span className="text-sm font-semibold text-slate-900 group-hover:text-[#7B62FC] transition-colors line-clamp-1">
            {nextDoc.title}
          </span>
        </Link>
      ) : (
        <div className="hidden sm:block" />
      )}
    </div>
  );
}

