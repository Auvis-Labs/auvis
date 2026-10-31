"use client";

import React, { useState } from "react";
import { Check, Copy } from "lucide-react";
import Prism from "prismjs";

// Load syntax grammars for builder docs & devlogs
import "prismjs/components/prism-yaml";
import "prismjs/components/prism-go";
import "prismjs/components/prism-bash";
import "prismjs/components/prism-json";
import "prismjs/components/prism-promql";
import "prismjs/components/prism-typescript";
import "prismjs/components/prism-python";
import "prismjs/components/prism-sql";
import "prismjs/components/prism-diff";
import "prismjs/components/prism-docker";

interface PreProps extends React.HTMLAttributes<HTMLPreElement> {
  children?: React.ReactNode;
}

// Language normalization map
const LANG_MAP: Record<string, string> = {
  yml: "yaml",
  yaml: "yaml",
  go: "go",
  golang: "go",
  bash: "bash",
  sh: "bash",
  shell: "bash",
  zsh: "bash",
  json: "json",
  promql: "promql",
  prometheus: "promql",
  ts: "typescript",
  typescript: "typescript",
  js: "javascript",
  javascript: "javascript",
  py: "python",
  python: "python",
  sql: "sql",
  diff: "diff",
  docker: "docker",
  dockerfile: "docker",
};

// Language display labels and dot accent colors
const LANG_METADATA: Record<string, { label: string; dotClass: string }> = {
  yaml: { label: "YAML", dotClass: "bg-purple-400" },
  go: { label: "GO", dotClass: "bg-cyan-400" },
  bash: { label: "BASH", dotClass: "bg-emerald-400" },
  json: { label: "JSON", dotClass: "bg-amber-400" },
  promql: { label: "PROMQL", dotClass: "bg-rose-400" },
  typescript: { label: "TS", dotClass: "bg-blue-400" },
  javascript: { label: "JS", dotClass: "bg-yellow-400" },
  python: { label: "PYTHON", dotClass: "bg-blue-400" },
  sql: { label: "SQL", dotClass: "bg-indigo-400" },
  diff: { label: "DIFF", dotClass: "bg-teal-400" },
  docker: { label: "DOCKER", dotClass: "bg-sky-400" },
  text: { label: "TEXT", dotClass: "bg-slate-500" },
};

function extractText(node: React.ReactNode): string {
  if (!node) return "";
  if (typeof node === "string") return node;
  if (typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(extractText).join("");
  if (React.isValidElement(node) && node.props && (node.props as { children?: React.ReactNode }).children) {
    return extractText((node.props as { children?: React.ReactNode }).children);
  }
  return "";
}

function autoDetectLang(code: string): string {
  const trimmed = code.trim();
  if (trimmed.startsWith("{") && trimmed.endsWith("}")) return "json";
  if (trimmed.startsWith("[") && trimmed.endsWith("]")) return "json";
  if (/^(auvis|curl|docker|npm|go run|make|git|\$ )/m.test(trimmed)) return "bash";
  if (/^(version:|gateway:|policies:|match:|rules:|sources:|destinations:)/m.test(trimmed)) return "yaml";
  if (/^(package main|func |import \()/m.test(trimmed)) return "go";
  if (/^(rate\(|histogram_quantile|sum\(|count\()/m.test(trimmed)) return "promql";
  return "text";
}

export function PreBlock({ children, ...props }: PreProps) {
  const [copied, setCopied] = useState(false);

  // Extract raw text and language from children (e.g. <code className="language-yaml">)
  let rawCode = "";
  let rawLang = "";

  React.Children.forEach(children, (child) => {
    if (React.isValidElement(child)) {
      const className = (child.props as { className?: string })?.className || "";
      const match = /language-([a-z0-9_-]+)/i.exec(className);
      if (match) {
        rawLang = match[1].toLowerCase();
      }
      const innerText = extractText((child.props as { children?: React.ReactNode })?.children);
      if (innerText) {
        rawCode = innerText;
      }
    }
  });

  if (!rawCode) {
    rawCode = extractText(children);
  }

  // Normalize language or autodetect
  const mappedLang = rawLang ? (LANG_MAP[rawLang] || rawLang) : autoDetectLang(rawCode);
  const meta = LANG_METADATA[mappedLang] || { label: (mappedLang || "SNIPPET").toUpperCase(), dotClass: "bg-slate-400" };

  // Check if the first line is a filename comment (e.g. # auvis.yaml, // main.go)
  let fileName: string | null = null;
  const firstLine = rawCode.trim().split("\n")[0] || "";
  const fileMatch = /^(?:#|\/\/)\s*([a-zA-Z0-9_\-\.\/]+\.[a-zA-Z0-9]+)\s*$/.exec(firstLine);
  if (fileMatch) {
    fileName = fileMatch[1];
  }

  // Generate highlighted HTML via Prism
  let highlightedHtml: string | null = null;
  const grammar = Prism.languages[mappedLang];
  if (grammar && rawCode) {
    try {
      highlightedHtml = Prism.highlight(rawCode.replace(/\n$/, ""), grammar, mappedLang);
    } catch {
      highlightedHtml = null;
    }
  }

  const handleCopy = () => {
    const textToCopy = rawCode.replace(/\n$/, "");
    if (textToCopy) {
      navigator.clipboard.writeText(textToCopy);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div className="group relative my-6 overflow-hidden rounded-xl border border-slate-800/80 bg-[#0B0C11] text-slate-100 shadow-xl shadow-black/20">
      {/* Chrome header with terminal dots, language badge, and copy button */}
      <div className="flex items-center justify-between border-b border-slate-800/80 bg-[#12131C]/90 px-4 py-2.5 backdrop-blur-sm">
        <div className="flex items-center gap-3">
          {/* Terminal window buttons */}
          <div className="flex items-center gap-1.5" aria-hidden="true">
            <span className="h-2.5 w-2.5 rounded-full bg-red-500/30 border border-red-500/50" />
            <span className="h-2.5 w-2.5 rounded-full bg-yellow-500/30 border border-yellow-500/50" />
            <span className="h-2.5 w-2.5 rounded-full bg-emerald-500/30 border border-emerald-500/50" />
          </div>

          <span className="h-3.5 w-[1px] bg-slate-800" />

          {/* Language / File pill */}
          <div className="flex items-center gap-1.5">
            <span className={`h-1.5 w-1.5 rounded-full ${meta.dotClass}`} />
            <span className="text-[11px] font-mono font-medium tracking-wider text-slate-300">
              {fileName || meta.label}
            </span>
            {fileName && (
              <span className="text-[10px] font-mono text-slate-500 uppercase">
                · {meta.label}
              </span>
            )}
          </div>
        </div>

        {/* Copy button */}
        <button
          onClick={handleCopy}
          className="flex items-center gap-1.5 rounded-md px-2.5 py-1 text-slate-400 transition-all hover:bg-slate-800/90 hover:text-white active:scale-95 border border-transparent hover:border-slate-700/60"
          title="Copy snippet to clipboard"
          aria-label="Copy snippet"
        >
          {copied ? (
            <>
              <Check className="h-3.5 w-3.5 text-emerald-400 animate-in zoom-in-50 duration-200" />
              <span className="text-[11px] font-medium text-emerald-400">Copied!</span>
            </>
          ) : (
            <>
              <Copy className="h-3.5 w-3.5 transition-transform group-hover:scale-105" />
              <span className="text-[11px] font-medium">Copy</span>
            </>
          )}
        </button>
      </div>

      {/* Code contents */}
      <pre
        {...props}
        className="overflow-x-auto p-4 sm:p-5 text-xs sm:text-[13px] font-mono leading-relaxed text-slate-200 selection:bg-[#7B62FC]/30 selection:text-white [&>code]:bg-transparent [&>code]:border-0 [&>code]:p-0 [&>code]:text-inherit [&>code]:rounded-none"
      >
        {highlightedHtml ? (
          <code
            className={`language-${mappedLang} font-mono`}
            dangerouslySetInnerHTML={{ __html: highlightedHtml }}
          />
        ) : (
          <code className={`language-${mappedLang} font-mono`}>
            {rawCode.replace(/\n$/, "")}
          </code>
        )}
      </pre>
    </div>
  );
}

export function InlineCode({ children, className, ...props }: React.HTMLAttributes<HTMLElement>) {
  // If this code tag is inside a code block (e.g. language-yaml, language-bash, or within pre),
  // do not add the inline pill background.
  const isCodeBlock = className && (className.includes("language-") || className.includes("token"));
  if (isCodeBlock) {
    return (
      <code {...props} className={`bg-transparent border-0 p-0 text-inherit font-inherit ${className || ""}`}>
        {children}
      </code>
    );
  }

  return (
    <code
      {...props}
      className={`rounded-md bg-purple-50/80 px-1.5 py-0.5 font-mono text-[0.875em] font-medium text-[#7B62FC] border border-purple-200/60 ${className || ""}`}
    >
      {children}
    </code>
  );
}
