import React from "react";
import Link from "next/link";
import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { getAllDocs, getDocBySlug } from "@/lib/docs";
import { compileMDX } from "next-mdx-remote/rsc";
import { docsMdxComponents } from "../components/docs-mdx-components";
import { TableOfContents } from "../components/TableOfContents";
import { DocsPager } from "../components/DocsPager";
import { ChevronRight } from "lucide-react";
import remarkGfm from "remark-gfm";

interface PageProps {
  params: Promise<{ slug: string[] }>;
}

export async function generateStaticParams() {
  const docs = getAllDocs();
  return docs.map((d) => ({
    slug: d.slug.split("/"),
  }));
}

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const { slug } = await params;
  const slugString = slug.join("/");
  const doc = getDocBySlug(slugString);

  if (!doc) {
    return {
      title: "Document Not Found - Auvis",
    };
  }

  return {
    title: `${doc.meta.title} — Auvis Documentation`,
    description: doc.meta.description,
  };
}

export default async function DocPage({ params }: PageProps) {
  const { slug } = await params;
  const slugString = slug.join("/");
  const docData = getDocBySlug(slugString);

  if (!docData) {
    notFound();
  }

  const { meta, rawContent, toc } = docData;

  const plugin = (remarkGfm as unknown as { default?: unknown }).default || remarkGfm;
  const { content } = await compileMDX({
    source: rawContent,
    components: docsMdxComponents,
    options: {
      mdxOptions: {
        remarkPlugins: [plugin as any],
      },
    },
  });

  return (
    <div className="flex gap-10">
      {/* Center Content Column */}
      <article className="flex-1 min-w-0 max-w-3xl">
        {/* Breadcrumb */}
        <div className="flex items-center gap-1.5 text-xs text-slate-500 mb-6">
          <Link href="/docs" className="transition-colors hover:text-[#7B62FC]">
            Docs
          </Link>
          <ChevronRight className="h-3 w-3 text-slate-400" />
          <span className="text-slate-600 font-medium">{meta.section}</span>
          <ChevronRight className="h-3 w-3 text-slate-400" />
          <span className="text-slate-900 font-semibold truncate">{meta.title}</span>
        </div>

        {/* Page Header */}
        <header className="mb-8 pb-6 border-b border-slate-200/80">
          <h1
            className="text-3xl sm:text-4xl font-bold tracking-tight text-slate-900"
            style={{ fontFamily: "var(--font-bricolage), sans-serif" }}
          >
            {meta.title}
          </h1>
          {meta.description && (
            <p className="mt-3 text-base text-slate-600 leading-relaxed">
              {meta.description}
            </p>
          )}
        </header>

        {/* MDX Body */}
        <div className="prose prose-slate max-w-none">
          {content}
        </div>

        {/* Bottom Pager */}
        <DocsPager currentSlug={slugString} />
      </article>

      {/* Right Sticky Table of Contents (Desktop) */}
      <div className="hidden xl:block w-64 shrink-0 pl-4">
        <div className="sticky top-24 max-h-[calc(100vh-8rem)] overflow-y-auto">
          <TableOfContents toc={toc} slug={slugString} />
        </div>
      </div>
    </div>
  );
}

