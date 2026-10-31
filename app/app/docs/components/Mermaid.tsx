"use client";

import React, { useEffect, useId, useState } from "react";

interface MermaidProps {
  chart: string;
}

export function Mermaid({ chart }: MermaidProps) {
  const [svg, setSvg] = useState<string>("");
  const [error, setError] = useState<string>("");
  const rawId = useId();
  // Strip colons and invalid characters from React useId for valid DOM ID
  const id = `mermaid_${rawId.replace(/[^a-zA-Z0-9_-]/g, "")}`;

  useEffect(() => {
    let isMounted = true;

    async function renderChart() {
      try {
        const mermaid = (await import("mermaid")).default;
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: "loose",
          theme: "base",
          themeVariables: {
            primaryColor: "#f5f3ff",
            primaryTextColor: "#0f172a",
            primaryBorderColor: "#7B62FC",
            lineColor: "#7B62FC",
            secondaryColor: "#ffffff",
            tertiaryColor: "#faf5ff",
            edgeLabelBackground: "transparent",
            fontFamily: "var(--font-dm-sans), system-ui, -apple-system, sans-serif",
            fontSize: "13px",
            nodeBorder: "1.5px",
          },
        });

        // Clean chart string
        const cleanChart = chart.trim();
        const { svg: renderedSvg } = await mermaid.render(id, cleanChart);

        if (isMounted) {
          setSvg(renderedSvg);
          setError("");
        }
      } catch (err: unknown) {
        if (isMounted) {
          console.error("Mermaid diagram rendering error:", err);
          setError(err instanceof Error ? err.message : "Failed to render Mermaid diagram");
        }
      }
    }

    renderChart();

    return () => {
      isMounted = false;
    };
  }, [chart, id]);

  if (error) {
    return (
      <div className="my-6 rounded-xl border border-amber-300 bg-amber-50/50 p-4 text-xs font-mono text-amber-800">
        <div className="font-semibold mb-1">Diagram rendering error:</div>
        <pre className="overflow-x-auto">{chart}</pre>
      </div>
    );
  }

  if (!svg) {
    return (
      <div className="my-6 flex items-center justify-center rounded-xl border border-slate-200/80 bg-slate-50/50 p-8 text-xs text-slate-400 font-mono">
        Loading architecture diagram...
      </div>
    );
  }

  return (
    <div className="my-6 flex justify-center overflow-x-auto rounded-xl border border-slate-200/80 bg-white p-6 shadow-xs transition-all hover:border-[#7B62FC]/30">
      <div
        className="w-full flex justify-center [&>svg]:max-w-full [&>svg]:h-auto [&_.edgeLabel]:bg-transparent [&_.edgeLabel_rect]:fill-transparent [&_.edgeLabel_span]:bg-transparent"
        dangerouslySetInnerHTML={{ __html: svg }}
      />
    </div>
  );
}

