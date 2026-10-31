import React from "react";
import { Callout } from "../../report/components/Callout";
import { MetricGrid, MetricCard } from "../../report/components/MetricGrid";
import { ShippedList, ShippedItem } from "../../report/components/ShippedList";
import { PreBlock, InlineCode } from "../../report/components/CodeBlock";
import { Mermaid } from "./Mermaid";

function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^\w\s-]/g, "")
    .replace(/\s+/g, "-");
}

function getNodeText(node: React.ReactNode): string {
  if (!node) return "";
  if (typeof node === "string") return node;
  if (typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(getNodeText).join("");
  if (React.isValidElement(node) && node.props && (node.props as { children?: React.ReactNode }).children) {
    return getNodeText((node.props as { children?: React.ReactNode }).children);
  }
  return "";
}

export const docsMdxComponents = {
  // Custom interactive components
  Callout,
  MetricGrid,
  MetricCard,
  ShippedList,
  ShippedItem,
  Mermaid,

  // Styled standard HTML elements with automated ID generation for TOC
  h1: ({ children, ...props }: React.HTMLAttributes<HTMLHeadingElement>) => {
    const id = slugify(getNodeText(children));
    return (
      <h1
        id={id}
        {...props}
        className="mt-8 mb-4 text-3xl sm:text-4xl font-bold tracking-tight text-slate-900 border-b border-slate-200/60 pb-3"
        style={{ fontFamily: "var(--font-bricolage), sans-serif" }}
      >
        {children}
      </h1>
    );
  },
  h2: ({ children, ...props }: React.HTMLAttributes<HTMLHeadingElement>) => {
    const id = slugify(getNodeText(children));
    return (
      <h2
        id={id}
        {...props}
        className="mt-10 mb-4 text-xl sm:text-2xl font-bold tracking-tight text-slate-900 scroll-mt-20 border-b border-slate-100 pb-2"
        style={{ fontFamily: "var(--font-bricolage), sans-serif" }}
      >
        {children}
      </h2>
    );
  },
  h3: ({ children, ...props }: React.HTMLAttributes<HTMLHeadingElement>) => {
    const id = slugify(getNodeText(children));
    return (
      <h3
        id={id}
        {...props}
        className="mt-7 mb-2.5 text-base sm:text-lg font-semibold tracking-tight text-slate-900 scroll-mt-20"
        style={{ fontFamily: "var(--font-bricolage), sans-serif" }}
      >
        {children}
      </h3>
    );
  },
  p: ({ children, ...props }: React.HTMLAttributes<HTMLParagraphElement>) => (
    <p {...props} className="my-4 text-sm sm:text-[15px] leading-relaxed text-slate-700">
      {children}
    </p>
  ),
  ul: ({ children, ...props }: React.HTMLAttributes<HTMLUListElement>) => (
    <ul {...props} className="my-4 ml-6 list-disc space-y-1.5 text-sm sm:text-[15px] text-slate-700">
      {children}
    </ul>
  ),
  ol: ({ children, ...props }: React.HTMLAttributes<HTMLOListElement>) => (
    <ol {...props} className="my-4 ml-6 list-decimal space-y-1.5 text-sm sm:text-[15px] text-slate-700">
      {children}
    </ol>
  ),
  li: ({ children, ...props }: React.HTMLAttributes<HTMLLIElement>) => (
    <li {...props} className="leading-relaxed">
      {children}
    </li>
  ),
  blockquote: ({ children, ...props }: React.HTMLAttributes<HTMLQuoteElement>) => (
    <blockquote
      {...props}
      className="my-6 border-l-3 border-[#7B62FC] bg-slate-50/80 py-2.5 px-4 italic text-slate-700 rounded-r-lg text-sm"
    >
      {children}
    </blockquote>
  ),
  hr: (props: React.HTMLAttributes<HTMLHRElement>) => (
    <hr {...props} className="my-8 border-slate-200/80" />
  ),
  pre: (props: React.HTMLAttributes<HTMLPreElement>) => {
    // Intercept code fences with language-mermaid to render live Mermaid SVG
    const childrenArray = React.Children.toArray(props.children);
    if (childrenArray.length === 1 && React.isValidElement(childrenArray[0])) {
      const child = childrenArray[0] as React.ReactElement<{ className?: string; children?: React.ReactNode }>;
      const className = child.props.className || "";
      if (className.includes("language-mermaid")) {
        const chartText = getNodeText(child.props.children);
        return <Mermaid chart={chartText} />;
      }
    }
    return <PreBlock {...props} />;
  },
  code: InlineCode,
  table: ({ children, ...props }: React.TableHTMLAttributes<HTMLTableElement>) => (
    <div className="my-8 overflow-hidden rounded-xl border border-slate-200/90 bg-white shadow-sm ring-1 ring-slate-900/5">
      <div className="overflow-x-auto">
        <table {...props} className="w-full min-w-[700px] border-collapse text-left text-xs sm:text-sm">
          {children}
        </table>
      </div>
    </div>
  ),
  thead: ({ children, ...props }: React.HTMLAttributes<HTMLTableSectionElement>) => (
    <thead {...props} className="border-b border-slate-200/90 bg-slate-50/90">
      {children}
    </thead>
  ),
  tbody: ({ children, ...props }: React.HTMLAttributes<HTMLTableSectionElement>) => (
    <tbody {...props} className="divide-y divide-slate-100 bg-white">
      {children}
    </tbody>
  ),
  tr: ({ children, ...props }: React.HTMLAttributes<HTMLTableRowElement>) => (
    <tr
      {...props}
      className="border-b border-slate-100 last:border-b-0 transition-colors hover:bg-slate-50/70 group"
    >
      {children}
    </tr>
  ),
  th: ({ children, className, ...props }: React.ThHTMLAttributes<HTMLTableCellElement>) => (
    <th
      {...props}
      className={`px-4 py-3 text-[11px] font-mono font-semibold uppercase tracking-wider text-slate-600 border-b border-slate-200/90 bg-slate-50/90 whitespace-nowrap first:pl-5 last:pr-5 last:min-w-[280px] ${className || ""}`}
    >
      {children}
    </th>
  ),
  td: ({ children, className, ...props }: React.TdHTMLAttributes<HTMLTableCellElement>) => {
    const isDash = children === "—" || children === "-";
    return (
      <td
        {...props}
        className={`align-top px-4 py-3.5 text-xs sm:text-[13px] leading-relaxed ${
          isDash ? "text-slate-400 font-sans" : "text-slate-600"
        } first:pl-5 first:font-mono first:font-semibold first:text-slate-900 first:whitespace-nowrap first:[&_code]:bg-transparent first:[&_code]:border-0 first:[&_code]:p-0 first:[&_code]:text-slate-900 last:pr-5 last:min-w-[280px] last:text-slate-600 last:font-sans [&_code]:font-mono [&_code]:text-[11.5px] [&_code]:bg-slate-100 [&_code]:text-slate-800 [&_code]:border [&_code]:border-slate-200 [&_code]:px-1.5 [&_code]:py-0.5 [&_code]:rounded ${className || ""}`}
      >
        {children}
      </td>
    );
  },
  a: ({ href, children, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a
      href={href}
      {...props}
      className="font-medium text-[#7B62FC] underline decoration-[#7B62FC]/30 underline-offset-3 transition-colors hover:decoration-[#7B62FC]"
      target={href?.startsWith("http") ? "_blank" : undefined}
      rel={href?.startsWith("http") ? "noopener noreferrer" : undefined}
    >
      {children}
    </a>
  ),
};
